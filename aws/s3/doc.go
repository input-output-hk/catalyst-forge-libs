// Package s3 provides a high-level Go module for AWS S3 operations.
// It wraps AWS SDK v2 to provide an intuitive and efficient interface
// for common S3 operations while maintaining flexibility for advanced use cases.
//
// The module emphasizes developer experience through simple APIs while
// maintaining performance through intelligent defaults for concurrency,
// buffering, and retries.
//
// Key features:
//   - Simple, zero-configuration usage with AWS credential chain
//   - Progressive enhancement through functional options
//   - Automatic multipart upload for large files
//   - Concurrent operations with configurable limits
//   - Comprehensive error handling with context
//   - Sync functionality for directory synchronization
//
// Example usage:
//
//	client, err := s3.New(ctx)
//	if err != nil {
//	    return err
//	}
//
//	// Upload a file
//	result, err := client.UploadFile(ctx, "my-bucket", "path/file.txt", "/local/file.txt")
//	if err != nil {
//	    return err
//	}
package s3
