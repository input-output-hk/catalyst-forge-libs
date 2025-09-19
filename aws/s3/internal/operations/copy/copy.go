// Package copy handles S3 object copy and move operations.
// This includes server-side copying between buckets and multipart copy
// operations for large objects.
//
// Copy operations are optimized to use S3's server-side copy when possible
// to minimize data transfer and improve performance.
package copy

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awstypes "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/s3api"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Copier handles copy operations with automatic multipart support
type Copier struct {
	s3Client s3api.S3API
}

// NewCopier creates a new copy operation handler
func NewCopier(s3Client s3api.S3API) *Copier {
	return &Copier{
		s3Client: s3Client,
	}
}

// Copy performs a copy operation, automatically choosing between simple and multipart copy
func (c *Copier) Copy(
	ctx context.Context,
	srcBucket, srcKey, dstBucket, dstKey string,
	config *s3types.CopyOptionConfig,
) error {
	// First get the source object metadata to determine size
	srcMetadata, err := c.getObjectMetadata(ctx, srcBucket, srcKey)
	if err != nil {
		return errors.NewError("copy", err).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("failed to get source object metadata")
	}

	// AWS S3 simple copy has a 5GB limit
	const maxSimpleCopySize = 5 * 1024 * 1024 * 1024 // 5GB
	const minMultipartCopySize = 100 * 1024 * 1024   // 100MB - reasonable threshold

	objectSize := aws.ToInt64(srcMetadata.ContentLength)

	// Use multipart copy for:
	// 1. Objects > 5GB (AWS limit for simple copy)
	// 2. Objects > 100MB (performance optimization)
	if objectSize > maxSimpleCopySize || objectSize > minMultipartCopySize {
		return c.multipartCopy(ctx, srcBucket, srcKey, dstBucket, dstKey, objectSize, config)
	}

	// Use simple copy for smaller objects
	return c.simpleCopy(ctx, srcBucket, srcKey, dstBucket, dstKey, config)
}

// simpleCopy performs a simple copy operation using CopyObject
func (c *Copier) simpleCopy(
	ctx context.Context,
	srcBucket, srcKey, dstBucket, dstKey string,
	config *s3types.CopyOptionConfig,
) error {
	copySource := srcBucket + "/" + srcKey

	input := &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(copySource),
	}

	// Apply copy options if provided
	c.applyCopyOptions(input, config)

	_, err := c.s3Client.CopyObject(ctx, input)
	if err != nil {
		return errors.NewError("simpleCopy", err).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage("failed to copy from " + copySource)
	}

	return nil
}

// applyCopyOptions applies configuration options to the copy input
func (c *Copier) applyCopyOptions(input *s3.CopyObjectInput, config *s3types.CopyOptionConfig) {
	if config == nil {
		return
	}

	if config.Metadata != nil {
		input.Metadata = config.Metadata
		if !config.ReplaceMetadata {
			input.MetadataDirective = awstypes.MetadataDirectiveCopy
		} else {
			input.MetadataDirective = awstypes.MetadataDirectiveReplace
		}
	}

	if config.StorageClass != "" {
		input.StorageClass = awstypes.StorageClass(config.StorageClass)
	}

	c.applySSEOptionsToCopy(input, config.SSE)

	// Set ACL - default to private for security
	acl := s3types.ACLPrivate
	if config.ACL != "" {
		acl = config.ACL
	}
	input.ACL = awstypes.ObjectCannedACL(acl)
}

// applySSEOptionsToCopy applies server-side encryption options to CopyObjectInput
func (c *Copier) applySSEOptionsToCopy(input *s3.CopyObjectInput, sse *s3types.SSEConfig) {
	if sse == nil {
		return
	}

	switch sse.Type {
	case s3types.SSES3:
		input.ServerSideEncryption = awstypes.ServerSideEncryptionAes256
	case s3types.SSEKMS:
		input.ServerSideEncryption = awstypes.ServerSideEncryptionAwsKms
		if sse.KMSKeyID != "" {
			input.SSEKMSKeyId = aws.String(sse.KMSKeyID)
		}
	default: // SSEC (customer-provided encryption)
		if sse.CustomerKey != "" {
			input.ServerSideEncryption = awstypes.ServerSideEncryptionAes256
			input.SSECustomerAlgorithm = aws.String(string(sse.Type))
			input.SSECustomerKey = aws.String(sse.CustomerKey)
			input.SSECustomerKeyMD5 = aws.String(sse.CustomerKeyMD5)
		}
	}
}

