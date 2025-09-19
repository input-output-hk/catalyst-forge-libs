// Package multipart handles multipart upload operations with concurrent part uploads
// and automatic error recovery.
package multipart

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awstypes "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/s3api"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Uploader handles multipart upload operations
type Uploader struct {
	s3Client  s3api.S3API
	totalSize int64 // Track total uploaded size
}

// NewUploader creates a new multipart uploader
func NewUploader(s3Client s3api.S3API) *Uploader {
	return &Uploader{
		s3Client: s3Client,
	}
}

// Upload performs a multipart upload of data from an io.Reader
func (u *Uploader) Upload(
	ctx context.Context,
	bucket, key string,
	reader io.Reader,
	size int64,
	config *s3types.UploadConfig,
	startTime time.Time,
) (*s3types.UploadResult, error) {
	return u.UploadWithClientConcurrency(ctx, bucket, key, reader, size, config, startTime, 0)
}

// UploadWithClientConcurrency performs a multipart upload with explicit client concurrency
func (u *Uploader) UploadWithClientConcurrency(
	ctx context.Context,
	bucket, key string,
	reader io.Reader,
	size int64,
	config *s3types.UploadConfig,
	startTime time.Time,
	clientConcurrency int,
) (*s3types.UploadResult, error) {
	// Determine part size
	partSize := u.getPartSize(config.PartSize)

	// Calculate number of parts needed
	numParts := u.calculateParts(size, partSize)

	// Create multipart upload
	uploadID, err := u.createMultipartUpload(ctx, bucket, key, config)
	if err != nil {
		return nil, err
	}

	// Upload parts concurrently
	parts, err := u.uploadParts(
		ctx,
		bucket,
		key,
		reader,
		size,
		uploadID,
		partSize,
		numParts,
		config,
		clientConcurrency,
	)
	if err != nil {
		// Clean up on failure
		u.abortMultipartUpload(ctx, bucket, key, uploadID)
		return nil, err
	}

	// Complete multipart upload
	return u.completeMultipartUpload(ctx, bucket, key, uploadID, parts, startTime)
}

// getPartSize returns the configured part size or default
func (u *Uploader) getPartSize(configuredSize int64) int64 {
	if configuredSize > 0 {
		return configuredSize
	}
	return 5 * 1024 * 1024 // 5MB default
}

// calculateParts calculates the number of parts needed for the given size and part size
func (u *Uploader) calculateParts(size, partSize int64) int {
	if size == 0 {
		return 1
	}
	parts := int((size + partSize - 1) / partSize) // Ceiling division
	return parts
}

// createMultipartUpload creates a new multipart upload
func (u *Uploader) createMultipartUpload(
	ctx context.Context,
	bucket, key string,
	config *s3types.UploadConfig,
) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(config.ContentType),
	}

	// Set storage class if specified
	if config.StorageClass != "" {
		input.StorageClass = awstypes.StorageClass(config.StorageClass)
	}

	// Set metadata if provided
	if len(config.Metadata) > 0 {
		input.Metadata = config.Metadata
	}

	// Set SSE if configured
	if config.SSE != nil {
		switch config.SSE.Type {
		case s3types.SSES3:
			input.ServerSideEncryption = awstypes.ServerSideEncryptionAes256
		case s3types.SSEKMS:
			input.ServerSideEncryption = awstypes.ServerSideEncryptionAwsKms
			if config.SSE.KMSKeyID != "" {
				input.SSEKMSKeyId = aws.String(config.SSE.KMSKeyID)
			}
		default: // SSEC (customer-provided encryption)
			if config.SSE.CustomerKey != "" {
				input.ServerSideEncryption = awstypes.ServerSideEncryptionAes256
				input.SSECustomerAlgorithm = aws.String(string(config.SSE.Type))
				input.SSECustomerKey = aws.String(config.SSE.CustomerKey)
				input.SSECustomerKeyMD5 = aws.String(config.SSE.CustomerKeyMD5)
			}
		}
	}

	output, err := u.s3Client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", errors.NewError("createMultipartUpload", err).WithBucket(bucket).WithKey(key)
	}

	return aws.ToString(output.UploadId), nil
}

