# S3 Architecture Document
## High-Level Go Module for AWS S3 Operations

### Version 1.0
### Date: December 2024

---

## 1. Executive Summary

S3 is a high-level Go module that wraps AWS SDK v2 for S3, providing an intuitive and efficient interface for common S3 operations. The module emphasizes developer experience through simple APIs while maintaining flexibility for advanced use cases.

### Core Design Principles
- **Simplicity First**: Common operations should require minimal code
- **Progressive Enhancement**: Advanced features available through options, not required upfront
- **Performance by Default**: Intelligent defaults for concurrency, buffering, and retries
- **Fail Fast**: Validate inputs early, provide clear error messages
- **Zero Configuration**: Module should work with AWS default credentials

---

## 2. Module Structure

```
github.com/yourorg/S3/
├── S3.go                 # Main client and core operations
├── client.go                 # Client initialization and configuration
├── options.go                # Functional options for all operations
├── errors.go                 # Error types and handling
├── types.go                  # Public types and interfaces
├── sync.go                   # Public sync API
├── internal/
│   ├── operations/
│   │   ├── upload.go         # Upload implementation
│   │   ├── download.go       # Download implementation
│   │   ├── copy.go           # Copy/Move operations
│   │   ├── delete.go         # Delete operations
│   │   └── list.go           # List operations
│   ├── transfer/
│   │   ├── manager.go        # Transfer orchestration
│   │   ├── multipart.go      # Multipart handling
│   │   └── progress.go       # Progress tracking
│   ├── sync/
│   │   ├── sync.go           # Sync orchestration
│   │   ├── scanner.go        # File system & S3 scanning
│   │   ├── comparator.go     # Change detection strategies
│   │   ├── planner.go        # Operation planning
│   │   └── executor.go       # Parallel execution
│   ├── validation/
│   │   └── validate.go       # Input validation
│   └── pool/
│       └── buffer.go         # Buffer pooling
├── examples/
├── go.mod
└── go.sum
```

### Package Responsibilities

- **Public packages**: Define API contracts, types, and user-facing functionality
- **internal/operations**: Core S3 operations isolated from public API
- **internal/transfer**: Manages complex transfers (multipart, concurrent)
- **internal/sync**: Complete sync implementation
- **internal/validation**: Centralized validation logic
- **internal/pool**: Memory management optimizations

---

## 3. Public API Specification

### 3.1 Client Initialization

```go
type Client struct {
    // Internal fields not exposed
}

// Constructor with sensible defaults
func New(ctx context.Context, opts ...Option) (*Client, error)

// Options for client configuration
func WithRegion(region string) Option
func WithEndpoint(endpoint string) Option
func WithCredentials(creds aws.CredentialsProvider) Option
func WithMaxRetries(n int) Option
func WithTimeout(timeout time.Duration) Option
```

**Design Decisions:**
- Client uses AWS SDK v2 under the hood
- Respects AWS credential chain by default
- All configuration is optional
- Client is safe for concurrent use

### 3.2 Core Operations API

#### Upload Operations
```go
// Stream-based upload (auto-detects multipart threshold)
func (c *Client) Upload(ctx, bucket, key string, reader io.Reader, opts ...UploadOption) (*UploadResult, error)

// File-based upload (optimized for local files)
func (c *Client) UploadFile(ctx, bucket, key, filepath string, opts ...UploadOption) (*UploadResult, error)

// Byte slice upload (convenience for small data)
func (c *Client) Put(ctx, bucket, key string, data []byte, opts ...UploadOption) error
```

#### Download Operations
```go
// Stream-based download
func (c *Client) Download(ctx, bucket, key string, writer io.Writer, opts ...DownloadOption) (*DownloadResult, error)

// File-based download
func (c *Client) DownloadFile(ctx, bucket, key, filepath string, opts ...DownloadOption) (*DownloadResult, error)

// Byte slice download (convenience for small objects)
func (c *Client) Get(ctx, bucket, key string) ([]byte, error)
```

#### Management Operations
```go
// Object operations
func (c *Client) Delete(ctx, bucket, key string) error
func (c *Client) DeleteMany(ctx, bucket string, keys []string) (*DeleteResult, error)
func (c *Client) Exists(ctx, bucket, key string) (bool, error)
func (c *Client) Copy(ctx, srcBucket, srcKey, dstBucket, dstKey string, opts ...CopyOption) error
func (c *Client) Move(ctx, srcBucket, srcKey, dstBucket, dstKey string, opts ...CopyOption) error
func (c *Client) GetMetadata(ctx, bucket, key string) (*ObjectMetadata, error)

// Listing operations
func (c *Client) List(ctx, bucket, prefix string, opts ...ListOption) (*ListResult, error)
func (c *Client) ListAll(ctx, bucket, prefix string) <-chan *Object

// Bucket operations
func (c *Client) CreateBucket(ctx, bucket string, opts ...BucketOption) error
func (c *Client) DeleteBucket(ctx, bucket string) error
```