// applySSEOptionsToMultipart applies server-side encryption options to CreateMultipartUploadInput
func (c *Copier) applySSEOptionsToMultipart(input *s3.CreateMultipartUploadInput, sse *s3types.SSEConfig) {
	if sse == nil {
		return
	}

	switch sse.Type {
	case s3types.SSES3:
		input.ServerSideEncryption = awstypes.ServerSideEncryptionAes256
	case s3types.SSEKMS:
		input.ServerSideEncryption = awstypes.ServerSideEncryptionAwsKms
		if sse.KMSKeyID != "" {
			input.SSEKMSKeyId = aws.String(sse.KMSKeyID)
		}
	default: // SSEC (customer-provided encryption)
		if sse.CustomerKey != "" {
			input.ServerSideEncryption = awstypes.ServerSideEncryptionAes256
			input.SSECustomerAlgorithm = aws.String(string(sse.Type))
			input.SSECustomerKey = aws.String(sse.CustomerKey)
			input.SSECustomerKeyMD5 = aws.String(sse.CustomerKeyMD5)
		}
	}
}

// multipartCopy performs a multipart copy operation for large objects
func (c *Copier) multipartCopy(
	ctx context.Context,
	srcBucket, srcKey, dstBucket, dstKey string,
	objectSize int64,
	config *s3types.CopyOptionConfig,
) error {
	// Determine part size (same logic as multipart upload)
	partSize := c.getPartSize(config)
	numParts := c.calculateParts(objectSize, partSize)

	// Create multipart upload
	uploadID, err := c.createMultipartUpload(ctx, dstBucket, dstKey, config)
	if err != nil {
		return err
	}

	// Copy parts concurrently
	parts, err := c.copyParts(
		ctx,
		srcBucket,
		srcKey,
		dstBucket,
		dstKey,
		uploadID,
		objectSize,
		partSize,
		numParts,
		config,
	)
	if err != nil {
		// Clean up on failure
		c.abortMultipartUpload(ctx, dstBucket, dstKey, uploadID)
		return err
	}

	// Complete multipart upload
	return c.completeMultipartUpload(ctx, dstBucket, dstKey, uploadID, parts)
}

// getPartSize returns the configured part size or default
func (c *Copier) getPartSize(config *s3types.CopyOptionConfig) int64 {
	// CopyOptionConfig doesn't have PartSize, so use default
	return 8 * 1024 * 1024 // 8MB default (same as upload default)
}

// calculateParts calculates the number of parts needed
func (c *Copier) calculateParts(size, partSize int64) int {
	if size == 0 {
		return 1
	}
	parts := int((size + partSize - 1) / partSize) // Ceiling division
	return parts
}

// getObjectMetadata retrieves metadata for an object
func (c *Copier) getObjectMetadata(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := c.s3Client.HeadObject(ctx, input)
	if err != nil {
		return nil, errors.NewError("getObjectMetadata", err).WithBucket(bucket).WithKey(key)
	}
	return result, nil
}

// createMultipartUpload creates a new multipart upload for copy destination
func (c *Copier) createMultipartUpload(
	ctx context.Context,
	bucket, key string,
	config *s3types.CopyOptionConfig,
) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	// Apply copy options
	c.applyMultipartUploadOptions(input, config)

	output, err := c.s3Client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", errors.NewError("createMultipartUpload", err).WithBucket(bucket).WithKey(key)
	}

	return aws.ToString(output.UploadId), nil
}

