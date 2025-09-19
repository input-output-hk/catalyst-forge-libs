// Package sync provides the main sync orchestration logic.
// This includes coordinating the sync phases and managing the overall sync process.
//
// This package acts as the main entry point for all sync-related operations.
package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/comparator"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/executor"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/planner"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/scanner"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Manager coordinates the three phases of sync operations:
// 1. Inventory Building: Scan local filesystem and remote S3
// 2. Change Detection: Compare files and determine operations
// 3. Execution: Execute operations with concurrency control
type Manager struct {
	scanner    scanner.Scanner
	comparator comparator.Comparator
	planner    planner.Planner
	executor   executor.Executor
}

// NewManager creates a new sync manager with the provided components.
func NewManager(
	sc scanner.Scanner,
	cmp comparator.Comparator,
	pl planner.Planner,
	ex executor.Executor,
) *Manager {
	return &Manager{
		scanner:    sc,
		comparator: cmp,
		planner:    pl,
		executor:   ex,
	}
}

// Sync executes a complete sync operation following the three-phase approach.
func (sm *Manager) Sync(ctx context.Context, config *Config) (*Result, error) {
	startTime := time.Now()

	// Phase 1: Inventory Building
	localFiles, remoteObjects, err := sm.buildInventory(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to build inventory: %w", err)
	}

	// Phase 2: Change Detection and Planning
	operations, err := sm.planOperations(ctx, config, localFiles, remoteObjects)
	if err != nil {
		return nil, fmt.Errorf("failed to plan operations: %w", err)
	}

	// For dry run, return the planned operations without executing
	if config.DryRun {
		syncOperations := convertToSyncOperations(operations)
		return &Result{
			Operations: syncOperations,
			Duration:   time.Since(startTime),
		}, nil
	}

	// Phase 3: Execution
	result, err := sm.executeOperations(ctx, config, operations)
	if err != nil {
		return nil, fmt.Errorf("failed to execute operations: %w", err)
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// convertToSyncOperations converts planner.Operation slice to sync.Operation slice
func convertToSyncOperations(plannerOps []*planner.Operation) []Operation {
	syncOps := make([]Operation, len(plannerOps))
	for i, op := range plannerOps {
		syncOps[i] = Operation{
			Type:      OperationType(op.Type),
			LocalPath: op.LocalPath,
			RemoteKey: op.RemoteKey,
			Size:      op.Size,
			Reason:    op.Reason,
		}
	}
	return syncOps
}

// buildInventory performs Phase 1: Inventory Building.
// It scans both the local filesystem and remote S3 to build complete inventories.
func (sm *Manager) buildInventory(
	ctx context.Context,
	config *Config,
) ([]*s3types.LocalFile, []*s3types.RemoteFile, error) {
	// Scan local filesystem
	localFiles, err := sm.scanner.ScanLocal(ctx, config.LocalPath, config.IncludePatterns, config.ExcludePatterns)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan local directory: %w", err)
	}

	// Scan remote S3
	remoteObjects, err := sm.scanner.ScanRemote(ctx, config.Bucket, config.Prefix)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan remote bucket: %w", err)
	}

	return localFiles, remoteObjects, nil
}

// planOperations performs Phase 2: Change Detection and Planning.
// It compares local and remote inventories to determine what operations are needed.
func (sm *Manager) planOperations(
	ctx context.Context,
	config *Config,
	localFiles []*s3types.LocalFile,
	remoteObjects []*s3types.RemoteFile,
) ([]*planner.Operation, error) {
	// Use the planner package to create the operation plan
	operations, err := sm.planner.Plan(
		config.LocalPath,
		config.Bucket,
		config.Prefix,
		localFiles,
		remoteObjects,
		config.DeleteExtra,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation plan: %w", err)
	}
	return operations, nil
}

// executeOperations performs Phase 3: Execution.
// It executes the planned operations with concurrency control.
func (sm *Manager) executeOperations(
	ctx context.Context,
	config *Config,
	operations []*planner.Operation,
) (*Result, error) {
	result := &Result{}

	// Separate operations by type
	var uploads, deletes []*planner.Operation
	for _, op := range operations {
		switch op.Type {
		case planner.OperationUpload:
			uploads = append(uploads, op)
		case planner.OperationDelete:
			deletes = append(deletes, op)
		case planner.OperationSkip:
			result.FilesSkipped++
		}
	}

	// Execute uploads with concurrency control
	if len(uploads) > 0 {
		uploadResult, err := sm.executor.ExecuteUploads(ctx, &executor.SyncConfig{
			Bucket: config.Bucket,
			Prefix: config.Prefix,
		}, uploads)
		if err != nil {
			result.Errors = append(result.Errors, s3types.SyncError{
				Path:    "uploads",
				Code:    "EXECUTION_ERROR",
				Message: err.Error(),
			})
		} else {
			result.FilesUploaded = uploadResult.FilesUploaded()
			result.BytesUploaded = uploadResult.BytesUploaded
		}
	}

	// Execute deletes
	if len(deletes) > 0 {
		deleteResult, err := sm.executor.ExecuteDeletes(ctx, &executor.SyncConfig{
			Bucket: config.Bucket,
			Prefix: config.Prefix,
		}, deletes)
		if err != nil {
			result.Errors = append(result.Errors, s3types.SyncError{
				Path:    "deletes",
				Code:    "EXECUTION_ERROR",
				Message: err.Error(),
			})
		} else {
			result.FilesDeleted = deleteResult.FilesDeleted()
		}
	}

	return result, nil
}
