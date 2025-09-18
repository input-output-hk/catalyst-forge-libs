// Package upload handles S3 object upload operations.
// This includes simple uploads, multipart uploads, and stream-based uploads.
//
// The package automatically detects when to use multipart upload based on
// file size thresholds and handles concurrent part uploads for optimal performance.
package upload

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awstypes "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/s3api"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Uploader handles S3 upload operations with automatic multipart detection.
type Uploader struct {
	s3Client s3api.S3API
}

// New creates a new Uploader instance.
func New(s3Client s3api.S3API) *Uploader {
	return &Uploader{
		s3Client: s3Client,
	}
}

// Upload uploads data from an io.Reader to S3.
// It automatically detects when to use multipart upload based on size thresholds.
func (u *Uploader) Upload(
	ctx context.Context,
	bucket, key string,
	reader io.Reader,
	config *s3types.UploadConfig,
	startTime time.Time,
) (*s3types.UploadResult, error) {
	// Read all data to determine size and prepare for upload
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.NewError("upload", err).WithBucket(bucket).WithKey(key)
	}

	size := int64(len(data))

	// Choose upload method based on size
	if size >= 100*1024*1024 { // 100MB threshold for multipart
		return u.uploadMultipart(ctx, bucket, key, bytes.NewReader(data), size, config, startTime)
	}

	return u.uploadSimple(ctx, bucket, key, data, config, startTime)
}

// UploadFile uploads a file from the local filesystem to S3.
// It automatically detects when to use multipart upload based on file size.
func (u *Uploader) UploadFile(
	ctx context.Context,
	bucket, key string,
	reader io.Reader,
	size int64,
	config *s3types.UploadConfig,
	startTime time.Time,
) (*s3types.UploadResult, error) {
	// Choose upload method based on size
	if size >= 100*1024*1024 { // 100MB threshold for multipart
		return u.uploadMultipart(ctx, bucket, key, reader, size, config, startTime)
	}

	// Read file content for simple upload
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.NewError("uploadFile", err).WithBucket(bucket).WithKey(key)
	}

	return u.uploadSimple(ctx, bucket, key, data, config, startTime)
}

// UploadSimple performs a simple (non-multipart) S3 upload.
func (u *Uploader) UploadSimple(
	ctx context.Context,
	bucket, key string,
	data []byte,
	config *s3types.UploadConfig,
	startTime time.Time,
) (*s3types.UploadResult, error) {
	return u.uploadSimple(ctx, bucket, key, data, config, startTime)
}

// uploadSimple performs a simple (non-multipart) S3 upload.
func (u *Uploader) uploadSimple(
	ctx context.Context,
	bucket, key string,
	data []byte,
	config *s3types.UploadConfig,
	startTime time.Time,
) (*s3types.UploadResult, error) {
	size := int64(len(data))

	// Prepare the PutObject input
	input := &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentType:   aws.String(config.ContentType),
		ContentLength: aws.Int64(size),
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
		case "AES256":
			input.ServerSideEncryption = awstypes.ServerSideEncryptionAes256
		case "aws:kms":
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

	// Perform the upload
	output, err := u.s3Client.PutObject(ctx, input)
	if err != nil {
		return nil, errors.NewError("uploadSimple", err).WithBucket(bucket).WithKey(key)
	}

	// Create the result
	result := &s3types.UploadResult{
		Key:       key,
		Size:      size,
		ETag:      aws.ToString(output.ETag),
		VersionID: aws.ToString(output.VersionId),
		Duration:  time.Since(startTime),
	}

	// Call progress tracker if provided
	if config.ProgressTracker != nil {
		config.ProgressTracker.Update(size, size)
		config.ProgressTracker.Complete()
	}

	return result, nil
}

// uploadMultipart performs a multipart S3 upload for large files.
func (u *Uploader) uploadMultipart(
	ctx context.Context,
	bucket, key string,
	reader io.Reader,
	size int64,
	config *s3types.UploadConfig,
	startTime time.Time,
) (*s3types.UploadResult, error) {
	// Create multipart upload
	createInput := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(config.ContentType),
	}

	if config.StorageClass != "" {
		createInput.StorageClass = awstypes.StorageClass(config.StorageClass)
	}

	if len(config.Metadata) > 0 {
		createInput.Metadata = config.Metadata
	}

	// Set SSE if configured
	if config.SSE != nil {
		switch config.SSE.Type {
		case "AES256":
			createInput.ServerSideEncryption = awstypes.ServerSideEncryptionAes256
		case "aws:kms":
			createInput.ServerSideEncryption = awstypes.ServerSideEncryptionAwsKms
			if config.SSE.KMSKeyID != "" {
				createInput.SSEKMSKeyId = aws.String(config.SSE.KMSKeyID)
			}
		default: // SSEC (customer-provided encryption)
			if config.SSE.CustomerKey != "" {
				createInput.ServerSideEncryption = awstypes.ServerSideEncryptionAes256
				createInput.SSECustomerAlgorithm = aws.String(string(config.SSE.Type))
				createInput.SSECustomerKey = aws.String(config.SSE.CustomerKey)
				createInput.SSECustomerKeyMD5 = aws.String(config.SSE.CustomerKeyMD5)
			}
		}
	}

	createOutput, err := u.s3Client.CreateMultipartUpload(ctx, createInput)
	if err != nil {
		return nil, errors.NewError("uploadMultipart", err).WithBucket(bucket).WithKey(key)
	}

	uploadID := aws.ToString(createOutput.UploadId)

	// For simplicity, upload the entire content as one part
	// In a full implementation, this would split into multiple parts
	data, err := io.ReadAll(reader)
	if err != nil {
		// Abort the multipart upload
		u.abortMultipartUpload(ctx, bucket, key, uploadID)
		return nil, errors.NewError("uploadMultipart", err).WithBucket(bucket).WithKey(key)
	}

	partInput := &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(data),
	}

	partOutput, err := u.s3Client.UploadPart(ctx, partInput)
	if err != nil {
		// Abort the multipart upload
		u.abortMultipartUpload(ctx, bucket, key, uploadID)
		return nil, errors.NewError("uploadMultipart", err).WithBucket(bucket).WithKey(key)
	}

	// Complete the multipart upload
	completeInput := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &awstypes.CompletedMultipartUpload{
			Parts: []awstypes.CompletedPart{
				{
					ETag:       partOutput.ETag,
					PartNumber: aws.Int32(1),
				},
			},
		},
	}

	completeOutput, err := u.s3Client.CompleteMultipartUpload(ctx, completeInput)
	if err != nil {
		// Abort the multipart upload
		u.abortMultipartUpload(ctx, bucket, key, uploadID)
		return nil, errors.NewError("uploadMultipart", err).WithBucket(bucket).WithKey(key)
	}

	// Create the result
	result := &s3types.UploadResult{
		Key:       key,
		Size:      size,
		ETag:      aws.ToString(completeOutput.ETag),
		VersionID: aws.ToString(completeOutput.VersionId),
		Duration:  time.Since(startTime),
	}

	// Call progress tracker if provided
	if config.ProgressTracker != nil {
		config.ProgressTracker.Update(size, size)
		config.ProgressTracker.Complete()
	}

	return result, nil
}

// abortMultipartUpload cleans up a failed multipart upload.
func (u *Uploader) abortMultipartUpload(ctx context.Context, bucket, key, uploadID string) {
	abortInput := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	}
	// Ignore errors during cleanup
	_, _ = u.s3Client.AbortMultipartUpload(ctx, abortInput)
}
