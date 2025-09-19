// Package s3 provides public sync API for directory synchronization.
package s3

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/comparator"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/executor"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/planner"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/scanner"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/sync"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Sync synchronizes a local directory with an S3 bucket prefix.
// It supports bidirectional sync with configurable options for
// include/exclude patterns, deletion of extra files, and dry-run mode.
//
// The sync operation follows a three-phase approach:
// 1. Inventory: Scan both local and remote locations
// 2. Planning: Determine what operations are needed
// 3. Execution: Perform the operations with concurrency control
//
// By default, sync only uploads new or modified files. Use WithSyncDeleteExtra
// to remove files from S3 that don't exist locally.
//
// Returns:
//   - *SyncResult: Contains statistics about the sync operation
//   - error: Returns an error if the sync fails
//
// Errors:
//   - ErrInvalidInput: If localPath or bucket is empty
//   - ErrAccessDenied: If credentials lack required permissions
//   - ErrBucketNotFound: If the specified bucket doesn't exist
//   - File system errors for local path access
//   - Network errors or AWS SDK errors wrapped in Error type
//
// Example:
//
//	result, err := client.Sync(ctx, "/local/path", "my-bucket", "prefix/",
//	    s3.WithSyncDryRun(true),
//	    s3.WithSyncIncludePattern("*.txt"),
//	    s3.WithSyncExcludePattern("*.tmp"),
//	    s3.WithSyncProgressTracker(tracker),
//	)
//	if err != nil {
//	    return fmt.Errorf("sync failed: %w", err)
//	}
//	fmt.Printf("Uploaded %d files (%d bytes)\n", result.FilesUploaded, result.BytesUploaded)
//	fmt.Printf("Skipped %d unchanged files\n", result.FilesSkipped)
func (c *Client) Sync(
	ctx context.Context,
	localPath, bucket, prefix string,
	opts ...s3types.SyncOption,
) (*s3types.SyncResult, error) {
	// Apply functional options
	cfg := &s3types.SyncOptionConfig{
		DryRun:          false,
		ExcludePatterns: []string{},
		IncludePatterns: []string{},
		Parallelism:     5, // Default parallelism
		DeleteExtra:     false,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Validate inputs
	if localPath == "" {
		return nil, errors.NewValidationError("localPath cannot be empty")
	}
	if bucket == "" {
		return nil, errors.NewValidationError("bucket cannot be empty")
	}

	// Ensure prefix ends with "/" if not empty
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// Resolve local path
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return nil, errors.NewError("sync operation", fmt.Errorf("failed to resolve local path: %w", err))
	}

	// Create internal components
	sc := scanner.NewScanner(c.s3Client, c.fs)

	// Use provided comparator or default to SmartComparator
	var comp comparator.Comparator
	if cfg.Comparator != nil {
		comp = cfg.Comparator
	} else {
		comp = comparator.NewSmartComparator()
	}

	pl := planner.NewPlanner(comp)

	// Configure executor with parallelism - use client-level concurrency as default
	parallelism := cfg.Parallelism
	if parallelism <= 0 {
		// Use client-level concurrency setting as default, fallback to 5
		clientCfg := c.getClientConfig()
		parallelism = clientCfg.Concurrency
		if parallelism <= 0 {
			parallelism = 5
		}
	}
	ex := executor.NewExecutor(c.s3Client, parallelism)

	// Set progress tracker if provided
	if cfg.ProgressTracker != nil {
		ex = ex.WithProgressTracker(cfg.ProgressTracker)
	}

	// Create sync manager
	manager := sync.NewManager(*sc, comp, *pl, *ex)

	// Configure sync
	syncConfig := &sync.Config{
		LocalPath:       absPath,
		Bucket:          bucket,
		Prefix:          prefix,
		IncludePatterns: cfg.IncludePatterns,
		ExcludePatterns: cfg.ExcludePatterns,
		DeleteExtra:     cfg.DeleteExtra,
		DryRun:          cfg.DryRun,
		ProgressTracker: cfg.ProgressTracker,
		Parallelism:     parallelism,
	}

	// Execute sync
	result, err := manager.Sync(ctx, syncConfig)
	if err != nil {
		return nil, errors.NewError("sync operation", err)
	}

	// Convert internal result to public result
	return &s3types.SyncResult{
		FilesUploaded: result.FilesUploaded,
		FilesSkipped:  result.FilesSkipped,
		FilesDeleted:  result.FilesDeleted,
		BytesUploaded: result.BytesUploaded,
		Errors:        result.Errors,
		Duration:      result.Duration,
	}, nil
}

// SyncDownload synchronizes from S3 to local filesystem (download only).
// This is a convenience method that downloads new and updated files from S3
// without uploading local changes.
//
// NOTE: This method is not yet implemented and will return an error.
// Use the AWS CLI or implement custom download logic for now.
//
// Returns:
//   - *SyncResult: Would contain download statistics when implemented
//   - error: Currently always returns ErrNotImplemented
//
// Example (when implemented):
//
//	result, err := client.SyncDownload(ctx, "my-bucket", "prefix/", "/local/path",
//	    s3.WithSyncProgressTracker(tracker),
//	)
//	if err != nil {
//	    return fmt.Errorf("download sync failed: %w", err)
//	}
func (c *Client) SyncDownload(
	ctx context.Context,
	bucket, prefix, localPath string,
	opts ...s3types.SyncOption,
) (*s3types.SyncResult, error) {
	// TODO: Implement download-only sync in a future phase
	// This would require reversing the sync direction in the sync manager
	return nil, errors.NewError("sync download", fmt.Errorf("download-only sync not yet implemented"))
}

// SyncUpload synchronizes from local filesystem to S3 (upload only).
// This is equivalent to Sync without the DeleteExtra option.
//
// This method only uploads new or modified files to S3.
// It never deletes files from S3, even if they don't exist locally.
//
// Returns:
//   - *SyncResult: Contains statistics about uploaded files
//   - error: Returns an error if the sync fails
//
// Errors:
//   - Same as Sync method
//
// Example:
//
//	result, err := client.SyncUpload(ctx, "/local/path", "my-bucket", "prefix/",
//	    s3.WithSyncProgressTracker(tracker),
//	    s3.WithSyncIncludePattern("*.jpg"),
//	)
//	if err != nil {
//	    return fmt.Errorf("upload sync failed: %w", err)
//	}
//	fmt.Printf("Uploaded %d files\n", result.FilesUploaded)
func (c *Client) SyncUpload(
	ctx context.Context,
	localPath, bucket, prefix string,
	opts ...s3types.SyncOption,
) (*s3types.SyncResult, error) {
	// Ensure DeleteExtra is false for upload-only sync
	modifiedOpts := make([]s3types.SyncOption, len(opts))
	copy(modifiedOpts, opts)
	modifiedOpts = append(modifiedOpts, WithSyncDeleteExtra(false))

	return c.Sync(ctx, localPath, bucket, prefix, modifiedOpts...)
}
