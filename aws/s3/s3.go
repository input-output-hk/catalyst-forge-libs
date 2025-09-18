// Package s3 provides the main S3 client and core operations.
package s3

import (
	"context"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gabriel-vasile/mimetype"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/operations/copy"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/operations/download"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/operations/upload"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

const (
	// DefaultContentType is the default content type used when content type detection fails
	DefaultContentType = "application/octet-stream"
)

// Upload uploads data from an io.Reader to S3.
// It automatically detects when to use multipart upload based on size thresholds.
// Progress tracking and other options can be configured via UploadOption parameters.
func (c *Client) Upload(
	ctx context.Context,
	bucket, key string,
	reader io.Reader,
	opts ...s3types.UploadOption,
) (*s3types.UploadResult, error) {
	if bucket == "" {
		return nil, errors.NewError("upload", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if key == "" {
		return nil, errors.NewError("upload", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("object key cannot be empty")
	}
	if reader == nil {
		return nil, errors.NewError("upload", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("reader cannot be nil")
	}

	// Apply upload options
	config := &s3types.UploadOptionConfig{
		ContentType:  DefaultContentType, // Default content type
		StorageClass: s3types.StorageClassStandard,
		Metadata:     make(map[string]string),
		PartSize:     8 * 1024 * 1024, // 8MB default
		Concurrency:  5,
	}
	for _, opt := range opts {
		opt(config)
	}

	// Determine content type if not explicitly set
	if config.ContentType == DefaultContentType {
		// Try to detect content type from key extension
		config.ContentType = c.detectContentType(key)
	}

	startTime := time.Now()

	// Use internal upload package
	uploader := upload.New(c.s3Client)
	var sseConfig *s3types.SSEConfig
	if config.SSE != nil {
		sseConfig = &s3types.SSEConfig{
			Type:           config.SSE.Type,
			KMSKeyID:       config.SSE.KMSKeyID,
			CustomerKey:    config.SSE.CustomerKey,
			CustomerKeyMD5: config.SSE.CustomerKeyMD5,
		}
	}

	internalConfig := &s3types.UploadConfig{
		ContentType:     config.ContentType,
		Metadata:        config.Metadata,
		StorageClass:    config.StorageClass,
		SSE:             sseConfig,
		ACL:             config.ACL,
		ProgressTracker: config.ProgressTracker,
		PartSize:        config.PartSize,
		Concurrency:     config.Concurrency,
	}

	result, err := uploader.Upload(ctx, bucket, key, reader, internalConfig, startTime)
	if err != nil {
		return nil, errors.NewError("upload", err).WithBucket(bucket).WithKey(key)
	}

	return result, nil
}

// UploadFile uploads a file from the local filesystem to S3.
// It automatically detects when to use multipart upload based on file size.
func (c *Client) UploadFile(
	ctx context.Context,
	bucket, key, filepath string,
	opts ...s3types.UploadOption,
) (*s3types.UploadResult, error) {
	if bucket == "" {
		return nil, errors.NewError("uploadFile", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if key == "" {
		return nil, errors.NewError("uploadFile", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("object key cannot be empty")
	}
	if filepath == "" {
		return nil, errors.NewError("uploadFile", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("filepath cannot be empty")
	}

	// Check if file exists and get its info
	info, err := c.fs.Stat(filepath)
	if err != nil {
		return nil, errors.NewError("uploadFile", err).WithBucket(bucket).WithKey(key)
	}
	if info.IsDir() {
		return nil, errors.NewError("uploadFile", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("filepath points to a directory, not a file")
	}

	// Apply upload options
	config := &s3types.UploadOptionConfig{
		ContentType:  DefaultContentType,
		StorageClass: s3types.StorageClassStandard,
		Metadata:     make(map[string]string),
		PartSize:     8 * 1024 * 1024, // 8MB default
		Concurrency:  5,
	}
	for _, opt := range opts {
		opt(config)
	}

	// Determine content type if not explicitly set
	if config.ContentType == DefaultContentType {
		config.ContentType = c.detectContentType(filepath)
	}

	// Open the file
	file, err := c.fs.Open(filepath)
	if err != nil {
		return nil, errors.NewError("uploadFile", err).WithBucket(bucket).WithKey(key)
	}
	defer file.Close()

	size := info.Size()
	startTime := time.Now()

	// Use internal upload package
	uploader := upload.New(c.s3Client)
	var sseConfig *s3types.SSEConfig
	if config.SSE != nil {
		sseConfig = &s3types.SSEConfig{
			Type:           config.SSE.Type,
			KMSKeyID:       config.SSE.KMSKeyID,
			CustomerKey:    config.SSE.CustomerKey,
			CustomerKeyMD5: config.SSE.CustomerKeyMD5,
		}
	}

	internalConfig := &s3types.UploadConfig{
		ContentType:     config.ContentType,
		Metadata:        config.Metadata,
		StorageClass:    config.StorageClass,
		SSE:             sseConfig,
		ACL:             config.ACL,
		ProgressTracker: config.ProgressTracker,
		PartSize:        config.PartSize,
		Concurrency:     config.Concurrency,
	}

	result, err := uploader.UploadFile(ctx, bucket, key, file, size, internalConfig, startTime)
	if err != nil {
		return nil, errors.NewError("uploadFile", err).WithBucket(bucket).WithKey(key)
	}

	return result, nil
}

// Put uploads byte data to S3.
// This is a convenience method for small amounts of data that fit in memory.
func (c *Client) Put(ctx context.Context, bucket, key string, data []byte, opts ...s3types.UploadOption) error {
	if bucket == "" {
		return errors.NewError("put", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if key == "" {
		return errors.NewError("put", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("object key cannot be empty")
	}

	// Apply upload options
	config := &s3types.UploadOptionConfig{
		ContentType:  DefaultContentType,
		StorageClass: s3types.StorageClassStandard,
		Metadata:     make(map[string]string),
		PartSize:     8 * 1024 * 1024, // 8MB default
		Concurrency:  5,
	}
	for _, opt := range opts {
		opt(config)
	}

	// Determine content type if not explicitly set
	if config.ContentType == DefaultContentType {
		config.ContentType = c.detectContentType(key)
	}

	startTime := time.Now()

	// Use internal upload package for Put
	uploader := upload.New(c.s3Client)
	var sseConfig *s3types.SSEConfig
	if config.SSE != nil {
		sseConfig = &s3types.SSEConfig{
			Type:           config.SSE.Type,
			KMSKeyID:       config.SSE.KMSKeyID,
			CustomerKey:    config.SSE.CustomerKey,
			CustomerKeyMD5: config.SSE.CustomerKeyMD5,
		}
	}

	internalConfig := &s3types.UploadConfig{
		ContentType:     config.ContentType,
		Metadata:        config.Metadata,
		StorageClass:    config.StorageClass,
		SSE:             sseConfig,
		ACL:             config.ACL,
		ProgressTracker: config.ProgressTracker,
		PartSize:        config.PartSize,
		Concurrency:     config.Concurrency,
	}

	result, err := uploader.UploadSimple(ctx, bucket, key, data, internalConfig, startTime)
	if err != nil {
		return errors.NewError("put", err).WithBucket(bucket).WithKey(key)
	}

	// Put doesn't return a result, just the error
	_ = result
	return nil
}

// Download downloads an object from S3 and writes it to an io.Writer.
// It provides stream-based downloading with memory-efficient handling of large files.
// Progress tracking and range requests can be configured via DownloadOption parameters.
func (c *Client) Download(
	ctx context.Context,
	bucket, key string,
	writer io.Writer,
	opts ...s3types.DownloadOption,
) (*s3types.DownloadResult, error) {
	if bucket == "" {
		return nil, errors.NewError("download", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if key == "" {
		return nil, errors.NewError("download", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("object key cannot be empty")
	}
	if writer == nil {
		return nil, errors.NewError("download", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("writer cannot be nil")
	}

	// Apply download options
	config := &s3types.DownloadOptionConfig{}
	for _, opt := range opts {
		opt(config)
	}

	startTime := time.Now()

	// Use internal download package
	downloader := download.New(c.s3Client)
	internalConfig := &s3types.DownloadConfig{
		ProgressTracker: config.ProgressTracker,
		RangeSpec:       config.RangeSpec,
	}

	result, err := downloader.Download(ctx, bucket, key, writer, internalConfig, startTime)
	if err != nil {
		return nil, errors.NewError("download", err).WithBucket(bucket).WithKey(key)
	}

	return result, nil
}

// DownloadFile downloads an object from S3 to a local file.
// The file will be created if it doesn't exist, or truncated if it does.
// It provides memory-efficient handling of large files.
func (c *Client) DownloadFile(
	ctx context.Context,
	bucket, key, filepath string,
	opts ...s3types.DownloadOption,
) (*s3types.DownloadResult, error) {
	if bucket == "" {
		return nil, errors.NewError("downloadFile", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if key == "" {
		return nil, errors.NewError("downloadFile", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("object key cannot be empty")
	}
	if filepath == "" {
		return nil, errors.NewError("downloadFile", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("filepath cannot be empty")
	}

	// Apply download options
	config := &s3types.DownloadOptionConfig{}
	for _, opt := range opts {
		opt(config)
	}

	startTime := time.Now()

	// Use internal download package
	downloader := download.New(c.s3Client)
	internalConfig := &s3types.DownloadConfig{
		ProgressTracker: config.ProgressTracker,
		RangeSpec:       config.RangeSpec,
	}

	result, err := downloader.DownloadFile(ctx, bucket, key, filepath, internalConfig, startTime)
	if err != nil {
		return nil, errors.NewError("downloadFile", err).WithBucket(bucket).WithKey(key)
	}

	return result, nil
}

// Get downloads an entire object from S3 and returns it as a byte slice.
// This is a convenience method for small objects that can fit in memory.
// For large objects, use Download or DownloadFile instead.
func (c *Client) Get(ctx context.Context, bucket, key string, opts ...s3types.DownloadOption) ([]byte, error) {
	if bucket == "" {
		return nil, errors.NewError("get", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if key == "" {
		return nil, errors.NewError("get", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("object key cannot be empty")
	}

	// Apply download options
	config := &s3types.DownloadOptionConfig{}
	for _, opt := range opts {
		opt(config)
	}

	startTime := time.Now()

	// Use internal download package
	downloader := download.New(c.s3Client)
	internalConfig := &s3types.DownloadConfig{
		ProgressTracker: config.ProgressTracker,
		RangeSpec:       config.RangeSpec,
	}

	data, err := downloader.Get(ctx, bucket, key, internalConfig, startTime)
	if err != nil {
		return nil, errors.NewError("get", err).WithBucket(bucket).WithKey(key)
	}

	return data, nil
}

// detectContentType determines the content type using mimetype where possible,
// falling back to extension-based lookup when the path is not a local file.
func (c *Client) detectContentType(path string) string {
	// If the path points to an existing local file, prefer sniffing its content.
	info, err := c.fs.Stat(path)
	if err != nil || info.IsDir() {
		// Fall back to extension-based detection
		return c.detectContentTypeFromExtension(path)
	}

	// Try to read first few bytes to detect content type
	file, err := c.fs.Open(path)
	if err != nil {
		// Fall back to extension-based detection
		return c.detectContentTypeFromExtension(path)
	}
	defer file.Close()

	// Read first 512 bytes for content detection
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	if n > 0 {
		if mt := mimetype.Detect(buf[:n]); mt != nil {
			return mt.String()
		}
	}

	// Fall back to extension-based detection
	return c.detectContentTypeFromExtension(path)
}

// Delete deletes a single object from S3.
// Returns an error if the operation fails.
func (c *Client) Delete(ctx context.Context, bucket, key string) error {
	if bucket == "" {
		return errors.NewError("delete", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if key == "" {
		return errors.NewError("delete", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("object key cannot be empty")
	}

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := c.s3Client.DeleteObject(ctx, input)
	if err != nil {
		return errors.NewError("delete", err).WithBucket(bucket).WithKey(key)
	}

	return nil
}

// DeleteMany deletes multiple objects from S3 in a single batch operation.
// This method uses S3's DeleteObjects API which can delete up to 1000 objects at once.
// Returns a DeleteResult containing information about successful and failed deletions.
func (c *Client) DeleteMany(ctx context.Context, bucket string, keys []string) (*s3types.DeleteResult, error) {
	if bucket == "" {
		return nil, errors.NewError("deleteMany", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithMessage("bucket name cannot be empty")
	}
	if len(keys) == 0 {
		return nil, errors.NewError("deleteMany", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithMessage("keys cannot be empty")
	}

	// S3 allows up to 1000 objects per delete request
	const maxKeysPerRequest = 1000
	if len(keys) > maxKeysPerRequest {
		return nil, errors.NewError("deleteMany", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithMessage("too many keys: maximum is 1000 per request")
	}

	startTime := time.Now()

	// Build the delete request
	deleteObjects := make([]types.ObjectIdentifier, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			return nil, errors.NewError("deleteMany", errors.ErrInvalidInput).
				WithBucket(bucket).
				WithMessage("empty key in keys slice")
		}
		deleteObjects = append(deleteObjects, types.ObjectIdentifier{
			Key: aws.String(key),
		})
	}

	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: deleteObjects,
		},
	}

	result, err := c.s3Client.DeleteObjects(ctx, input)
	if err != nil {
		return nil, errors.NewError("deleteMany", err).WithBucket(bucket)
	}

	// Process the result
	deleteResult := &s3types.DeleteResult{
		Duration: time.Since(startTime),
	}

	// Process successfully deleted objects
	if result.Deleted != nil {
		deleteResult.Deleted = make([]s3types.Object, 0, len(result.Deleted))
		for _, deleted := range result.Deleted {
			deleteResult.Deleted = append(deleteResult.Deleted, s3types.Object{
				Key: aws.ToString(deleted.Key),
			})
		}
	}

	// Process errors
	if result.Errors != nil {
		deleteResult.Errors = make([]s3types.DeleteError, 0, len(result.Errors))
		for _, err := range result.Errors {
			deleteResult.Errors = append(deleteResult.Errors, s3types.DeleteError{
				Key:     aws.ToString(err.Key),
				Version: aws.ToString(err.VersionId),
				Code:    aws.ToString(err.Code),
				Message: aws.ToString(err.Message),
			})
		}
	}

	return deleteResult, nil
}

// Exists checks if an object exists in S3 using a HEAD request.
// Returns true if the object exists, false if it doesn't exist.
// Returns an error for other types of failures (network issues, permissions, etc.).
func (c *Client) Exists(ctx context.Context, bucket, key string) (bool, error) {
	if bucket == "" {
		return false, errors.NewError("exists", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if key == "" {
		return false, errors.NewError("exists", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("object key cannot be empty")
	}

	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := c.s3Client.HeadObject(ctx, input)
	if err != nil {
		// Check if it's a "not found" error by examining the error message
		errMsg := err.Error()
		if strings.Contains(errMsg, "NotFound") || strings.Contains(errMsg, "NoSuchKey") {
			return false, nil
		}
		return false, errors.NewError("exists", err).WithBucket(bucket).WithKey(key)
	}

	return true, nil
}

// GetMetadata retrieves metadata for an S3 object without downloading the content.
// This is more efficient than Get() for metadata-only operations.
func (c *Client) GetMetadata(ctx context.Context, bucket, key string) (*s3types.ObjectMetadata, error) {
	if bucket == "" {
		return nil, errors.NewError("getMetadata", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if key == "" {
		return nil, errors.NewError("getMetadata", errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("object key cannot be empty")
	}

	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := c.s3Client.HeadObject(ctx, input)
	if err != nil {
		return nil, errors.NewError("getMetadata", err).WithBucket(bucket).WithKey(key)
	}

	metadata := &s3types.ObjectMetadata{
		ContentType:   aws.ToString(result.ContentType),
		ContentLength: aws.ToInt64(result.ContentLength),
		LastModified:  aws.ToTime(result.LastModified),
		ETag:          aws.ToString(result.ETag),
	}

	// Copy user metadata if present
	if result.Metadata != nil {
		metadata.Metadata = make(map[string]string, len(result.Metadata))
		for k, v := range result.Metadata {
			metadata.Metadata[k] = v
		}
	}

	return metadata, nil
}

// Copy copies an object from one location to another within S3.
// This is a server-side copy operation that doesn't require downloading the data.
// For large objects, this automatically uses multipart copy operations.
func (c *Client) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	if srcBucket == "" {
		return errors.NewError("copy", errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("source bucket name cannot be empty")
	}
	if srcKey == "" {
		return errors.NewError("copy", errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("source object key cannot be empty")
	}
	if dstBucket == "" {
		return errors.NewError("copy", errors.ErrInvalidInput).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage("destination bucket name cannot be empty")
	}
	if dstKey == "" {
		return errors.NewError("copy", errors.ErrInvalidInput).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage("destination object key cannot be empty")
	}

	// Prevent copying to the same location
	if srcBucket == dstBucket && srcKey == dstKey {
		return errors.NewError("copy", errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("cannot copy object to itself")
	}

	// Use the internal copy package for multipart support
	copier := copy.NewCopier(c.s3Client)
	err := copier.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey, nil)
	if err != nil {
		return errors.NewError("copy", err).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage("failed to copy from " + srcBucket + "/" + srcKey)
	}
	return nil
}

// Move moves an object from one location to another by copying it and then deleting the original.
// This operation is atomic for the destination but not for the source (copy-then-delete pattern).
func (c *Client) Move(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	if srcBucket == "" {
		return errors.NewError("move", errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("source bucket name cannot be empty")
	}
	if srcKey == "" {
		return errors.NewError("move", errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("source object key cannot be empty")
	}
	if dstBucket == "" {
		return errors.NewError("move", errors.ErrInvalidInput).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage("destination bucket name cannot be empty")
	}
	if dstKey == "" {
		return errors.NewError("move", errors.ErrInvalidInput).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage("destination object key cannot be empty")
	}

	// Prevent moving to the same location
	if srcBucket == dstBucket && srcKey == dstKey {
		return errors.NewError("move", errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("cannot move object to itself")
	}

	// First copy the object
	err := c.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey)
	if err != nil {
		return errors.NewError("move", err).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("failed to copy object during move")
	}

	// Then delete the original
	err = c.Delete(ctx, srcBucket, srcKey)
	if err != nil {
		return errors.NewError("move", err).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("failed to delete original object after copy")
	}

	return nil
}

// detectContentTypeFromExtension detects content type from file extension
func (c *Client) detectContentTypeFromExtension(path string) string {
	// Fallback to extension-based detection for S3 keys or unknown files
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			return byExt
		}
	}

	return DefaultContentType
}
