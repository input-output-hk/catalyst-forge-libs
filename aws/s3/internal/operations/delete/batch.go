package delete

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	s3types "github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// S3Interface defines the S3 operations we need.
type S3Interface interface {
	DeleteObjects(
		ctx context.Context,
		input *s3.DeleteObjectsInput,
		opts ...func(*s3.Options),
	) (*s3.DeleteObjectsOutput, error)
	DeleteObject(
		ctx context.Context,
		input *s3.DeleteObjectInput,
		opts ...func(*s3.Options),
	) (*s3.DeleteObjectOutput, error)
}

// BatchDeleter handles optimized batch deletion of S3 objects.
type BatchDeleter struct {
	client       S3Interface
	maxBatchSize int
}

// New creates a new BatchDeleter with optimal batch size.
func New(client S3Interface) *BatchDeleter {
	return &BatchDeleter{
		client:       client,
		maxBatchSize: 1000, // S3 maximum
	}
}

// DeleteBatch deletes objects in optimal batch sizes.
func (b *BatchDeleter) DeleteBatch(ctx context.Context, bucket string, keys []string) (*s3types.DeleteResult, error) {
	if len(keys) == 0 {
		return &s3types.DeleteResult{}, nil
	}

	// If the batch is small enough, delete directly
	if len(keys) <= b.maxBatchSize {
		return b.deleteBatchDirect(ctx, bucket, keys)
	}

	// For large batches, split and process
	return b.deleteLargeBatch(ctx, bucket, keys)
}

// deleteBatchDirect handles a single batch deletion.
func (b *BatchDeleter) deleteBatchDirect(
	ctx context.Context,
	bucket string,
	keys []string,
) (*s3types.DeleteResult, error) {
	deleteObjects := make([]types.ObjectIdentifier, 0, len(keys))
	for _, key := range keys {
		deleteObjects = append(deleteObjects, types.ObjectIdentifier{
			Key: aws.String(key),
		})
	}

	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: deleteObjects,
			Quiet:   aws.Bool(false), // Get detailed results
		},
	}

	output, err := b.client.DeleteObjects(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("delete objects: %w", err)
	}

	return b.convertOutput(output), nil
}

// deleteLargeBatch handles deletion of more than 1000 objects.
func (b *BatchDeleter) deleteLargeBatch(
	ctx context.Context,
	bucket string,
	keys []string,
) (*s3types.DeleteResult, error) {
	result := &s3types.DeleteResult{
		Deleted: make([]s3types.Object, 0, len(keys)),
		Errors:  make([]s3types.DeleteError, 0),
	}

	// Process in batches
	for i := 0; i < len(keys); i += b.maxBatchSize {
		end := i + b.maxBatchSize
		if end > len(keys) {
			end = len(keys)
		}

		batchResult, err := b.deleteBatchDirect(ctx, bucket, keys[i:end])
		if err != nil {
			// Add error for this batch
			for j := i; j < end; j++ {
				result.Errors = append(result.Errors, s3types.DeleteError{
					Key:     keys[j],
					Code:    "BatchError",
					Message: err.Error(),
				})
			}
			continue
		}

		// Merge results
		result.Deleted = append(result.Deleted, batchResult.Deleted...)
		result.Errors = append(result.Errors, batchResult.Errors...)
	}

	return result, nil
}

