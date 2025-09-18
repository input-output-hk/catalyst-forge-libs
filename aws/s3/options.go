// Package s3 provides functional options for configuring S3 client behavior.
// These options follow the functional options pattern for clean, composable configuration.
package s3

import (
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/input-output-hk/catalyst-forge-libs/fs"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// WithRegion sets the AWS region for S3 operations.
// If not specified, uses the default AWS region from the credential chain.
func WithRegion(region string) s3types.Option {
	return func(c *s3types.ClientConfig) {
		c.Region = region
	}
}

// WithMaxRetries sets the maximum number of retry attempts for failed operations.
// Default is 3 retries. Set to 0 to disable retries.
func WithMaxRetries(maxRetries int) s3types.Option {
	return func(c *s3types.ClientConfig) {
		c.MaxRetries = maxRetries
	}
}

// WithTimeout sets the timeout for individual S3 operations.
// Default is no timeout (0). Values should be positive durations.
func WithTimeout(timeout time.Duration) s3types.Option {
	return func(c *s3types.ClientConfig) {
		c.Timeout = timeout
	}
}

// WithConcurrency sets the maximum number of concurrent operations.
// This affects multipart uploads and batch operations.
// Default is 5 concurrent operations.
func WithConcurrency(concurrency int) s3types.Option {
	return func(c *s3types.ClientConfig) {
		if concurrency > 0 {
			c.Concurrency = concurrency
		}
	}
}

// WithPartSize sets the part size for multipart uploads.
// Default is 8MB. Must be at least 5MB for S3 multipart uploads.
func WithPartSize(partSize int64) s3types.Option {
	return func(c *s3types.ClientConfig) {
		if partSize > 0 {
			c.PartSize = partSize
		}
	}
}

// WithForcePathStyle forces the use of path-style URLs instead of virtual-hosted style.
// This is required for S3-compatible services that don't support virtual hosting.
// Default is false (uses virtual-hosted style).
func WithForcePathStyle(forcePathStyle bool) s3types.Option {
	return func(c *s3types.ClientConfig) {
		c.ForcePathStyle = forcePathStyle
	}
}

// WithAWSConfig allows providing a custom AWS configuration.
// This overrides the default configuration loading behavior.
// Use this when you need fine-grained control over AWS SDK configuration.
func WithAWSConfig(config *aws.Config) s3types.Option {
	return func(c *s3types.ClientConfig) {
		// Store the custom config for later use
		c.CustomAWSConfig = config
	}
}

// WithEndpoint sets a custom S3 endpoint URL.
// This is useful for S3-compatible services or local testing with LocalStack.
func WithEndpoint(endpoint string) s3types.Option {
	return func(c *s3types.ClientConfig) {
		c.Endpoint = endpoint
	}
}

// WithDisableSSL disables SSL/TLS for S3 connections.
// Only use this for local testing or S3-compatible services that don't support SSL.
// Default is false (SSL enabled).
func WithDisableSSL(disableSSL bool) s3types.Option {
	return func(c *s3types.ClientConfig) {
		c.DisableSSL = disableSSL
	}
}

// WithConcurrencyLimit sets an upper bound on concurrent S3 operations.
// This is an alias for WithConcurrency for backward compatibility.
func WithConcurrencyLimit(limit int) s3types.Option {
	return WithConcurrency(limit)
}

// WithRetryMode sets the retry mode for AWS SDK operations.
// Options are "standard", "adaptive". Default is "standard".
func WithRetryMode(mode string) s3types.Option {
	return func(c *s3types.ClientConfig) {
		c.RetryMode = mode
	}
}

// WithCustomHTTPClient allows providing a custom HTTP client.
// This gives full control over HTTP behavior including timeouts, proxies, etc.
func WithCustomHTTPClient(client *http.Client) s3types.Option {
	return func(c *s3types.ClientConfig) {
		c.CustomHTTPClient = client
	}
}

// WithDefaultBucket sets a default bucket for operations that don't specify one.
// This can be overridden on a per-operation basis.
func WithDefaultBucket(bucket string) s3types.Option {
	return func(c *s3types.ClientConfig) {
		c.DefaultBucket = bucket
	}
}

// WithFilesystem sets a custom filesystem implementation for file operations.
// This allows using in-memory filesystems for testing or virtual filesystems.
// If not specified, defaults to the OS filesystem.
func WithFilesystem(filesystem fs.Filesystem) s3types.Option {
	return func(c *s3types.ClientConfig) {
		c.Filesystem = filesystem
	}
}

// WithContentType sets the content type for upload operations.
func WithContentType(contentType string) s3types.UploadOption {
	return func(c *s3types.UploadOptionConfig) {
		c.ContentType = contentType
	}
}

// WithMetadata sets metadata for upload operations.
func WithMetadata(metadata map[string]string) s3types.UploadOption {
	return func(c *s3types.UploadOptionConfig) {
		if c.Metadata == nil {
			c.Metadata = make(map[string]string)
		}
		for k, v := range metadata {
			c.Metadata[k] = v
		}
	}
}

// WithStorageClass sets the storage class for upload operations.
func WithStorageClass(storageClass s3types.StorageClass) s3types.UploadOption {
	return func(c *s3types.UploadOptionConfig) {
		c.StorageClass = storageClass
	}
}

// WithServerSideEncryption sets server-side encryption configuration for upload operations.
func WithServerSideEncryption(sse *s3types.SSEConfig) s3types.UploadOption {
	return func(c *s3types.UploadOptionConfig) {
		c.SSE = sse
	}
}

// WithProgress sets a progress tracker for upload operations.
func WithProgress(tracker s3types.ProgressTracker) s3types.UploadOption {
	return func(c *s3types.UploadOptionConfig) {
		c.ProgressTracker = tracker
	}
}

// WithUploadPartSize sets the part size for multipart uploads in upload operations.
// This overrides the client-level default for this specific upload.
func WithUploadPartSize(partSize int64) s3types.UploadOption {
	return func(c *s3types.UploadOptionConfig) {
		if partSize > 0 {
			c.PartSize = partSize
		}
	}
}

// WithUploadConcurrency sets the concurrency level for multipart uploads in upload operations.
// This overrides the client-level default for this specific upload.
func WithUploadConcurrency(concurrency int) s3types.UploadOption {
	return func(c *s3types.UploadOptionConfig) {
		if concurrency > 0 {
			c.Concurrency = concurrency
		}
	}
}