### 3.3 Functional Options Pattern

All operations use functional options for configuration:

```go
type UploadOption func(*uploadConfig)

// Examples of upload options
func WithContentType(contentType string) UploadOption
func WithMetadata(metadata map[string]string) UploadOption
func WithStorageClass(class StorageClass) UploadOption
func WithServerSideEncryption(sse SSEConfig) UploadOption
func WithProgress(tracker ProgressTracker) UploadOption
func WithPartSize(size int64) UploadOption
func WithConcurrency(n int) UploadOption
```

**Design Rationale:**
- Options pattern allows zero-config usage
- New options can be added without breaking changes
- Options are composable and order-independent

---

## 4. Sync Functionality

### 4.1 Sync API

```go
func (c *Client) Sync(ctx, localPath, bucket, prefix string, opts ...SyncOption) (*SyncResult, error)

type SyncResult struct {
    FilesUploaded   int
    FilesSkipped    int
    FilesDeleted    int
    BytesUploaded   int64
    Errors          []SyncError
    Duration        time.Duration
}

type SyncOption func(*syncConfig)

func WithDryRun(dryRun bool) SyncOption
func WithExclude(patterns ...string) SyncOption
func WithInclude(patterns ...string) SyncOption
func WithSyncProgress(tracker SyncProgressTracker) SyncOption
func WithParallelism(n int) SyncOption
func WithComparator(comp FileComparator) SyncOption
```

### 4.2 Sync Algorithm

#### Phase 1: Inventory Building
1. **Local Scan**: Walk filesystem, respecting include/exclude patterns
2. **Remote Scan**: List all S3 objects with given prefix
3. **Normalization**: Ensure path separators are consistent (use forward slashes)

#### Phase 2: Change Detection
```
For each local file:
    if not exists in remote:
        mark for UPLOAD (new file)
    else if changed:
        mark for UPLOAD (modified)
    else:
        mark for SKIP (unchanged)

For each remote file:
    if not exists in local:
        mark for DELETE (extra file)
```

#### Phase 3: Execution
1. Group operations by type
2. Execute uploads in parallel (respecting concurrency limit)
3. Batch delete operations (up to 1000 per request)
4. Track progress and errors

### 4.3 Change Detection Strategies

The module provides pluggable comparators:

```go
type FileComparator interface {
    HasChanged(local *LocalFile, remote *RemoteFile) bool
}
```

**Built-in Comparators:**
1. **SmartComparator** (default):
   - Compare size first (different size = changed)
   - Compare MD5/ETag if available and reliable
   - Fallback to modification time for multipart uploads

2. **SizeOnlyComparator**:
   - Only compares file sizes
   - Fast but may miss changes that don't affect size

3. **ChecksumComparator**:
   - Always computes and compares checksums
   - Most accurate but requires more computation

### 4.4 Sync Optimization Strategies

- **Parallel Uploads**: Default concurrency of 5, configurable
- **Batch Deletes**: Group up to 1000 deletes per API call
- **Smart Scanning**: Stop scanning if context cancelled
- **Memory Efficiency**: Stream large files, don't load into memory
- **Progress Tracking**: Optional real-time progress updates

---

## 5. Error Handling

### 5.1 Error Types

```go
type Error struct {
    Op     string // Operation that failed
    Bucket string
    Key    string
    Err    error  // Underlying AWS SDK error
}

// Sentinel errors for common conditions
var (
    ErrObjectNotFound = errors.New("S3: object not found")
    ErrBucketNotFound = errors.New("S3: bucket not found")
    ErrAccessDenied   = errors.New("S3: access denied")
    ErrInvalidInput   = errors.New("S3: invalid input")
)
```

### 5.2 Error Handling Strategy

1. **Wrap AWS SDK errors** with context about operation
2. **Validate inputs early** to avoid unnecessary API calls
3. **Provide actionable error messages** with operation context
4. **Use sentinel errors** for common conditions that callers might handle
5. **Aggregate errors** in batch operations (sync, deleteMany)

---

## 6. Performance Considerations

### 6.1 Multipart Upload Thresholds

- Files > 100MB: Automatically use multipart upload
- Files > 5GB: Multipart upload required by S3
- Default part size: 5MB (configurable)
- Concurrent parts: 5 (configurable)

### 6.2 Memory Management

- Use `sync.Pool` for buffer reuse in transfers
- Stream large files instead of loading into memory
- Use io.Copy with buffering for optimal performance
- Release resources promptly with proper defer statements

### 6.3 Concurrency Controls