// DeleteParallel deletes objects in parallel batches for optimal performance.
func (b *BatchDeleter) DeleteParallel(
	ctx context.Context,
	bucket string,
	keys []string,
	parallelism int,
) (*s3types.DeleteResult, error) {
	if parallelism <= 0 {
		parallelism = 3 // Default parallelism
	}

	if len(keys) <= b.maxBatchSize {
		// Small batch, no need for parallelism
		return b.deleteBatchDirect(ctx, bucket, keys)
	}

	// Split into batches
	batches := b.splitIntoBatches(keys, b.maxBatchSize)

	// Process batches in parallel
	resultChan := make(chan *s3types.DeleteResult, len(batches))
	errorChan := make(chan error, len(batches))
	sem := make(chan struct{}, parallelism)

	var wg sync.WaitGroup

	for _, batch := range batches {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(batchKeys []string) {
			defer func() {
				<-sem // Release semaphore
				wg.Done()
			}()

			result, err := b.deleteBatchDirect(ctx, bucket, batchKeys)
			if err != nil {
				errorChan <- err
				return
			}
			resultChan <- result
		}(batch)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(resultChan)
	close(errorChan)

	// Merge results
	finalResult := &s3types.DeleteResult{
		Deleted: make([]s3types.Object, 0, len(keys)),
		Errors:  make([]s3types.DeleteError, 0),
	}

	for result := range resultChan {
		finalResult.Deleted = append(finalResult.Deleted, result.Deleted...)
		finalResult.Errors = append(finalResult.Errors, result.Errors...)
	}

	// Handle any errors
	for err := range errorChan {
		// Add error entries for failed batches
		finalResult.Errors = append(finalResult.Errors, s3types.DeleteError{
			Code:    "ParallelBatchError",
			Message: err.Error(),
		})
	}

	return finalResult, nil
}

// DeleteStream deletes objects as they come from a channel.
func (b *BatchDeleter) DeleteStream(
	ctx context.Context,
	bucket string,
	keyChan <-chan string,
	parallelism int,
) (*s3types.DeleteResult, error) {
	if parallelism <= 0 {
		parallelism = 3
	}

	var mu sync.Mutex
	finalResult := &s3types.DeleteResult{
		Deleted: make([]s3types.Object, 0),
		Errors:  make([]s3types.DeleteError, 0),
	}

	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup

	// Batch collector
	batch := make([]string, 0, b.maxBatchSize)

	processBatch := func(keys []string) {
		if len(keys) == 0 {
			return
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(batchKeys []string) {
			defer func() {
				<-sem // Release semaphore
				wg.Done()
			}()

			result, err := b.deleteBatchDirect(ctx, bucket, batchKeys)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				for _, key := range batchKeys {
					finalResult.Errors = append(finalResult.Errors, s3types.DeleteError{
						Key:     key,
						Code:    "StreamBatchError",
						Message: err.Error(),
					})
				}
				return
			}

			finalResult.Deleted = append(finalResult.Deleted, result.Deleted...)
			finalResult.Errors = append(finalResult.Errors, result.Errors...)
		}(keys)
	}

	// Collect keys into batches
	for key := range keyChan {
		batch = append(batch, key)

		if len(batch) >= b.maxBatchSize {
			processBatch(batch)
			batch = make([]string, 0, b.maxBatchSize)
		}
	}

	// Process remaining batch
	if len(batch) > 0 {
		processBatch(batch)
	}

	// Wait for all deletions to complete
	wg.Wait()

	return finalResult, nil
}

// convertOutput converts S3 output to our DeleteResult type.
func (b *BatchDeleter) convertOutput(output *s3.DeleteObjectsOutput) *s3types.DeleteResult {
	result := &s3types.DeleteResult{
		Deleted: make([]s3types.Object, 0),
		Errors:  make([]s3types.DeleteError, 0),
	}

	if output.Deleted != nil {
		for _, deleted := range output.Deleted {
			result.Deleted = append(result.Deleted, s3types.Object{
				Key: aws.ToString(deleted.Key),
			})
		}
	}

	if output.Errors != nil {
		for _, err := range output.Errors {
			result.Errors = append(result.Errors, s3types.DeleteError{
				Key:     aws.ToString(err.Key),
				Version: aws.ToString(err.VersionId),
				Code:    aws.ToString(err.Code),
				Message: aws.ToString(err.Message),
			})
		}
	}

	return result
}

// splitIntoBatches splits a slice into batches of specified size.
func (b *BatchDeleter) splitIntoBatches(keys []string, batchSize int) [][]string {
	batches := make([][]string, 0, (len(keys)+batchSize-1)/batchSize)

	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		batches = append(batches, keys[i:end])
	}

	return batches
}

// OptimizedDeleter provides deletion with caching and optimization.
type OptimizedDeleter struct {
	deleter     *BatchDeleter
	pendingKeys []string
	mu          sync.Mutex
	flushSize   int
}

// NewOptimizedDeleter creates an optimized deleter with buffering.
func NewOptimizedDeleter(client S3Interface, flushSize int) *OptimizedDeleter {
	if flushSize <= 0 {
		flushSize = 1000
	}
	return &OptimizedDeleter{
		deleter:     New(client),
		pendingKeys: make([]string, 0, flushSize),
		flushSize:   flushSize,
	}
}

// Add adds a key for deletion.
func (o *OptimizedDeleter) Add(key string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pendingKeys = append(o.pendingKeys, key)
}

// Flush processes all pending deletions.
func (o *OptimizedDeleter) Flush(ctx context.Context, bucket string) (*s3types.DeleteResult, error) {
	o.mu.Lock()
	keys := o.pendingKeys
	o.pendingKeys = make([]string, 0, o.flushSize)
	o.mu.Unlock()

	if len(keys) == 0 {
		return &s3types.DeleteResult{}, nil
	}

	return o.deleter.DeleteBatch(ctx, bucket, keys)
}

// DeleteWithAutoFlush deletes a key and auto-flushes when buffer is full.
func (o *OptimizedDeleter) DeleteWithAutoFlush(ctx context.Context, bucket, key string) (*s3types.DeleteResult, error) {
	o.mu.Lock()
	o.pendingKeys = append(o.pendingKeys, key)
	shouldFlush := len(o.pendingKeys) >= o.flushSize
	o.mu.Unlock()

	if shouldFlush {
		return o.Flush(ctx, bucket)
	}

	return nil, nil
}
