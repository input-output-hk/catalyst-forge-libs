// Package executor handles the parallel execution of sync operations.
// This includes managing concurrency limits and coordinating multiple transfers.
//
// The executor ensures operations are performed efficiently and safely.
package executor

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/s3api"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/planner"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Executor handles the parallel execution of sync operations.
type Executor struct {
	s3Client s3api.S3API

	// Concurrency control
	maxConcurrency int
	semaphore      chan struct{}

	// Progress tracking
	progressTracker s3types.ProgressTracker
}

// NewExecutor creates a new executor with the specified concurrency limit.
func NewExecutor(s3Client s3api.S3API, maxConcurrency int) *Executor {
	if maxConcurrency <= 0 {
		maxConcurrency = 5 // Default concurrency
	}

	return &Executor{
		s3Client:        s3Client,
		maxConcurrency:  maxConcurrency,
		semaphore:       make(chan struct{}, maxConcurrency),
		progressTracker: nil,
	}
}

// WithProgressTracker sets the progress tracker for the executor.
func (e *Executor) WithProgressTracker(tracker s3types.ProgressTracker) *Executor {
	e.progressTracker = tracker
	return e
}

// UploadResult contains the result of upload operations.
type UploadResult struct {
	// filesUploaded is the number of files successfully uploaded (internal counter)
	filesUploaded int64

	// BytesUploaded is the total bytes uploaded
	BytesUploaded int64

	// Errors contains any errors that occurred during uploads
	Errors []UploadError

	// Duration is how long the upload operations took
	Duration time.Duration
}

// FilesUploaded returns the number of files uploaded (safe for concurrent access)
func (r *UploadResult) FilesUploaded() int {
	return int(atomic.LoadInt64(&r.filesUploaded))
}

// UploadError represents an error that occurred during an upload operation.
type UploadError struct {
	// LocalPath is the local file path that failed to upload
	LocalPath string

	// RemoteKey is the S3 key that failed to upload to
	RemoteKey string

	// Error is the underlying error
	Error error
}

// ExecuteUploads executes upload operations with concurrency control.
func (e *Executor) ExecuteUploads(
	ctx context.Context,
	config *SyncConfig,
	operations []*planner.Operation,
) (*UploadResult, error) {
	startTime := time.Now()
	result := &UploadResult{}

	// Filter to only upload operations
	var uploadOps []*planner.Operation
	for _, op := range operations {
		if op.Type == planner.OperationUpload {
			uploadOps = append(uploadOps, op)
		}
	}

	if len(uploadOps) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Execute uploads with concurrency control
	err := e.executeWithConcurrency(ctx, uploadOps, func(ctx context.Context, op *planner.Operation) error {
		return e.uploadFile(ctx, config, op, result)
	})

	result.Duration = time.Since(startTime)
	return result, err
}

// DeleteResult contains the result of delete operations.
type DeleteResult struct {
	// filesDeleted is the number of files successfully deleted (internal counter)
	filesDeleted int64

	// Errors contains any errors that occurred during deletions
	Errors []DeleteError

	// Duration is how long the delete operations took
	Duration time.Duration
}

// FilesDeleted returns the number of files deleted (safe for concurrent access)
func (r *DeleteResult) FilesDeleted() int {
	return int(atomic.LoadInt64(&r.filesDeleted))
}

// DeleteError represents an error that occurred during a delete operation.
type DeleteError struct {
	// RemoteKey is the S3 key that failed to delete
	RemoteKey string

	// Error is the underlying error
	Error error
}

// ExecuteDeletes executes delete operations with batching.
func (e *Executor) ExecuteDeletes(
	ctx context.Context,
	config *SyncConfig,
	operations []*planner.Operation,
) (*DeleteResult, error) {
	startTime := time.Now()
	result := &DeleteResult{}

	// Filter to only delete operations
	var deleteOps []*planner.Operation
	for _, op := range operations {
		if op.Type == planner.OperationDelete {
			deleteOps = append(deleteOps, op)
		}
	}

	if len(deleteOps) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Execute deletes in batches
	err := e.executeDeletesBatch(ctx, config, deleteOps, result)

	result.Duration = time.Since(startTime)
	return result, err
}

