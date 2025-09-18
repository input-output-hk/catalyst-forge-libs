// Package s3 provides the main S3 client and core operations.
package s3

import (
	"context"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
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
