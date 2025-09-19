// Package download handles S3 object download operations.
// This includes stream-based downloads, file downloads, and range requests.
//
// The package provides memory-efficient streaming for large files and
// supports progress tracking during download operations.
package download

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/s3api"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Downloader handles S3 download operations with progress tracking support.
type Downloader struct {
	s3Client s3api.S3API
}

// New creates a new Downloader instance.
func New(s3Client s3api.S3API) *Downloader {
	return &Downloader{
		s3Client: s3Client,
	}
}

// Download downloads an object from S3 and writes it to an io.Writer.
// This provides stream-based downloading with memory-efficient handling of large files.
func (d *Downloader) Download(
	ctx context.Context,
	bucket, key string,
	writer io.Writer,
	config *s3types.DownloadConfig,
	startTime time.Time,
) (*s3types.DownloadResult, error) {
	// Prepare the GetObject input
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	// Set range if specified
	if config.RangeSpec != "" {
		input.Range = aws.String(config.RangeSpec)
	}

	// Perform the download
	output, err := d.s3Client.GetObject(ctx, input)
	if err != nil {
		// Convert AWS SDK errors to sentinel errors
		if isObjectNotFound(err) {
			return nil, errors.NewError("download", errors.ErrObjectNotFound).WithBucket(bucket).WithKey(key)
		}
		return nil, errors.NewError("download", err).WithBucket(bucket).WithKey(key)
	}
	defer output.Body.Close()

	// Get the content length
	size := int64(0)
	if output.ContentLength != nil {
		size = *output.ContentLength
	}

	// Create a progress reader if progress tracking is enabled
	var reader io.Reader = output.Body
	if config.ProgressTracker != nil {
		reader = &progressReader{
			reader:          output.Body,
			progressTracker: config.ProgressTracker,
			total:           size,
			bytesRead:       0,
		}
	}

	// Copy the data to the writer
	bytesWritten, err := io.Copy(writer, reader)
	if err != nil {
		return nil, errors.NewError("download", err).WithBucket(bucket).WithKey(key)
	}

	// Update size if ContentLength was not provided
	if size == 0 {
		size = bytesWritten
	}

	// Call progress tracker completion
	if config.ProgressTracker != nil {
		config.ProgressTracker.Update(bytesWritten, size)
		config.ProgressTracker.Complete()
	}

	// Create the result
	result := &s3types.DownloadResult{
		Key:       key,
		Size:      size,
		ETag:      aws.ToString(output.ETag),
		VersionID: aws.ToString(output.VersionId),
		Duration:  time.Since(startTime),
	}

	return result, nil
}

// DownloadFile downloads an object from S3 to a local file.
// The file will be created if it doesn't exist, or truncated if it does.
func (d *Downloader) DownloadFile(
	ctx context.Context,
	bucket, key, filepath string,
	config *s3types.DownloadConfig,
	startTime time.Time,
) (*s3types.DownloadResult, error) {
	// Open the file for writing
	file, err := os.Create(filepath)
	if err != nil {
		return nil, errors.NewError("downloadFile", err).WithBucket(bucket).WithKey(key)
	}
	defer file.Close()

	// Use the stream download method
	return d.Download(ctx, bucket, key, file, config, startTime)
}

// Get downloads an entire object from S3 and returns it as a byte slice.
// This is a convenience method for small objects that can fit in memory.
func (d *Downloader) Get(
	ctx context.Context,
	bucket, key string,
	config *s3types.DownloadConfig,
	startTime time.Time,
) ([]byte, error) {
	var buf bytes.Buffer
	_, err := d.Download(ctx, bucket, key, &buf, config, startTime)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// progressReader wraps an io.Reader to track progress
type progressReader struct {
	reader          io.Reader
	progressTracker s3types.ProgressTracker
	total           int64
	bytesRead       int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.bytesRead += int64(n)
		if pr.progressTracker != nil {
			pr.progressTracker.Update(pr.bytesRead, pr.total)
		}
	}
	//nolint:wrapcheck // io.Reader interface contract - error comes from underlying reader
	return n, err
}

// isObjectNotFound checks if an error indicates that an object was not found.
// This is a temporary implementation - in production, this should check AWS SDK error types.
func isObjectNotFound(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "NoSuchKey") || strings.Contains(errStr, "NotFound")
}
