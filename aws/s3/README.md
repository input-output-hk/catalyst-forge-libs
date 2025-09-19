# AWS S3 Go Module

[![Go Reference](https://pkg.go.dev/badge/github.com/input-output-hk/catalyst-forge-libs/aws/s3.svg)](https://pkg.go.dev/github.com/input-output-hk/catalyst-forge-libs/aws/s3)
[![Go Report Card](https://goreportcard.com/badge/github.com/input-output-hk/catalyst-forge-libs/aws/s3)](https://goreportcard.com/report/github.com/input-output-hk/catalyst-forge-libs/aws/s3)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

A high-level Go module for AWS S3 operations that provides an intuitive and efficient interface built on AWS SDK v2. This module emphasizes developer experience through simple APIs while maintaining performance through intelligent defaults for concurrency, buffering, and retries.

## Features

- 🚀 **Simple, zero-configuration usage** with AWS credential chain
- 📦 **Automatic multipart upload** for large files (>100MB)
- ⚡ **Concurrent operations** with configurable limits
- 🔄 **Directory synchronization** with include/exclude patterns
- 📊 **Progress tracking** for uploads and downloads
- 🛡️ **Comprehensive error handling** with contextual information
- 🔐 **Server-side encryption** support (SSE-S3, SSE-KMS, SSE-C)
- 🏷️ **Metadata and tagging** support
- 💾 **Memory-efficient streaming** for large files
- 🧪 **Testing-friendly** design with interface-based architecture

## Installation

```bash
go get github.com/input-output-hk/catalyst-forge-libs/aws/s3
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/input-output-hk/catalyst-forge-libs/aws/s3"
)

func main() {
    // Create a new S3 client with default AWS credentials
    client, err := s3.New()
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Upload a file
    result, err := client.UploadFile(ctx, "my-bucket", "documents/report.pdf", "/local/report.pdf")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Uploaded %d bytes in %v\n", result.Size, result.Duration)

    // Download a file
    _, err = client.DownloadFile(ctx, "my-bucket", "documents/report.pdf", "/tmp/downloaded.pdf")
    if err != nil {
        log.Fatal(err)
    }

    // List objects
    objects := client.ListAll(ctx, "my-bucket", "documents/")
    for obj := range objects {
        fmt.Printf("%s - %d bytes\n", obj.Key, obj.Size)
    }
}
```

## Configuration

### Client Options

Configure the client using functional options:

```go
client, err := s3.New(
    s3.WithRegion("us-west-2"),
    s3.WithMaxRetries(5),
    s3.WithConcurrency(10),
    s3.WithTimeout(30 * time.Second),
    s3.WithForcePathStyle(true), // For S3-compatible services
)
```

### Custom AWS Configuration

For fine-grained control over AWS configuration:

```go
awsCfg, err := config.LoadDefaultConfig(ctx,
    config.WithRegion("eu-west-1"),
    config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
        "ACCESS_KEY_ID",
        "SECRET_ACCESS_KEY",
        "SESSION_TOKEN",
    )),
)
if err != nil {
    log.Fatal(err)
}

client, err := s3.New(s3.WithAWSConfig(&awsCfg))
```

### S3-Compatible Services

For MinIO, LocalStack, or other S3-compatible services:

```go
client, err := s3.New(
    s3.WithEndpoint("http://localhost:9000"),
    s3.WithForcePathStyle(true),
    s3.WithDisableSSL(true), // Only for local testing
)
```

## Core Operations

### Upload Operations

#### Upload from io.Reader
```go
file, err := os.Open("data.txt")
if err != nil {
    return err
}
defer file.Close()

result, err := client.Upload(ctx, "my-bucket", "data.txt", file,
    s3.WithContentType("text/plain"),
    s3.WithMetadata(map[string]string{
        "Author": "John Doe",
        "Version": "1.0",
    }),
    s3.WithProgress(progressTracker),
)
```

#### Upload File
```go
result, err := client.UploadFile(ctx, "my-bucket", "images/photo.jpg", "/path/to/photo.jpg",
    s3.WithStorageClass(s3types.StorageClassStandardIA),
    s3.WithACL(s3types.ACLPrivate),
)
```

#### Upload Bytes
```go
data := []byte(`{"config": "value"}`)
err := client.Put(ctx, "my-bucket", "config.json", data,
    s3.WithContentType("application/json"),
)
```

### Download Operations

#### Download to io.Writer
```go
file, err := os.Create("downloaded.txt")
if err != nil {
    return err
}
defer file.Close()

result, err := client.Download(ctx, "my-bucket", "data.txt", file,
    s3.WithDownloadProgress(progressTracker),
)
```

#### Download File
```go
result, err := client.DownloadFile(ctx, "my-bucket", "report.pdf", "/tmp/report.pdf")
```

#### Get as Bytes
```go
data, err := client.Get(ctx, "my-bucket", "config.json")
if err != nil {
    return err
}

var config Config
err = json.Unmarshal(data, &config)
```

### List Operations

#### List with Pagination
```go
result, err := client.List(ctx, "my-bucket", "photos/",
    s3.WithMaxKeys(100),
    s3.WithDelimiter("/"),
)
if err != nil {
    return err
}

for _, obj := range result.Objects {
    fmt.Printf("%s - %d bytes\n", obj.Key, obj.Size)
}

if result.IsTruncated {
    // Use result.NextContinuationToken for next page
}
```

#### List All (Streaming)
```go
objects := client.ListAll(ctx, "my-bucket", "photos/")
for obj := range objects {
    fmt.Printf("Processing: %s (%d bytes)\n", obj.Key, obj.Size)
}
```

### Delete Operations

#### Delete Single Object
```go
err := client.Delete(ctx, "my-bucket", "old-file.txt")
```

#### Delete Multiple Objects
```go
keys := []string{"file1.txt", "file2.txt", "file3.txt"}
result, err := client.DeleteMany(ctx, "my-bucket", keys)
if err != nil {
    return err
}

fmt.Printf("Deleted %d objects\n", len(result.Deleted))
for _, e := range result.Errors {
    fmt.Printf("Failed to delete %s: %s\n", e.Key, e.Message)
}
```

### Management Operations

#### Check Object Existence
```go
exists, err := client.Exists(ctx, "my-bucket", "data.txt")
if err != nil {
    return err
}

if exists {
    fmt.Println("Object exists")
}
```

#### Get Object Metadata
```go
metadata, err := client.GetMetadata(ctx, "my-bucket", "document.pdf")
if err != nil {
    return err
}

fmt.Printf("Content-Type: %s\n", metadata.ContentType)
fmt.Printf("Size: %d bytes\n", metadata.ContentLength)
fmt.Printf("Last Modified: %v\n", metadata.LastModified)
```

#### Copy Object
```go
err := client.Copy(ctx, "source-bucket", "file.txt", "dest-bucket", "backup/file.txt")
```

#### Move Object
```go
err := client.Move(ctx, "temp-bucket", "processing/file.txt", "final-bucket", "complete/file.txt")
```

### Bucket Operations

#### Create Bucket
```go
err := client.CreateBucket(ctx, "new-bucket",
    s3.WithBucketRegion("us-west-2"),
)
```

#### Delete Bucket
```go
// Bucket must be empty
err := client.DeleteBucket(ctx, "old-bucket")
```

## Directory Synchronization

Synchronize local directories with S3:

```go
// Upload only (never deletes from S3)
result, err := client.SyncUpload(ctx, "/local/photos", "my-bucket", "backup/photos/",
    s3.WithSyncIncludePattern("*.jpg"),
    s3.WithSyncExcludePattern("*.tmp"),
    s3.WithSyncProgressTracker(tracker),
)

// Full sync with deletion
result, err := client.Sync(ctx, "/local/data", "my-bucket", "data/",
    s3.WithSyncDeleteExtra(true), // Delete S3 objects not in local
    s3.WithSyncDryRun(true),      // Preview changes without applying
)

fmt.Printf("Uploaded: %d files (%d bytes)\n", result.FilesUploaded, result.BytesUploaded)
fmt.Printf("Skipped: %d unchanged files\n", result.FilesSkipped)
fmt.Printf("Deleted: %d files\n", result.FilesDeleted)
```

## Progress Tracking

Implement custom progress tracking:

```go
type ProgressTracker struct {
    totalBytes     int64
    transferredBytes int64
    mu             sync.Mutex
}

func (p *ProgressTracker) Update(transferred, total int64) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.transferredBytes = transferred
    p.totalBytes = total

    percentage := float64(transferred) / float64(total) * 100
    fmt.Printf("\rProgress: %.2f%% (%d/%d bytes)", percentage, transferred, total)
}

func (p *ProgressTracker) Complete() {
    fmt.Println("\nTransfer complete!")
}

func (p *ProgressTracker) Error(err error) {
    fmt.Printf("\nTransfer failed: %v\n", err)
}

// Use the tracker
tracker := &ProgressTracker{}
result, err := client.UploadFile(ctx, "my-bucket", "large-file.zip", "/path/to/file.zip",
    s3.WithProgress(tracker),
)
```

## Server-Side Encryption

### SSE-S3 (S3-Managed Keys)
```go
result, err := client.UploadFile(ctx, "my-bucket", "secure.txt", "/path/to/file.txt",
    s3.WithServerSideEncryption(&s3types.SSEConfig{
        Type: s3types.SSES3,
    }),
)
```

### SSE-KMS (KMS-Managed Keys)
```go
result, err := client.UploadFile(ctx, "my-bucket", "secure.txt", "/path/to/file.txt",
    s3.WithServerSideEncryption(&s3types.SSEConfig{
        Type:     s3types.SSEKMS,
        KMSKeyID: "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012",
    }),
)
```

### SSE-C (Customer-Provided Keys)
```go
customerKey := "32-byte-base64-encoded-string-here"
result, err := client.UploadFile(ctx, "my-bucket", "secure.txt", "/path/to/file.txt",
    s3.WithServerSideEncryption(&s3types.SSEConfig{
        Type:           s3types.SSEC,
        CustomerKey:    customerKey,
        CustomerKeyMD5: calculateMD5(customerKey),
    }),
)
```

## Error Handling

The module provides detailed error information with context:

```go
result, err := client.Upload(ctx, "my-bucket", "file.txt", reader)
if err != nil {
    var s3Err *errors.Error
    if errors.As(err, &s3Err) {
        fmt.Printf("Operation: %s\n", s3Err.Op)
        fmt.Printf("Bucket: %s\n", s3Err.Bucket)
        fmt.Printf("Key: %s\n", s3Err.Key)
        fmt.Printf("Error: %v\n", s3Err.Err)
    }

    // Check for specific error types
    if errors.Is(err, errors.ErrObjectNotFound) {
        fmt.Println("Object does not exist")
    } else if errors.Is(err, errors.ErrAccessDenied) {
        fmt.Println("Access denied - check permissions")
    } else if errors.Is(err, errors.ErrBucketNotFound) {
        fmt.Println("Bucket does not exist")
    }
}
```

## Performance Considerations

### Multipart Upload
- Automatically triggered for files >100MB
- Configurable part size (default 8MB, minimum 5MB)
- Concurrent part uploads with configurable parallelism

```go
result, err := client.UploadFile(ctx, "my-bucket", "large.zip", "/path/to/large.zip",
    s3.WithUploadPartSize(16 * 1024 * 1024), // 16MB parts
    s3.WithUploadConcurrency(10),             // 10 concurrent uploads
)
```

### Memory Management
- Stream-based operations for large files
- Buffer pooling to reduce allocations
- Configurable buffer sizes

### Concurrency Control
```go
client, err := s3.New(
    s3.WithConcurrency(20), // Limit concurrent operations
)
```

## Testing

### Using Mock Interfaces

The module is designed with testing in mind:

```go
// Your code accepts an interface
type S3Operations interface {
    Upload(ctx context.Context, bucket, key string, reader io.Reader, opts ...s3types.UploadOption) (*s3types.UploadResult, error)
    Download(ctx context.Context, bucket, key string, writer io.Writer, opts ...s3types.DownloadOption) (*s3types.DownloadResult, error)
}

func ProcessData(s3 S3Operations, data io.Reader) error {
    // Your logic here
}

// In tests, use a mock implementation
type mockS3 struct{}

func (m *mockS3) Upload(ctx context.Context, bucket, key string, reader io.Reader, opts ...s3types.UploadOption) (*s3types.UploadResult, error) {
    // Mock implementation
    return &s3types.UploadResult{}, nil
}
```

### Using LocalStack

For integration testing with LocalStack:

```go
client, err := s3.New(
    s3.WithEndpoint("http://localhost:4566"),
    s3.WithForcePathStyle(true),
    s3.WithDisableSSL(true),
    s3.WithRegion("us-east-1"),
)
```

## Best Practices

1. **Always use context**: Pass context for cancellation and timeout control
2. **Handle errors appropriately**: Check for specific error types when needed
3. **Use streaming for large files**: Prefer Download/Upload over Get/Put for large objects
4. **Configure appropriate timeouts**: Set timeouts based on expected file sizes
5. **Use progress tracking**: Implement progress tracking for better UX
6. **Leverage concurrency**: Configure concurrency limits based on your needs
7. **Clean up resources**: Always close files and readers when done
8. **Use sync for batch operations**: Use Sync methods for directory operations
9. **Validate input**: The module validates bucket names and object keys automatically
10. **Use appropriate storage classes**: Choose the right storage class for your use case

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](../../CONTRIBUTING.md) for details on how to contribute to this project.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](../../LICENSE) file for details.

## Support

For issues, questions, or contributions, please visit our [GitHub repository](https://github.com/input-output-hk/catalyst-forge-libs).