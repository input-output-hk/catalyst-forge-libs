// Package sync provides shared types for the sync functionality.
package sync

import (
	"time"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Config holds configuration for a sync operation.
type Config struct {
	// LocalPath is the local directory to sync from
	LocalPath string

	// Bucket is the S3 bucket to sync to
	Bucket string

	// Prefix is the S3 key prefix to sync to
	Prefix string

	// IncludePatterns are glob patterns for files to include
	IncludePatterns []string

	// ExcludePatterns are glob patterns for files to exclude
	ExcludePatterns []string

	// DeleteExtra determines if extra files in S3 should be deleted
	DeleteExtra bool

	// DryRun determines if this should be a dry run (no actual changes)
	DryRun bool

	// ProgressTracker tracks sync progress
	ProgressTracker s3types.ProgressTracker

	// Parallelism controls the number of concurrent operations
	Parallelism int
}

// Result contains the results of a sync operation.
type Result struct {
	// FilesUploaded is the number of files uploaded
	FilesUploaded int

	// FilesSkipped is the number of files skipped (unchanged)
	FilesSkipped int

	// FilesDeleted is the number of files deleted
	FilesDeleted int

	// BytesUploaded is the total bytes uploaded
	BytesUploaded int64

	// Errors contains any errors that occurred during sync
	Errors []s3types.SyncError

	// Duration is how long the sync operation took
	Duration time.Duration

	// Operations contains details about planned operations (for dry run)
	Operations []Operation
}

// Operation represents a sync operation to be performed.
// Note: This is a simplified view. The full Operation type with priority
// is defined in the planner package.
type Operation struct {
	// Type is the operation type (upload, delete, skip)
	Type OperationType

	// LocalPath is the local file path (for uploads)
	LocalPath string

	// RemoteKey is the S3 key (for uploads and deletes)
	RemoteKey string

	// Size is the file/object size
	Size int64

	// Reason describes why this operation is needed
	Reason string
}

// OperationType defines the type of sync operation.
type OperationType string

const (
	// OperationUpload indicates a file needs to be uploaded
	OperationUpload OperationType = "upload"

	// OperationDelete indicates a remote file needs to be deleted
	OperationDelete OperationType = "delete"

	// OperationSkip indicates a file is unchanged and should be skipped
	OperationSkip OperationType = "skip"
)