// uploadParts uploads all parts concurrently
func (u *Uploader) uploadParts(
	ctx context.Context,
	bucket, key string,
	reader io.Reader,
	size int64,
	uploadID string,
	partSize int64,
	numParts int,
	config *s3types.UploadConfig,
	clientConcurrency int,
) ([]awstypes.CompletedPart, error) {
	// Create channels for coordination
	type partResult struct {
		partNumber int32
		etag       string
		size       int64
		err        error
	}

	results := make(chan partResult, numParts)
	parts := make([]awstypes.CompletedPart, numParts)

	// Determine concurrency level
	concurrency := u.getConcurrency(config.Concurrency, clientConcurrency)

	// Use semaphore to limit concurrent uploads
	sem := make(chan struct{}, concurrency)

	// Read all data first (simplified approach for this implementation)
	// In production, you'd want to read parts on-demand or use a more sophisticated approach
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numParts; i++ {
		wg.Add(1)
		go func(partNum int) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Upload part
			etag, partSizeActual, err := u.uploadPart(
				ctx,
				bucket,
				key,
				data,
				size,
				uploadID,
				partSize,
				int32(partNum+1),
				config,
			)

			// Send result
			results <- partResult{
				partNumber: int32(partNum + 1),
				etag:       etag,
				size:       partSizeActual,
				err:        err,
			}
		}(i)
	}

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	totalSize := int64(0)
	for result := range results {
		if result.err != nil {
			return nil, result.err
		}
		parts[result.partNumber-1] = awstypes.CompletedPart{
			ETag:       aws.String(result.etag),
			PartNumber: aws.Int32(result.partNumber),
		}
		totalSize += result.size
	}

	// Store total size for result
	u.totalSize = totalSize
	return parts, nil
}

// getConcurrency returns the configured concurrency level or default
func (u *Uploader) getConcurrency(configuredConcurrency, clientConcurrency int) int {
	if configuredConcurrency > 0 {
		return configuredConcurrency
	}
	if clientConcurrency > 0 {
		return clientConcurrency
	}
	return 5 // Default concurrency
}

// uploadPart uploads a single part
func (u *Uploader) uploadPart(
	ctx context.Context,
	bucket, key string,
	data []byte,
	totalSize int64,
	uploadID string,
	partSize int64,
	partNumber int32,
	config *s3types.UploadConfig,
) (string, int64, error) {
	// Calculate offset and size for this part
	offset := int64(partNumber-1) * partSize
	size := partSize

	// Adjust size for last part
	if offset+size > totalSize {
		size = totalSize - offset
	}

	// Extract part data
	partData := data[offset : offset+size]

	// Prepare upload part input
	input := &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
		Body:       bytes.NewReader(partData),
	}

	// Set SSE for part upload if customer-provided encryption
	if config.SSE != nil && config.SSE.CustomerKey != "" {
		input.SSECustomerAlgorithm = aws.String(string(config.SSE.Type))
		input.SSECustomerKey = aws.String(config.SSE.CustomerKey)
		input.SSECustomerKeyMD5 = aws.String(config.SSE.CustomerKeyMD5)
	}

	output, err := u.s3Client.UploadPart(ctx, input)
	if err != nil {
		return "", 0, errors.NewError("uploadPart", err).WithBucket(bucket).WithKey(key)
	}

	return aws.ToString(output.ETag), size, nil
}

// completeMultipartUpload completes the multipart upload
func (u *Uploader) completeMultipartUpload(
	ctx context.Context,
	bucket, key, uploadID string,
	parts []awstypes.CompletedPart,
	startTime time.Time,
) (*s3types.UploadResult, error) {
	input := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &awstypes.CompletedMultipartUpload{
			Parts: parts,
		},
	}

	output, err := u.s3Client.CompleteMultipartUpload(ctx, input)
	if err != nil {
		// Clean up on failure
		u.abortMultipartUpload(ctx, bucket, key, uploadID)
		return nil, errors.NewError("completeMultipartUpload", err).WithBucket(bucket).WithKey(key)
	}

	// Total size is already tracked in u.totalSize during uploadParts

	result := &s3types.UploadResult{
		Key:       key,
		Size:      u.totalSize,
		ETag:      aws.ToString(output.ETag),
		VersionID: aws.ToString(output.VersionId),
		Duration:  time.Since(startTime),
	}

	return result, nil
}

// abortMultipartUpload cleans up a failed multipart upload
func (u *Uploader) abortMultipartUpload(ctx context.Context, bucket, key, uploadID string) {
	input := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	}
	// Ignore errors during cleanup
	_, _ = u.s3Client.AbortMultipartUpload(ctx, input)
}