```go
// Default concurrency limits
const (
    DefaultUploadConcurrency = 5
    DefaultSyncParallelism   = 5
    DefaultPartSize          = 5 * 1024 * 1024 // 5MB
)
```

### 6.4 API Call Optimization

- List operations use pagination (1000 items default)
- Batch delete operations (up to 1000 objects)
- Head requests for existence checks (not GET)
- Reuse AWS SDK client across operations

---

## 7. Configuration Strategy

### 7.1 Configuration Hierarchy

1. **Defaults**: Sensible defaults that work for most cases
2. **Client-level**: Configuration passed to New()
3. **Operation-level**: Options passed to individual operations

Operation-level options override client-level configuration.

### 7.2 AWS SDK Configuration

- Leverage AWS SDK v2's configuration chain
- Support standard AWS environment variables
- Respect AWS config files (~/.aws/config)
- Allow custom credential providers

---

## 8. Testing Strategy

### 8.1 Unit Tests

- Mock S3 client interface for operations
- Test error conditions and edge cases
- Validate option application
- Test sync planning logic separately from execution
- Use table-driven tests for comprehensive coverage

### 8.2 Integration Tests with Testcontainers & LocalStack

#### Setup Example
```go
// tests/integration_test.go
package tests

import (
    "context"
    "testing"

    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/localstack"
)

func setupLocalStack(t *testing.T) (*localstack.LocalStackContainer, string) {
    ctx := context.Background()

    container, err := localstack.RunContainer(ctx,
        testcontainers.WithImage("localstack/localstack:latest"),
        localstack.WithServices("s3"),
    )
    if err != nil {
        t.Fatal(err)
    }

    endpoint, err := container.Endpoint(ctx, "s3")
    if err != nil {
        t.Fatal(err)
    }

    return container, endpoint
}

func TestIntegrationSuite(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    container, endpoint := setupLocalStack(t)
    defer container.Terminate(context.Background())

    // Run tests against LocalStack S3
    client := S3.New(context.Background(),
        S3.WithEndpoint(endpoint),
        S3.WithRegion("us-east-1"),
        S3.ForcePathStyle(true), // Required for LocalStack
    )

    // Run test suite
    t.Run("Upload", func(t *testing.T) { testUpload(t, client) })
    t.Run("Download", func(t *testing.T) { testDownload(t, client) })
    t.Run("Sync", func(t *testing.T) { testSync(t, client) })
}
```

#### Benefits of Testcontainers + LocalStack
- **Isolation**: Each test run gets a fresh environment
- **Reproducibility**: Same environment across CI/CD and local development
- **Real S3 API**: Tests actual S3 behavior, not mocks
- **Fast Startup**: LocalStack starts in seconds
- **No External Dependencies**: No need for AWS credentials or internet access
- **Parallel Testing**: Can run multiple containers for parallel test suites

#### Alternative: MinIO for Simpler Setup
```go
// Alternative using MinIO (lighter weight than LocalStack)
func setupMinIO(t *testing.T) (testcontainers.Container, string) {
    ctx := context.Background()

    req := testcontainers.ContainerRequest{
        Image: "minio/minio:latest",
        Env: map[string]string{
            "MINIO_ROOT_USER":     "minioadmin",
            "MINIO_ROOT_PASSWORD": "minioadmin",
        },
        Cmd: []string{"server", "/data"},
        ExposedPorts: []string{"9000/tcp"},
        WaitingFor:   wait.ForHTTP("/minio/health/ready").WithPort("9000"),
    }

    container, err := testcontainers.GenericContainer(ctx,
        testcontainers.GenericContainerRequest{
            ContainerRequest: req,
            Started:          true,
        })

    endpoint, _ := container.Endpoint(ctx, "9000")
    return container, fmt.Sprintf("http://%s", endpoint)
}
```

### 8.3 Test Categories

#### Unit Tests (no external dependencies)
- Input validation logic
- Option application
- Error wrapping
- Path normalization
- Sync planning algorithms
- Comparator logic

#### Integration Tests (with LocalStack/MinIO)
- Full upload/download cycle
- Multipart upload behavior
- Sync operations with real files
- Error conditions (permissions, non-existent buckets)
- Concurrent operations
- Large file handling

#### End-to-End Tests (optional, with real S3)
- Performance benchmarks
- Cross-region operations
- Real AWS features (versioning, encryption)
- IAM permission validation

### 8.4 Testable Interfaces

```go
// Internal interfaces for mocking
type S3Interface interface {
    PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
    GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
    DeleteObjects(context.Context, *s3.DeleteObjectsInput, ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
    ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
    HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
    CopyObject(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
}
```

### 8.5 Test Helpers

