// Package s3types provides shared type definitions for the S3 module.
package s3types

import (
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/input-output-hk/catalyst-forge-libs/fs"
)

// StorageClass represents the S3 storage class for objects.
type StorageClass string

// Predefined S3 storage classes
const (
	// StorageClassStandard is the default S3 storage class
	StorageClassStandard StorageClass = "STANDARD"

	// StorageClassReducedRedundancy provides reduced redundancy storage
	StorageClassReducedRedundancy StorageClass = "REDUCED_REDUNDANCY"

	// StorageClassStandardIA provides infrequent access storage
	StorageClassStandardIA StorageClass = "STANDARD_IA"

	// StorageClassOneZoneIA provides one zone infrequent access storage
	StorageClassOneZoneIA StorageClass = "ONEZONE_IA"

	// StorageClassIntelligentTiering provides intelligent tiering storage
	StorageClassIntelligentTiering StorageClass = "INTELLIGENT_TIERING"

	// StorageClassGlacier provides Glacier archival storage
	StorageClassGlacier StorageClass = "GLACIER"

	// StorageClassDeepArchive provides Deep Archive storage
	StorageClassDeepArchive StorageClass = "DEEP_ARCHIVE"

	// StorageClassGlacierIR provides Glacier Instant Retrieval storage
	StorageClassGlacierIR StorageClass = "GLACIER_IR"
)

// SSEType represents the server-side encryption type for objects.
type SSEType string

// Predefined server-side encryption types
const (
	// SSES3 uses S3-managed encryption keys
	SSES3 SSEType = "AES256"

	// SSEKMS uses AWS KMS-managed encryption keys
	SSEKMS SSEType = "aws:kms"

	// SSEC uses customer-provided encryption keys
	SSEC SSEType = "AES256"
)

// ObjectACL represents the access control list for S3 objects.
type ObjectACL string

// Predefined object ACLs
const (
	// ACLPrivate grants private access (default)
	ACLPrivate ObjectACL = "private"

	// ACLPublicRead grants public read access
	ACLPublicRead ObjectACL = "public-read"

	// ACLPublicReadWrite grants public read and write access
	ACLPublicReadWrite ObjectACL = "public-read-write"

	// ACLAuthenticatedRead grants authenticated users read access
	ACLAuthenticatedRead ObjectACL = "authenticated-read"

	// ACLBucketOwnerRead grants bucket owner read access
	ACLOwnerRead ObjectACL = "bucket-owner-read"

	// ACLBucketOwnerFullControl grants bucket owner full control
	ACLOwnerFullControl ObjectACL = "bucket-owner-full-control"
)

// Object represents an S3 object with its basic metadata.
type Object struct {
	// Key is the S3 object key (path)
	Key string

	// Size is the object size in bytes
	Size int64

	// LastModified is when the object was last modified
	LastModified time.Time

	// ETag is the S3 entity tag for the object
	ETag string

	// StorageClass is the S3 storage class
	StorageClass string
}

// ObjectMetadata contains detailed metadata about an S3 object.
type ObjectMetadata struct {
	// ContentType is the MIME type of the object
	ContentType string

	// ContentLength is the size of the object in bytes
	ContentLength int64

	// LastModified is when the object was last modified
	LastModified time.Time

	// ETag is the S3 entity tag for the object
	ETag string

	// Metadata contains user-defined metadata
	Metadata map[string]string
}

// ProgressTracker defines the interface for tracking transfer progress.
// Implementations can provide real-time progress updates during uploads and downloads.
type ProgressTracker interface {
	// Update is called periodically with transfer progress
	Update(bytesTransferred, totalBytes int64)

	// Complete is called when the transfer completes successfully
	Complete()

	// Error is called when the transfer fails
	Error(err error)
}

// FileComparator defines the interface for comparing local and remote files.
// Different implementations can use various comparison strategies.
type FileComparator interface {
	// HasChanged determines if the local and remote files are different
	HasChanged(local *LocalFile, remote *RemoteFile) bool
}

// LocalFile represents a file on the local filesystem during sync operations.
type LocalFile struct {
	// Path is the local file path
	Path string

	// Size is the file size in bytes
	Size int64

	// ModTime is the file modification time
	ModTime time.Time

	// Checksum is an optional checksum for the file
	Checksum string
}

// RemoteFile represents an S3 object during sync operations.
type RemoteFile struct {
	// Key is the S3 object key
	Key string

	// Size is the object size in bytes
	Size int64

	// LastModified is when the object was last modified
	LastModified time.Time

	// ETag is the S3 entity tag
	ETag string
}

// SSEConfig contains server-side encryption configuration.
type SSEConfig struct {
	// Type is the encryption type (S3, KMS, or customer-provided)
	Type SSEType

	// KMSKeyID is the KMS key ID (required for SSE-KMS)
	KMSKeyID string

	// CustomerKey is the customer-provided encryption key (for SSE-C)
	CustomerKey string

	// CustomerKeyMD5 is the MD5 hash of the customer key (for SSE-C)
	CustomerKeyMD5 string
}

// UploadConfig holds configuration for upload operations.
type UploadConfig struct {
	ContentType     string
	Metadata        map[string]string
	StorageClass    StorageClass
	SSE             *SSEConfig
	ACL             ObjectACL
	ProgressTracker ProgressTracker
	PartSize        int64
	Concurrency     int
}