// SyncConfig holds configuration for sync operations.
type SyncConfig struct {
	Bucket string
	Prefix string
}

// executeWithConcurrency executes operations with concurrency control.
func (e *Executor) executeWithConcurrency(
	ctx context.Context,
	operations []*planner.Operation,
	operationFunc func(context.Context, *planner.Operation) error,
) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstError error

	// Execute operations with controlled concurrency
	for _, op := range operations {
		// Acquire semaphore
		select {
		case e.semaphore <- struct{}{}:
			// Semaphore acquired
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during semaphore acquisition: %w", ctx.Err())
		}

		wg.Add(1)
		go func(op *planner.Operation) {
			defer func() {
				// Release semaphore
				<-e.semaphore
				wg.Done()
			}()

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Execute operation
			if err := operationFunc(ctx, op); err != nil {
				mu.Lock()
				if firstError == nil {
					firstError = err
				}
				mu.Unlock()
			}
		}(op)
	}

	// Wait for all operations to complete
	wg.Wait()

	return firstError
}

// uploadFile uploads a single file to S3.
func (e *Executor) uploadFile(
	ctx context.Context,
	config *SyncConfig,
	op *planner.Operation,
	result *UploadResult,
) error {
	// Open the local file
	file, err := os.Open(op.LocalPath)
	if err != nil {
		result.Errors = append(result.Errors, UploadError{
			LocalPath: op.LocalPath,
			RemoteKey: op.RemoteKey,
			Error:     fmt.Errorf("failed to open file: %w", err),
		})
		return fmt.Errorf("failed to open file for upload: %w", err)
	}
	defer file.Close()

	// Prepare upload input
	input := &s3.PutObjectInput{
		Bucket: &config.Bucket,
		Key:    &op.RemoteKey,
		Body:   file,
	}

	// Execute upload
	_, err = e.s3Client.PutObject(ctx, input)
	if err != nil {
		result.Errors = append(result.Errors, UploadError{
			LocalPath: op.LocalPath,
			RemoteKey: op.RemoteKey,
			Error:     fmt.Errorf("failed to upload: %w", err),
		})
		return fmt.Errorf("failed to upload file %s to %s: %w", op.LocalPath, op.RemoteKey, err)
	}

	// Update progress
	atomic.AddInt64(&result.filesUploaded, 1)
	atomic.AddInt64(&result.BytesUploaded, op.Size)

	// Update progress tracker if available
	if e.progressTracker != nil {
		e.progressTracker.Update(result.BytesUploaded, 0) // We don't track total for sync
	}

	return nil
}

// executeDeletesBatch executes delete operations in batches.
func (e *Executor) executeDeletesBatch(
	ctx context.Context,
	config *SyncConfig,
	operations []*planner.Operation,
	result *DeleteResult,
) error {
	// AWS S3 allows up to 1000 objects per delete request
	const maxBatchSize = 1000

	// Process operations in batches
	for i := 0; i < len(operations); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(operations) {
			end = len(operations)
		}

		batch := operations[i:end]
		if err := e.executeDeleteBatch(ctx, config, batch, result); err != nil {
			return err
		}
	}

	return nil
}

