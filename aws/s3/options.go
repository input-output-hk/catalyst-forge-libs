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

// WithACL sets the access control list for upload operations.
// Defaults to private if not specified.
func WithACL(acl s3types.ObjectACL) s3types.UploadOption {
	return func(c *s3types.UploadOptionConfig) {
		c.ACL = acl
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

// WithDownloadProgress sets a progress tracker for download operations.
func WithDownloadProgress(tracker s3types.ProgressTracker) s3types.DownloadOption {
	return func(c *s3types.DownloadOptionConfig) {
		c.ProgressTracker = tracker
	}
}

// WithRange sets a range specification for download operations.
// The rangeSpec should be in HTTP Range header format (e.g., "bytes=0-1023").
func WithRange(rangeSpec string) s3types.DownloadOption {
	return func(c *s3types.DownloadOptionConfig) {
		c.RangeSpec = rangeSpec
	}
}

// WithPrefix sets the prefix filter for list operations.
// Only objects with keys that start with this prefix will be returned.
func WithPrefix(prefix string) s3types.ListOption {
	return func(c *s3types.ListOptionConfig) {
		c.Prefix = prefix
	}
}

// WithDelimiter sets the delimiter for list operations.
// This enables hierarchical listing where common prefixes are grouped together.
// Common values are "/" for directory-like listings.
func WithDelimiter(delimiter string) s3types.ListOption {
	return func(c *s3types.ListOptionConfig) {
		c.Delimiter = delimiter
	}
}

// WithMaxKeys sets the maximum number of keys to return in a list operation.
// This controls pagination size. Valid range is 1-1000.
// Default is 1000 if not specified.
func WithMaxKeys(maxKeys int32) s3types.ListOption {
	return func(c *s3types.ListOptionConfig) {
		if maxKeys > 0 && maxKeys <= 1000 {
			c.MaxKeys = maxKeys
		}
	}
}

// WithStartAfter sets the starting point for list operations.
// Only objects with keys that occur lexicographically after this value will be returned.
// This is useful for pagination and resuming interrupted listings.
func WithStartAfter(startAfter string) s3types.ListOption {
	return func(c *s3types.ListOptionConfig) {
		c.StartAfter = startAfter
	}
}

// WithBucketRegion sets the region for bucket operations.
// This is used to specify the region when creating buckets in regions other than us-east-1.
func WithBucketRegion(region string) s3types.BucketOption {
	return func(c *s3types.BucketOptionConfig) {
		c.Region = region
	}
}

// Sync Options

// WithSyncDryRun enables dry-run mode for sync operations.
// When enabled, the sync will only report what operations would be performed
// without actually executing them.
func WithSyncDryRun(dryRun bool) s3types.SyncOption {
	return func(c *s3types.SyncOptionConfig) {
		c.DryRun = dryRun
	}
}

// WithSyncDeleteExtra enables deletion of extra files in the destination.
// When enabled, files that exist in the destination but not in the source
// will be deleted.
func WithSyncDeleteExtra(deleteExtra bool) s3types.SyncOption {
	return func(c *s3types.SyncOptionConfig) {
		c.DeleteExtra = deleteExtra
	}
}

// WithSyncIncludePattern adds an include pattern for sync operations.
// Only files matching one of the include patterns will be synced.
// Patterns use glob syntax (e.g., "*.txt", "**/*.go").
func WithSyncIncludePattern(pattern string) s3types.SyncOption {
	return func(c *s3types.SyncOptionConfig) {
		c.IncludePatterns = append(c.IncludePatterns, pattern)
	}
}

// WithSyncExcludePattern adds an exclude pattern for sync operations.
// Files matching any exclude pattern will be skipped.
// Patterns use glob syntax (e.g., "*.tmp", "**/node_modules/**").
func WithSyncExcludePattern(pattern string) s3types.SyncOption {
	return func(c *s3types.SyncOptionConfig) {
		c.ExcludePatterns = append(c.ExcludePatterns, pattern)
	}
}

// WithSyncProgressTracker sets a progress tracker for sync operations.
// The tracker will receive updates about the sync progress.
func WithSyncProgressTracker(tracker s3types.ProgressTracker) s3types.SyncOption {
	return func(c *s3types.SyncOptionConfig) {
		c.ProgressTracker = tracker
	}
}

// WithSyncParallelism sets the number of parallel operations for sync.
// This controls how many upload/download operations can run concurrently.
// Default is 5 if not specified.
func WithSyncParallelism(parallelism int) s3types.SyncOption {
	return func(c *s3types.SyncOptionConfig) {
		c.Parallelism = parallelism
	}
}

// WithSyncComparator sets a custom file comparator for sync operations.
// This allows customization of how files are compared to determine if they need syncing.
// If not specified, SmartComparator is used by default.
func WithSyncComparator(comparator s3types.FileComparator) s3types.SyncOption {
	return func(c *s3types.SyncOptionConfig) {
		c.Comparator = comparator
	}
}
