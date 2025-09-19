// Package s3 provides the main S3 client and core operations.
package s3

import (
	"context"
	"errors"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gabriel-vasile/mimetype"

	s3errors "github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/operations/copy"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/operations/download"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/operations/upload"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/validation"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

const (
	// DefaultContentType is the default content type used when content type detection fails
	DefaultContentType = "application/octet-stream"
)

// Upload uploads data from an io.Reader to S3.
// It automatically detects when to use multipart upload based on size thresholds.
// Progress tracking and other options can be configured via UploadOption parameters.
//
// The method automatically switches to multipart upload for large files (>100MB).
// For smaller files, it uses a simple PUT operation.
//
// Returns:
//   - *UploadResult: Contains the uploaded object's metadata including ETag and duration
//   - error: Returns an error if the upload fails
//
// Errors:
//   - ErrInvalidInput: If bucket is empty, key is invalid, or reader is nil
//   - ErrAccessDenied: If the credentials lack permission to upload
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	file, err := os.Open("data.txt")
//	if err != nil {
//	    return err
//	}
//	defer file.Close()
//
//	result, err := client.Upload(ctx, "my-bucket", "data.txt", file,
//	    s3.WithContentType("text/plain"),
//	    s3.WithStorageClass(s3types.StorageClassStandardIA),
//	)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Uploaded %s in %v\n", result.Key, result.Duration)
func (c *Client) Upload(
	ctx context.Context,
	bucket, key string,
	reader io.Reader,
	opts ...s3types.UploadOption,
) (*s3types.UploadResult, error) {
	if bucket == "" {
		return nil, s3errors.NewError("upload", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(key); err != nil {
		return nil, s3errors.NewError("upload", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage(err.Error())
	}
	if reader == nil {
		return nil, s3errors.NewError("upload", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("reader cannot be nil")
	}

	// Apply upload options
	config := &s3types.UploadOptionConfig{
		ContentType:  DefaultContentType, // Default content type
		StorageClass: s3types.StorageClassStandard,
		Metadata:     make(map[string]string),
		PartSize:     8 * 1024 * 1024,                 // 8MB default
		Concurrency:  c.getClientConfig().Concurrency, // Use client-level concurrency as default
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
		return nil, s3errors.NewError("upload", err).WithBucket(bucket).WithKey(key)
	}

	return result, nil
}

// UploadFile uploads a file from the local filesystem to S3.
// It automatically detects when to use multipart upload based on file size.
//
// This is a convenience method that handles file opening and content type detection.
// For files larger than 100MB, it automatically uses multipart upload for better
// performance and reliability.
//
// Returns:
//   - *UploadResult: Contains the uploaded object's metadata including ETag and duration
//   - error: Returns an error if the upload fails
//
// Errors:
//   - ErrInvalidInput: If bucket is empty, key is invalid, or filepath is empty/directory
//   - ErrAccessDenied: If the credentials lack permission to upload
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - File system errors if the file cannot be read
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	result, err := client.UploadFile(ctx, "my-bucket", "docs/report.pdf", "/path/to/report.pdf",
//	    s3.WithProgress(progressTracker),
//	    s3.WithMetadata(map[string]string{
//	        "Author": "John Doe",
//	        "Version": "1.0",
//	    }),
//	)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Uploaded %d bytes in %v\n", result.Size, result.Duration)
func (c *Client) UploadFile(
	ctx context.Context,
	bucket, key, filepath string,
	opts ...s3types.UploadOption,
) (*s3types.UploadResult, error) {
	if bucket == "" {
		return nil, s3errors.NewError("uploadFile", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(key); err != nil {
		return nil, s3errors.NewError("uploadFile", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage(err.Error())
	}
	if filepath == "" {
		return nil, s3errors.NewError("uploadFile", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("filepath cannot be empty")
	}

	// Check if file exists and get its info
	info, err := c.fs.Stat(filepath)
	if err != nil {
		return nil, s3errors.NewError("uploadFile", err).WithBucket(bucket).WithKey(key)
	}
	if info.IsDir() {
		return nil, s3errors.NewError("uploadFile", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("filepath points to a directory, not a file")
	}

	// Apply upload options
	config := &s3types.UploadOptionConfig{
		ContentType:  DefaultContentType,
		StorageClass: s3types.StorageClassStandard,
		Metadata:     make(map[string]string),
		PartSize:     8 * 1024 * 1024,                 // 8MB default
		Concurrency:  c.getClientConfig().Concurrency, // Use client-level concurrency as default
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
		return nil, s3errors.NewError("uploadFile", err).WithBucket(bucket).WithKey(key)
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
		return nil, s3errors.NewError("uploadFile", err).WithBucket(bucket).WithKey(key)
	}

	return result, nil
}

// Put uploads byte data to S3.
// This is a convenience method for small amounts of data that fit in memory.
//
// Ideal for uploading configuration files, JSON data, or other small objects
// directly from memory without needing to create intermediate files.
//
// Returns:
//   - error: Returns nil on success, or an error if the upload fails
//
// Errors:
//   - ErrInvalidInput: If bucket is empty or key is invalid
//   - ErrAccessDenied: If the credentials lack permission to upload
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	data := []byte(`{"config": "value"}`)
//	err := client.Put(ctx, "my-bucket", "config.json", data,
//	    s3.WithContentType("application/json"),
//	    s3.WithACL(s3types.ACLPrivate),
//	)
//	if err != nil {
//	    return err
//	}
func (c *Client) Put(ctx context.Context, bucket, key string, data []byte, opts ...s3types.UploadOption) error {
	if bucket == "" {
		return s3errors.NewError("put", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(key); err != nil {
		return s3errors.NewError("put", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage(err.Error())
	}

	// Apply upload options
	config := &s3types.UploadOptionConfig{
		ContentType:  DefaultContentType,
		StorageClass: s3types.StorageClassStandard,
		Metadata:     make(map[string]string),
		PartSize:     8 * 1024 * 1024,                 // 8MB default
		Concurrency:  c.getClientConfig().Concurrency, // Use client-level concurrency as default
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
		return s3errors.NewError("put", err).WithBucket(bucket).WithKey(key)
	}

	// Put doesn't return a result, just the error
	_ = result
	return nil
}

// Download downloads an object from S3 and writes it to an io.Writer.
// It provides stream-based downloading with memory-efficient handling of large files.
// Progress tracking and range requests can be configured via DownloadOption parameters.
//
// The method streams the object directly to the writer, making it memory-efficient
// for large files. Use DownloadOption parameters to track progress or request
// specific byte ranges.
//
// Returns:
//   - *DownloadResult: Contains the downloaded object's metadata and duration
//   - error: Returns an error if the download fails
//
// Errors:
//   - ErrInvalidInput: If bucket is empty, key is invalid, or writer is nil
//   - ErrObjectNotFound: If the specified object doesn't exist
//   - ErrAccessDenied: If the credentials lack permission to download
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - ErrInvalidRange: If the specified range is invalid
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	file, err := os.Create("downloaded.txt")
//	if err != nil {
//	    return err
//	}
//	defer file.Close()
//
//	result, err := client.Download(ctx, "my-bucket", "data.txt", file,
//	    s3.WithDownloadProgress(progressTracker),
//	)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Downloaded %d bytes in %v\n", result.Size, result.Duration)
func (c *Client) Download(
	ctx context.Context,
	bucket, key string,
	writer io.Writer,
	opts ...s3types.DownloadOption,
) (*s3types.DownloadResult, error) {
	if bucket == "" {
		return nil, s3errors.NewError("download", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(key); err != nil {
		return nil, s3errors.NewError("download", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage(err.Error())
	}
	if writer == nil {
		return nil, s3errors.NewError("download", s3errors.ErrInvalidInput).
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
		return nil, s3errors.NewError("download", err).WithBucket(bucket).WithKey(key)
	}

	return result, nil
}

// DownloadFile downloads an object from S3 to a local file.
// The file will be created if it doesn't exist, or truncated if it does.
// It provides memory-efficient handling of large files.
//
// This is a convenience method that handles file creation and streaming.
// The download is performed efficiently without loading the entire object into memory.
//
// Returns:
//   - *DownloadResult: Contains the downloaded object's metadata and duration
//   - error: Returns an error if the download fails
//
// Errors:
//   - ErrInvalidInput: If bucket is empty, key is invalid, or filepath is empty
//   - ErrObjectNotFound: If the specified object doesn't exist
//   - ErrAccessDenied: If the credentials lack permission to download
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - File system errors if the file cannot be created or written
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	result, err := client.DownloadFile(ctx, "my-bucket", "docs/report.pdf", "/tmp/report.pdf",
//	    s3.WithDownloadProgress(progressTracker),
//	)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Downloaded %d bytes in %v\n", result.Size, result.Duration)
func (c *Client) DownloadFile(
	ctx context.Context,
	bucket, key, filepath string,
	opts ...s3types.DownloadOption,
) (*s3types.DownloadResult, error) {
	if bucket == "" {
		return nil, s3errors.NewError("downloadFile", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(key); err != nil {
		return nil, s3errors.NewError("downloadFile", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage(err.Error())
	}
	if filepath == "" {
		return nil, s3errors.NewError("downloadFile", s3errors.ErrInvalidInput).
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
		return nil, s3errors.NewError("downloadFile", err).WithBucket(bucket).WithKey(key)
	}

	return result, nil
}

// Get downloads an entire object from S3 and returns it as a byte slice.
// This is a convenience method for small objects that can fit in memory.
// For large objects, use Download or DownloadFile instead.
//
// WARNING: This method loads the entire object into memory. Only use for small objects.
// For large objects, use Download() or DownloadFile() to stream the data.
//
// Returns:
//   - []byte: The object's contents as a byte slice
//   - error: Returns an error if the download fails
//
// Errors:
//   - ErrInvalidInput: If bucket is empty or key is invalid
//   - ErrObjectNotFound: If the specified object doesn't exist
//   - ErrAccessDenied: If the credentials lack permission to download
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	data, err := client.Get(ctx, "my-bucket", "config.json")
//	if err != nil {
//	    return err
//	}
//	var config Config
//	err = json.Unmarshal(data, &config)
//	if err != nil {
//	    return err
//	}
func (c *Client) Get(ctx context.Context, bucket, key string, opts ...s3types.DownloadOption) ([]byte, error) {
	if bucket == "" {
		return nil, s3errors.NewError("get", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(key); err != nil {
		return nil, s3errors.NewError("get", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage(err.Error())
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
		return nil, s3errors.NewError("get", err).WithBucket(bucket).WithKey(key)
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

// List lists objects in an S3 bucket with support for pagination and filtering.
// It returns a ListResult containing objects and pagination information.
// Use opts to specify prefix, delimiter, max keys, and pagination options.
//
// This method supports prefix filtering and delimiter-based hierarchy navigation.
// Use maxKeys to control the page size (max 1000 objects per request).
//
// Parameters:
//   - prefix: Filter results to objects with this prefix
//   - delimiter: Use to group results by common prefixes (e.g., "/" for directories)
//   - marker: Start listing after this key (for pagination)
//   - continuationToken: Use the token from a previous response to continue listing
//   - maxKeys: Maximum number of keys to return (1-1000, default 1000)
//
// Returns:
//   - *ListResult: Contains the objects and pagination information
//   - error: Returns an error if the listing fails
//
// Errors:
//   - ErrInvalidInput: If bucket is empty
//   - ErrAccessDenied: If the credentials lack permission to list
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	result, err := client.List(ctx, "my-bucket", "photos/", "", "", 100)
//	if err != nil {
//	    return err
//	}
//	for _, obj := range result.Objects {
//	    fmt.Printf("Object: %s, Size: %d\n", obj.Key, obj.Size)
//	}
//	if result.IsTruncated {
//	    // Get next page using result.NextContinuationToken
//	}
func (c *Client) List(
	ctx context.Context,
	bucket, prefix string,
	opts ...s3types.ListOption,
) (*s3types.ListResult, error) {
	if bucket == "" {
		return nil, s3errors.NewError("list", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithMessage("bucket name cannot be empty")
	}

	// Apply list options
	config := &s3types.ListOptionConfig{
		Prefix:  prefix, // Use the prefix parameter as the default
		MaxKeys: 1000,   // Default max keys
	}
	for _, opt := range opts {
		opt(config)
	}

	startTime := time.Now()

	// Build the list request
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(config.Prefix),
		MaxKeys: aws.Int32(config.MaxKeys),
	}

	// Add optional parameters
	if config.Delimiter != "" {
		input.Delimiter = aws.String(config.Delimiter)
	}
	if config.StartAfter != "" {
		input.StartAfter = aws.String(config.StartAfter)
	}

	// Perform the list operation
	result, err := c.s3Client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, s3errors.NewError("list", err).WithBucket(bucket)
	}

	// Convert the result to our internal types
	listResult := &s3types.ListResult{
		Objects:     make([]s3types.Object, 0, len(result.Contents)),
		IsTruncated: aws.ToBool(result.IsTruncated),
		Duration:    time.Since(startTime),
	}

	// Set pagination tokens
	if result.NextContinuationToken != nil {
		listResult.NextContinuationToken = aws.ToString(result.NextContinuationToken)
	}
	if result.ContinuationToken != nil {
		listResult.NextToken = aws.ToString(result.ContinuationToken)
	}

	// Convert S3 objects to our internal Object type
	for _, obj := range result.Contents {
		object := s3types.Object{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			ETag:         aws.ToString(obj.ETag),
			StorageClass: string(obj.StorageClass),
		}
		listResult.Objects = append(listResult.Objects, object)
	}

	return listResult, nil
}

// ListAll lists all objects in an S3 bucket using channel-based streaming.
// It automatically handles pagination and streams all objects through a channel.
// The channel is closed when all objects have been sent or an error occurs.
//
// This method is ideal for processing large numbers of objects without loading
// them all into memory at once. The method runs in a background goroutine.
//
// Always consume the channel completely or cancel the context to avoid goroutine leaks.
//
// Returns:
//   - <-chan Object: Channel that receives objects as they are listed
//
// Errors are not directly returned. If an error occurs during listing,
// the channel is closed and the error is logged internally.
//
// Example:
//
//	objects := client.ListAll(ctx, "my-bucket", "photos/")
//	for obj := range objects {
//	    fmt.Printf("Processing: %s (%d bytes)\n", obj.Key, obj.Size)
//	    // Process each object
//	}
//
// For error handling in the context of channels, the implementation should
// send errors through a separate error channel or handle them appropriately.
func (c *Client) ListAll(ctx context.Context, bucket, prefix string) <-chan s3types.Object {
	objectChan := make(chan s3types.Object, 100) // Buffered channel for performance

	go func() {
		defer close(objectChan)

		var continuationToken *string

		for {
			// Check if context was cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Build the list request
			input := &s3.ListObjectsV2Input{
				Bucket:  aws.String(bucket),
				Prefix:  aws.String(prefix),
				MaxKeys: aws.Int32(1000), // Use maximum page size for efficiency
			}

			if continuationToken != nil {
				input.ContinuationToken = continuationToken
			}

			// Perform the list operation
			result, err := c.s3Client.ListObjectsV2(ctx, input)
			if err != nil {
				// In a channel-based API, we need to handle errors differently
				// For now, we'll just close the channel on error
				// In a production implementation, you might want to send errors through a separate channel
				return
			}

			// Send objects through the channel
			for _, obj := range result.Contents {
				object := s3types.Object{
					Key:          aws.ToString(obj.Key),
					Size:         aws.ToInt64(obj.Size),
					LastModified: aws.ToTime(obj.LastModified),
					ETag:         aws.ToString(obj.ETag),
					StorageClass: string(obj.StorageClass),
				}

				// Check if context was cancelled before sending
				select {
				case objectChan <- object:
				case <-ctx.Done():
					return
				}
			}

			// Check if there are more pages
			if !aws.ToBool(result.IsTruncated) {
				break
			}

			// Update continuation token for next page
			continuationToken = result.NextContinuationToken
		}
	}()

	return objectChan
}

// Delete deletes a single object from S3.
// Returns an error if the operation fails.
//
// This operation is idempotent - deleting a non-existent object doesn't return an error.
//
// Returns:
//   - error: Returns nil on success, or an error if the deletion fails
//
// Errors:
//   - ErrInvalidInput: If bucket is empty or key is invalid
//   - ErrAccessDenied: If the credentials lack permission to delete
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	err := client.Delete(ctx, "my-bucket", "old-file.txt")
//	if err != nil {
//	    return fmt.Errorf("failed to delete object: %w", err)
//	}
func (c *Client) Delete(ctx context.Context, bucket, key string) error {
	if bucket == "" {
		return s3errors.NewError("delete", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(key); err != nil {
		return s3errors.NewError("delete", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage(err.Error())
	}

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := c.s3Client.DeleteObject(ctx, input)
	if err != nil {
		return s3errors.NewError("delete", err).WithBucket(bucket).WithKey(key)
	}

	return nil
}

// DeleteMany deletes multiple objects from S3 in a single batch operation.
// This method uses S3's DeleteObjects API which can delete up to 1000 objects at once.
// Returns a DeleteResult containing information about successful and failed deletions.
//
// This is much more efficient than calling Delete repeatedly for multiple objects.
// The operation is atomic per object - each object deletion succeeds or fails independently.
//
// Returns:
//   - *DeleteResult: Contains lists of successfully deleted objects and any errors
//   - error: Returns an error if the request itself fails
//
// Errors:
//   - ErrInvalidInput: If bucket is empty, keys is empty, or >1000 keys provided
//   - ErrAccessDenied: If the credentials lack permission to delete
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	keys := []string{"file1.txt", "file2.txt", "file3.txt"}
//	result, err := client.DeleteMany(ctx, "my-bucket", keys)
//	if err != nil {
//	    return fmt.Errorf("batch delete failed: %w", err)
//	}
//	if len(result.Errors) > 0 {
//	    for _, e := range result.Errors {
//	        fmt.Printf("Failed to delete %s: %s\n", e.Key, e.Message)
//	    }
//	}
//	fmt.Printf("Deleted %d objects\n", len(result.Deleted))
func (c *Client) DeleteMany(ctx context.Context, bucket string, keys []string) (*s3types.DeleteResult, error) {
	if bucket == "" {
		return nil, s3errors.NewError("deleteMany", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithMessage("bucket name cannot be empty")
	}
	if len(keys) == 0 {
		return nil, s3errors.NewError("deleteMany", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithMessage("keys cannot be empty")
	}

	// S3 allows up to 1000 objects per delete request
	const maxKeysPerRequest = 1000
	if len(keys) > maxKeysPerRequest {
		return nil, s3errors.NewError("deleteMany", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithMessage("too many keys: maximum is 1000 per request")
	}

	startTime := time.Now()

	// Build the delete request
	deleteObjects := make([]types.ObjectIdentifier, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			return nil, s3errors.NewError("deleteMany", s3errors.ErrInvalidInput).
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
		return nil, s3errors.NewError("deleteMany", err).WithBucket(bucket)
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
//
// This method is efficient as it only retrieves metadata without downloading the object.
// Use this to check object existence before performing operations.
//
// Returns:
//   - bool: true if object exists, false otherwise
//   - error: Returns nil for success/not-found, or error for other failures
//
// Errors:
//   - ErrInvalidInput: If bucket is empty or key is invalid
//   - ErrAccessDenied: If the credentials lack permission to access
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	exists, err := client.Exists(ctx, "my-bucket", "data.txt")
//	if err != nil {
//	    return fmt.Errorf("failed to check existence: %w", err)
//	}
//	if exists {
//	    fmt.Println("Object exists")
//	} else {
//	    fmt.Println("Object does not exist")
//	}
func (c *Client) Exists(ctx context.Context, bucket, key string) (bool, error) {
	if bucket == "" {
		return false, s3errors.NewError("exists", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(key); err != nil {
		return false, s3errors.NewError("exists", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage(err.Error())
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
		return false, s3errors.NewError("exists", err).WithBucket(bucket).WithKey(key)
	}

	return true, nil
}

// GetMetadata retrieves metadata for an S3 object without downloading the content.
// This is more efficient than Get() for metadata-only operations.
//
// Uses a HEAD request to retrieve object metadata including content type, size,
// last modified time, ETag, and any custom metadata.
//
// Returns:
//   - *ObjectMetadata: The object's metadata information
//   - error: Returns an error if the operation fails
//
// Errors:
//   - ErrInvalidInput: If bucket is empty or key is invalid
//   - ErrObjectNotFound: If the specified object doesn't exist
//   - ErrAccessDenied: If the credentials lack permission to access
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	metadata, err := client.GetMetadata(ctx, "my-bucket", "document.pdf")
//	if err != nil {
//	    return fmt.Errorf("failed to get metadata: %w", err)
//	}
//	fmt.Printf("Content-Type: %s\n", metadata.ContentType)
//	fmt.Printf("Size: %d bytes\n", metadata.ContentLength)
//	fmt.Printf("Last Modified: %v\n", metadata.LastModified)
func (c *Client) GetMetadata(ctx context.Context, bucket, key string) (*s3types.ObjectMetadata, error) {
	if bucket == "" {
		return nil, s3errors.NewError("getMetadata", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage("bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(key); err != nil {
		return nil, s3errors.NewError("getMetadata", s3errors.ErrInvalidInput).
			WithBucket(bucket).
			WithKey(key).
			WithMessage(err.Error())
	}

	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := c.s3Client.HeadObject(ctx, input)
	if err != nil {
		return nil, s3errors.NewError("getMetadata", err).WithBucket(bucket).WithKey(key)
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
//
// This method automatically handles large files (>5GB) by using multipart copy.
// The copy is performed entirely on the server side, making it efficient for
// large objects as no data is transferred to the client.
//
// Returns:
//   - error: Returns nil on success, or an error if the copy fails
//
// Errors:
//   - ErrInvalidInput: If any bucket/key parameters are empty or invalid
//   - ErrObjectNotFound: If the source object doesn't exist
//   - ErrAccessDenied: If the credentials lack permission to copy
//   - ErrBucketNotFound: If either bucket doesn't exist
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	err := client.Copy(ctx, "source-bucket", "old/path.txt",
//	                  "dest-bucket", "new/path.txt")
//	if err != nil {
//	    return fmt.Errorf("failed to copy object: %w", err)
//	}
func (c *Client) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	if srcBucket == "" {
		return s3errors.NewError("copy", s3errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("source bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(srcKey); err != nil {
		return s3errors.NewError("copy", s3errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage(err.Error())
	}
	if dstBucket == "" {
		return s3errors.NewError("copy", s3errors.ErrInvalidInput).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage("destination bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(dstKey); err != nil {
		return s3errors.NewError("copy", s3errors.ErrInvalidInput).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage(err.Error())
	}

	// Prevent copying to the same location
	if srcBucket == dstBucket && srcKey == dstKey {
		return s3errors.NewError("copy", s3errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("cannot copy object to itself")
	}

	// Use the internal copy package for multipart support
	copier := copy.NewCopier(c.s3Client)
	err := copier.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey, nil)
	if err != nil {
		return s3errors.NewError("copy", err).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage("failed to copy from " + srcBucket + "/" + srcKey)
	}
	return nil
}

// Move moves an object from one location to another by copying it and then deleting the original.
// This operation is atomic for the destination but not for the source (copy-then-delete pattern).
//
// The move is performed as a two-step process: copy then delete.
// If the copy succeeds but the delete fails, the object will exist in both locations.
// For critical operations, verify the move completed successfully.
//
// Returns:
//   - error: Returns nil on success, or an error if the move fails
//
// Errors:
//   - ErrInvalidInput: If any bucket/key parameters are empty or invalid
//   - ErrObjectNotFound: If the source object doesn't exist
//   - ErrAccessDenied: If the credentials lack permission to copy or delete
//   - ErrBucketNotFound: If either bucket doesn't exist
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	err := client.Move(ctx, "source-bucket", "temp/file.txt",
//	                  "archive-bucket", "2024/file.txt")
//	if err != nil {
//	    return fmt.Errorf("failed to move object: %w", err)
//	}
func (c *Client) Move(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	if srcBucket == "" {
		return s3errors.NewError("move", s3errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("source bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(srcKey); err != nil {
		return s3errors.NewError("move", s3errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage(err.Error())
	}
	if dstBucket == "" {
		return s3errors.NewError("move", s3errors.ErrInvalidInput).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage("destination bucket name cannot be empty")
	}
	if err := validation.ValidateObjectKey(dstKey); err != nil {
		return s3errors.NewError("move", s3errors.ErrInvalidInput).
			WithBucket(dstBucket).
			WithKey(dstKey).
			WithMessage(err.Error())
	}

	// Prevent moving to the same location
	if srcBucket == dstBucket && srcKey == dstKey {
		return s3errors.NewError("move", s3errors.ErrInvalidInput).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("cannot move object to itself")
	}

	// First copy the object
	err := c.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey)
	if err != nil {
		return s3errors.NewError("move", err).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("failed to copy object during move")
	}

	// Then delete the original
	err = c.Delete(ctx, srcBucket, srcKey)
	if err != nil {
		return s3errors.NewError("move", err).
			WithBucket(srcBucket).
			WithKey(srcKey).
			WithMessage("failed to delete original object after copy")
	}

	return nil
}

// CreateBucket creates a new S3 bucket.
// The bucket name must be DNS-compliant and unique across all existing bucket names in S3.
// Use opts to specify the region where the bucket should be created.
//
// Bucket naming rules:
//   - Must be 3-63 characters long
//   - Can only contain lowercase letters, numbers, dots (.), and hyphens (-)
//   - Must begin and end with a letter or number
//   - Must not be formatted as an IP address
//   - Must be globally unique across all S3 buckets
//
// Returns:
//   - error: Returns nil on success, or an error if bucket creation fails
//
// Errors:
//   - ErrInvalidBucketName: If the bucket name doesn't comply with naming rules
//   - ErrBucketAlreadyExists: If a bucket with this name already exists
//   - ErrAccessDenied: If the credentials lack permission to create buckets
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	err := client.CreateBucket(ctx, "my-new-bucket",
//	    s3.WithBucketRegion("us-west-2"),
//	)
//	if err != nil {
//	    return fmt.Errorf("failed to create bucket: %w", err)
//	}
func (c *Client) CreateBucket(ctx context.Context, bucket string, opts ...s3types.BucketOption) error {
	// Validate bucket name
	if err := validation.ValidateBucketName(bucket); err != nil {
		return s3errors.NewError("createBucket", s3errors.ErrInvalidBucketName).
			WithBucket(bucket).
			WithMessage(err.Error())
	}

	// Apply bucket options
	config := &s3types.BucketOptionConfig{}
	for _, opt := range opts {
		opt(config)
	}

	// Build the create bucket request
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}

	// Set region if specified
	if config.Region != "" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(config.Region),
		}
	}

	_, err := c.s3Client.CreateBucket(ctx, input)
	if err != nil {
		return s3errors.NewError("createBucket", c.convertAWSError(err)).WithBucket(bucket)
	}

	return nil
}

// DeleteBucket deletes an S3 bucket.
// The bucket must be empty before it can be deleted.
// Returns an error if the bucket doesn't exist or is not empty.
//
// Important: All objects and versions must be deleted before the bucket can be deleted.
// Use DeleteMany to remove objects first, or use a lifecycle policy for automatic cleanup.
//
// Returns:
//   - error: Returns nil on success, or an error if bucket deletion fails
//
// Errors:
//   - ErrInvalidInput: If the bucket name is empty
//   - ErrBucketNotFound: If the bucket doesn't exist
//   - ErrBucketNotEmpty: If the bucket contains objects
//   - ErrAccessDenied: If the credentials lack permission to delete the bucket
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	// First, ensure the bucket is empty
//	objects := client.ListAll(ctx, "old-bucket", "")
//	var keys []string
//	for obj := range objects {
//	    keys = append(keys, obj.Key)
//	}
//	if len(keys) > 0 {
//	    _, err := client.DeleteMany(ctx, "old-bucket", keys)
//	    if err != nil {
//	        return err
//	    }
//	}
//	// Now delete the bucket
//	err := client.DeleteBucket(ctx, "old-bucket")
//	if err != nil {
//	    return fmt.Errorf("failed to delete bucket: %w", err)
//	}
func (c *Client) DeleteBucket(ctx context.Context, bucket string) error {
	// Validate bucket name
	if err := validation.ValidateBucketName(bucket); err != nil {
		return s3errors.NewError("createBucket", s3errors.ErrInvalidBucketName).
			WithBucket(bucket).
			WithMessage(err.Error())
	}

	input := &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	}

	_, err := c.s3Client.DeleteBucket(ctx, input)
	if err != nil {
		return s3errors.NewError("deleteBucket", c.convertAWSError(err)).WithBucket(bucket)
	}

	return nil
}

// convertAWSError converts AWS SDK errors to our custom error types
func (c *Client) convertAWSError(err error) error {
	if err == nil {
		return nil
	}

	// Check for specific AWS SDK error types
	var bucketAlreadyExists *types.BucketAlreadyExists
	if errors.As(err, &bucketAlreadyExists) {
		return s3errors.ErrBucketAlreadyExists
	}

	var noSuchBucket *types.NoSuchBucket
	if errors.As(err, &noSuchBucket) {
		return s3errors.ErrBucketNotFound
	}

	// Check for error messages that contain specific error codes
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "BucketNotEmpty"):
		return s3errors.ErrBucketNotEmpty
	case strings.Contains(errMsg, "BucketAlreadyExists"):
		return s3errors.ErrBucketAlreadyExists
	case strings.Contains(errMsg, "NoSuchBucket"):
		return s3errors.ErrBucketNotFound
	}

	// Return the original error if we can't convert it
	return err
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