// executeDeleteBatch executes a single batch of delete operations.
func (e *Executor) executeDeleteBatch(
	ctx context.Context,
	config *SyncConfig,
	operations []*planner.Operation,
	result *DeleteResult,
) error {
	// Build delete objects list
	var objects []types.ObjectIdentifier
	for _, op := range operations {
		objects = append(objects, types.ObjectIdentifier{
			Key: &op.RemoteKey,
		})
	}

	// Prepare delete input
	input := &s3.DeleteObjectsInput{
		Bucket: &config.Bucket,
		Delete: &types.Delete{
			Objects: objects,
		},
	}

	// Execute batch delete
	output, err := e.s3Client.DeleteObjects(ctx, input)
	if err != nil {
		// Add errors for all objects in the batch
		for _, op := range operations {
			result.Errors = append(result.Errors, DeleteError{
				RemoteKey: op.RemoteKey,
				Error:     fmt.Errorf("batch delete failed: %w", err),
			})
		}
		return fmt.Errorf("failed to delete objects from bucket %s: %w", config.Bucket, err)
	}

	// Process results
	successCount := 0
	for _, result := range output.Deleted {
		if result.Key != nil {
			successCount++
		}
	}

	// Handle any delete errors
	for _, deleteError := range output.Errors {
		result.Errors = append(result.Errors, DeleteError{
			RemoteKey: *deleteError.Key,
			Error:     fmt.Errorf("delete error: %s", *deleteError.Message),
		})
	}

	// Update result count
	atomic.AddInt64(&result.filesDeleted, int64(successCount))

	return nil
}

// ExecuteOperations executes both uploads and deletes in a coordinated manner.
func (e *Executor) ExecuteOperations(
	ctx context.Context,
	config *SyncConfig,
	operations []*planner.Operation,
) (*ExecutionResult, error) {
	startTime := time.Now()

	// Separate operations by type
	var uploads, deletes []*planner.Operation
	for _, op := range operations {
		switch op.Type {
		case planner.OperationUpload:
			uploads = append(uploads, op)
		case planner.OperationDelete:
			deletes = append(deletes, op)
		}
	}

	result := &ExecutionResult{}

	// Execute uploads and deletes concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstError error

	// Execute uploads
	if len(uploads) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uploadResult, err := e.ExecuteUploads(ctx, config, uploads)
			mu.Lock()
			defer mu.Unlock()

			if err != nil && firstError == nil {
				firstError = err
			}

			if uploadResult != nil {
				result.UploadResult = *uploadResult
			}
		}()
	}

	// Execute deletes
	if len(deletes) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deleteResult, err := e.ExecuteDeletes(ctx, config, deletes)
			mu.Lock()
			defer mu.Unlock()

			if err != nil && firstError == nil {
				firstError = err
			}

			if deleteResult != nil {
				result.DeleteResult = *deleteResult
			}
		}()
	}

	wg.Wait()

	result.Duration = time.Since(startTime)
	return result, firstError
}

// ExecutionResult contains the combined results of upload and delete operations.
type ExecutionResult struct {
	// UploadResult contains upload operation results
	UploadResult UploadResult

	// DeleteResult contains delete operation results
	DeleteResult DeleteResult

	// Duration is how long all operations took
	Duration time.Duration
}

// ValidateConcurrency checks if the concurrency settings are valid.
func (e *Executor) ValidateConcurrency() error {
	if e.maxConcurrency <= 0 {
		return fmt.Errorf("max concurrency must be positive, got %d", e.maxConcurrency)
	}
	if e.maxConcurrency > 100 {
		return fmt.Errorf("max concurrency too high: %d (recommended: <= 100)", e.maxConcurrency)
	}
	return nil
}

// GetStats returns current execution statistics.
func (e *Executor) GetStats() Stats {
	return Stats{
		MaxConcurrency:     e.maxConcurrency,
		CurrentConcurrency: len(e.semaphore),
		AvailableSlots:     cap(e.semaphore) - len(e.semaphore),
	}
}

// Stats contains statistics about the executor's current state.
type Stats struct {
	// MaxConcurrency is the maximum allowed concurrent operations
	MaxConcurrency int

	// CurrentConcurrency is the current number of running operations
	CurrentConcurrency int

	// AvailableSlots is the number of available concurrency slots
	AvailableSlots int
}