// UploadResult contains the result of an upload operation.
type UploadResult struct {
	// Key is the S3 object key that was uploaded
	Key string

	// Size is the size of the uploaded object in bytes
	Size int64

	// ETag is the S3 entity tag for the uploaded object
	ETag string

	// VersionID is the version ID if versioning is enabled
	VersionID string

	// Duration is how long the upload took
	Duration time.Duration
}

// DownloadResult contains the result of a download operation.
type DownloadResult struct {
	// Key is the S3 object key that was downloaded
	Key string

	// Size is the size of the downloaded object in bytes
	Size int64

	// ETag is the S3 entity tag for the downloaded object
	ETag string

	// VersionID is the version ID if versioning is enabled
	VersionID string

	// Duration is how long the download took
	Duration time.Duration
}

// DeleteResult contains the result of a delete operation.
type DeleteResult struct {
	// Deleted contains successfully deleted objects
	Deleted []Object

	// Errors contains any errors that occurred during deletion
	Errors []DeleteError

	// Duration is how long the operation took
	Duration time.Duration
}

// DeleteError represents an error that occurred during a delete operation.
type DeleteError struct {
	// Key is the S3 object key that failed to delete
	Key string

	// Version is the version ID if specified
	Version string

	// Code is the error code
	Code string

	// Message is the error message
	Message string
}

// ListResult contains the result of a list operation.
type ListResult struct {
	// Objects contains the listed objects
	Objects []Object

	// IsTruncated indicates if the results were truncated
	IsTruncated bool

	// NextToken is the token for the next page of results
	NextToken string

	// NextContinuationToken is the token for the next page of results (ListObjectsV2)
	NextContinuationToken string

	// Duration is how long the operation took
	Duration time.Duration
}

// SyncResult contains the result of a sync operation.
type SyncResult struct {
	// FilesUploaded is the number of files uploaded
	FilesUploaded int

	// FilesSkipped is the number of files skipped (unchanged)
	FilesSkipped int

	// FilesDeleted is the number of files deleted
	FilesDeleted int

	// BytesUploaded is the total bytes uploaded
	BytesUploaded int64

	// Errors contains any errors that occurred during sync
	Errors []SyncError

	// Duration is how long the sync operation took
	Duration time.Duration
}

// SyncError represents an error that occurred during a sync operation.
type SyncError struct {
	// Path is the file path that caused the error
	Path string

	// Code is the error code
	Code string

	// Message is the error message
	Message string
}

// Configuration types for functional options

// ClientConfig holds configuration for the S3 client.
type ClientConfig struct {
	Region           string
	Endpoint         string
	MaxRetries       int
	Timeout          time.Duration
	Concurrency      int
	PartSize         int64
	ForcePathStyle   bool
	CustomAWSConfig  *aws.Config
	DisableSSL       bool
	RetryMode        string
	CustomHTTPClient *http.Client
	DefaultBucket    string
	Filesystem       fs.Filesystem // Filesystem abstraction for file operations
}

// UploadOptionConfig holds configuration for upload operations via functional options.
type UploadOptionConfig struct {
	ContentType     string
	Metadata        map[string]string
	StorageClass    StorageClass
	SSE             *SSEConfig
	ACL             ObjectACL
	ProgressTracker ProgressTracker
	PartSize        int64
	Concurrency     int
}

// DownloadOptionConfig holds configuration for download operations via functional options.
type DownloadOptionConfig struct {
	ProgressTracker ProgressTracker
	RangeSpec       string // renamed from "range" to avoid Go keyword conflict
}

// CopyOptionConfig holds configuration for copy operations via functional options.
type CopyOptionConfig struct {
	Metadata        map[string]string
	StorageClass    StorageClass
	SSE             *SSEConfig
	ACL             ObjectACL
	ReplaceMetadata bool
}

// ListOptionConfig holds configuration for list operations via functional options.
type ListOptionConfig struct {
	Prefix     string
	Delimiter  string
	MaxKeys    int32
	StartAfter string
}

// BucketOptionConfig holds configuration for bucket operations via functional options.
type BucketOptionConfig struct {
	Region string
}

// SyncOptionConfig holds configuration for sync operations via functional options.
type SyncOptionConfig struct {
	DryRun          bool
	ExcludePatterns []string
	IncludePatterns []string
	ProgressTracker ProgressTracker
	Parallelism     int
	Comparator      FileComparator
	DeleteExtra     bool
}

// Option is a functional option for configuring the S3 client.
type (
	Option func(*ClientConfig)
	// UploadOption is a functional option for configuring S3 upload operations.
	UploadOption func(*UploadOptionConfig)
	// DownloadOption is a functional option for configuring S3 download operations.
	DownloadOption func(*DownloadOptionConfig)
	// CopyOption is a functional option for configuring S3 copy operations.
	CopyOption func(*CopyOptionConfig)
	// ListOption is a functional option for configuring S3 list operations.
	ListOption func(*ListOptionConfig)
	// BucketOption is a functional option for configuring S3 bucket operations.
	BucketOption func(*BucketOptionConfig)
	// SyncOption is a functional option for configuring S3 sync operations.
	SyncOption func(*SyncOptionConfig)
)