// applyMultipartUploadOptions applies options for multipart upload creation
func (c *Copier) applyMultipartUploadOptions(input *s3.CreateMultipartUploadInput, config *s3types.CopyOptionConfig) {
	if config == nil {
		return
	}

	if config.StorageClass != "" {
		input.StorageClass = awstypes.StorageClass(config.StorageClass)
	}

	c.applySSEOptionsToMultipart(input, config.SSE)

	// Set ACL - default to private for security
	acl := s3types.ACLPrivate
	if config.ACL != "" {
		acl = config.ACL
	}
	input.ACL = awstypes.ObjectCannedACL(acl)

	if config.Metadata != nil {
		input.Metadata = config.Metadata
	}
}

// copyParts copies all parts concurrently
func (c *Copier) copyParts(
	ctx context.Context,
	srcBucket, srcKey, dstBucket, dstKey, uploadID string,
	objectSize, partSize int64,
	numParts int,
	config *s3types.CopyOptionConfig,
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
	concurrency := c.getConcurrency(config)

	// Use semaphore to limit concurrent operations
	sem := make(chan struct{}, concurrency)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numParts; i++ {
		wg.Add(1)
		go func(partNum int) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Copy part
			etag, partSizeActual, err := c.copyPart(
				ctx,
				srcBucket,
				srcKey,
				dstBucket,
				dstKey,
				uploadID,
				objectSize,
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

	return parts, nil
}

// getConcurrency returns the configured concurrency level or default
func (c *Copier) getConcurrency(config *s3types.CopyOptionConfig) int {
	// CopyOptionConfig doesn't have Concurrency, so use default
	return 5 // Default concurrency
}

// copyPart copies a single part from source to destination
func (c *Copier) copyPart(
	ctx context.Context,
	srcBucket, srcKey, dstBucket, dstKey, uploadID string,
	objectSize, partSize int64,
	partNumber int32,
	config *s3types.CopyOptionConfig,
) (string, int64, error) {
	// Calculate byte range for this part
	offset := int64(partNumber-1) * partSize
	size := partSize

	// Adjust size for last part
	if offset+size > objectSize {
		size = objectSize - offset
	}

	// Create copy source with byte range
	copySource := fmt.Sprintf("%s/%s", srcBucket, srcKey)
	copySourceRange := fmt.Sprintf("bytes=%d-%d", offset, offset+size-1)

	input := &s3.UploadPartCopyInput{
		Bucket:          aws.String(dstBucket),
		Key:             aws.String(dstKey),
		CopySource:      aws.String(copySource),
		CopySourceRange: aws.String(copySourceRange),
		UploadId:        aws.String(uploadID),
		PartNumber:      aws.Int32(partNumber),
	}

	// Set SSE for copy part if customer-provided encryption
	if config != nil && config.SSE != nil && config.SSE.CustomerKey != "" {
		input.SSECustomerAlgorithm = aws.String(string(config.SSE.Type))
		input.SSECustomerKey = aws.String(config.SSE.CustomerKey)
		input.SSECustomerKeyMD5 = aws.String(config.SSE.CustomerKeyMD5)
	}

	output, err := c.s3Client.UploadPartCopy(ctx, input)
	if err != nil {
		return "", 0, errors.NewError("copyPart", err).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage(fmt.Sprintf("failed to copy part %d", partNumber))
	}

	return aws.ToString(output.CopyPartResult.ETag), size, nil
}

// completeMultipartUpload completes the multipart copy
func (c *Copier) completeMultipartUpload(
	ctx context.Context,
	bucket, key, uploadID string,
	parts []awstypes.CompletedPart,
) error {
	input := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &awstypes.CompletedMultipartUpload{
			Parts: parts,
		},
	}

	_, err := c.s3Client.CompleteMultipartUpload(ctx, input)
	if err != nil {
		// Clean up on failure
		c.abortMultipartUpload(ctx, bucket, key, uploadID)
		return errors.NewError("completeMultipartUpload", err).WithBucket(bucket).WithKey(key)
	}

	return nil
}

// abortMultipartUpload cleans up a failed multipart copy
func (c *Copier) abortMultipartUpload(ctx context.Context, bucket, key, uploadID string) {
	input := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	}
	// Ignore errors during cleanup
	_, _ = c.s3Client.AbortMultipartUpload(ctx, input)
}