```go
// tests/helpers.go
package tests

// Helper to create test bucket with cleanup
func CreateTestBucket(t *testing.T, client *S3.Client) string {
    bucket := fmt.Sprintf("test-bucket-%d", time.Now().Unix())
    err := client.CreateBucket(context.Background(), bucket)
    require.NoError(t, err)

    t.Cleanup(func() {
        // Clean up all objects first
        objects := client.ListAll(context.Background(), bucket, "")
        var keys []string
        for obj := range objects {
            keys = append(keys, obj.Key)
        }
        if len(keys) > 0 {
            client.DeleteMany(context.Background(), bucket, keys)
        }

        // Then delete bucket
        client.DeleteBucket(context.Background(), bucket)
    })

    return bucket
}

// Helper to create test files
func CreateTestFile(t *testing.T, content string) string {
    file, err := os.CreateTemp("", "test-*.txt")
    require.NoError(t, err)

    _, err = file.WriteString(content)
    require.NoError(t, err)
    file.Close()

    t.Cleanup(func() {
        os.Remove(file.Name())
    })

    return file.Name()
}
```

---

## 9. Security Considerations

### 9.1 Credential Management
- Never log or expose credentials
- Support IAM roles for EC2/ECS/Lambda
- Support temporary credentials via STS
- Clear sensitive data from memory after use

### 9.2 Input Validation
- Validate bucket names (DNS compliance)
- Validate object keys (no invalid characters)
- Prevent path traversal in file operations
- Sanitize metadata values

### 9.3 Secure Defaults
- Use HTTPS for all S3 operations
- Support SSE-S3, SSE-KMS, SSE-C encryption
- Default to private ACLs
- Validate content types to prevent injection

---

## 10. Future Enhancements

### 10.1 Potential Features (Not in V1)
- Presigned URL generation
- S3 event notification handling
- Versioning support
- Lifecycle policy management
- Cross-region replication setup
- S3 Select integration
- Bandwidth throttling
- Resume incomplete transfers

### 10.2 Extension Points
- Plugin system for custom comparators
- Hooks for pre/post operation actions
- Custom retry strategies
- Metrics collection interface

---

## 11. Implementation Guidelines

### 11.1 Code Organization
- Keep public API surface small
- Hide implementation details in internal packages
- Use interfaces for testability
- Document all public types and methods

### 11.2 Dependencies

#### Production Dependencies
- AWS SDK v2 for S3 operations (`github.com/aws/aws-sdk-go-v2`)
- Standard library for most functionality
- Minimal external dependencies

#### Development/Test Dependencies
- Testcontainers-Go (`github.com/testcontainers/testcontainers-go`)
- LocalStack module (`github.com/testcontainers/testcontainers-go/modules/localstack`)
- Testify for assertions (`github.com/stretchr/testify`) - optional but recommended
- MinIO client (alternative to LocalStack)

#### Dependency Management
- Use Go modules for dependency management
- Vendor dependencies for reproducible builds (optional)
- Keep test dependencies separate with build tags if needed

### 11.3 Error Handling
- Never panic in library code
- Always return errors to caller
- Wrap errors with context
- Clean up resources on error

### 11.4 Logging
- No logging by default
- Optional debug mode via configuration
- Allow caller to provide logger interface
- Log operations, not data

---

## 12. Example Usage Patterns

### Basic Upload
```go
client, _ := S3.New(ctx)
result, err := client.UploadFile(ctx, "my-bucket", "photos/vacation.jpg",
    "/path/to/file.jpg")
```

### Sync with Progress
```go
client, _ := S3.New(ctx)
result, err := client.Sync(ctx, "./dist", "website-bucket", "assets/",
    S3.WithExclude("*.tmp", "node_modules/**"),
    S3.WithParallelism(10),
    S3.WithSyncProgress(progressBar))
```

### Advanced Configuration
```go
client, _ := S3.New(ctx,
    S3.WithRegion("us-west-2"),
    S3.WithMaxRetries(5),
    S3.WithTimeout(30*time.Second))
```

---

## Appendix A: Type Definitions

```go
// Core types that must be defined
type StorageClass string
type SSEType string
type ObjectACL string

type Object struct {
    Key          string
    Size         int64
    LastModified time.Time
    ETag         string
    StorageClass string
}

type ObjectMetadata struct {
    ContentType   string
    ContentLength int64
    LastModified  time.Time
    ETag          string
    Metadata      map[string]string
}

type ProgressTracker interface {
    Update(bytesTransferred, totalBytes int64)
    Complete()
    Error(err error)
}

type FileComparator interface {
    HasChanged(local *LocalFile, remote *RemoteFile) bool
}
```

---

## Document Version History

- v1.0 - Initial architecture document
- Last Updated: December 2024
- Status: Ready for Implementation