// Package executor provides tests for the executor package.
package executor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/planner"
)

// mockS3Client is a mock implementation for testing
type mockS3Client struct{}

func (m *mockS3Client) PutObject(
	ctx context.Context,
	params *s3.PutObjectInput,
	optFns ...func(*s3.Options),
) (*s3.PutObjectOutput, error) {
	// Simulate some work
	time.Sleep(10 * time.Millisecond)
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(
	ctx context.Context,
	params *s3.GetObjectInput,
	optFns ...func(*s3.Options),
) (*s3.GetObjectOutput, error) {
	return &s3.GetObjectOutput{}, nil
}

func (m *mockS3Client) DeleteObject(
	ctx context.Context,
	params *s3.DeleteObjectInput,
	optFns ...func(*s3.Options),
) (*s3.DeleteObjectOutput, error) {
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3Client) DeleteObjects(
	ctx context.Context,
	params *s3.DeleteObjectsInput,
	optFns ...func(*s3.Options),
) (*s3.DeleteObjectsOutput, error) {
	return &s3.DeleteObjectsOutput{}, nil
}

func (m *mockS3Client) ListObjectsV2(
	ctx context.Context,
	params *s3.ListObjectsV2Input,
	optFns ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{}, nil
}

func (m *mockS3Client) HeadObject(
	ctx context.Context,
	params *s3.HeadObjectInput,
	optFns ...func(*s3.Options),
) (*s3.HeadObjectOutput, error) {
	return &s3.HeadObjectOutput{}, nil
}

func (m *mockS3Client) CopyObject(
	ctx context.Context,
	params *s3.CopyObjectInput,
	optFns ...func(*s3.Options),
) (*s3.CopyObjectOutput, error) {
	return &s3.CopyObjectOutput{}, nil
}

func (m *mockS3Client) CreateMultipartUpload(
	ctx context.Context,
	params *s3.CreateMultipartUploadInput,
	optFns ...func(*s3.Options),
) (*s3.CreateMultipartUploadOutput, error) {
	return &s3.CreateMultipartUploadOutput{}, nil
}

func (m *mockS3Client) UploadPart(
	ctx context.Context,
	params *s3.UploadPartInput,
	optFns ...func(*s3.Options),
) (*s3.UploadPartOutput, error) {
	return &s3.UploadPartOutput{}, nil
}

func (m *mockS3Client) UploadPartCopy(
	ctx context.Context,
	params *s3.UploadPartCopyInput,
	optFns ...func(*s3.Options),
) (*s3.UploadPartCopyOutput, error) {
	return &s3.UploadPartCopyOutput{}, nil
}

func (m *mockS3Client) CompleteMultipartUpload(
	ctx context.Context,
	params *s3.CompleteMultipartUploadInput,
	optFns ...func(*s3.Options),
) (*s3.CompleteMultipartUploadOutput, error) {
	return &s3.CompleteMultipartUploadOutput{}, nil
}

func (m *mockS3Client) AbortMultipartUpload(
	ctx context.Context,
	params *s3.AbortMultipartUploadInput,
	optFns ...func(*s3.Options),
) (*s3.AbortMultipartUploadOutput, error) {
	return &s3.AbortMultipartUploadOutput{}, nil
}

func (m *mockS3Client) CreateBucket(
	ctx context.Context,
	params *s3.CreateBucketInput,
	optFns ...func(*s3.Options),
) (*s3.CreateBucketOutput, error) {
	return &s3.CreateBucketOutput{}, nil
}

func (m *mockS3Client) DeleteBucket(
	ctx context.Context,
	params *s3.DeleteBucketInput,
	optFns ...func(*s3.Options),
) (*s3.DeleteBucketOutput, error) {
	return &s3.DeleteBucketOutput{}, nil
}

func TestExecutorConcurrencyControl(t *testing.T) {
	mockClient := &mockS3Client{}
	executor := NewExecutor(mockClient, 2) // Limit to 2 concurrent operations

	// Create test operations
	operations := make([]*planner.Operation, 10)
	for i := range operations {
		operations[i] = &planner.Operation{
			Type:      planner.OperationUpload,
			LocalPath: "/tmp/test" + string(rune(i)),
			RemoteKey: "test" + string(rune(i)),
			Size:      int64(100),
		}
	}

	// Track concurrent operations
	var concurrentOps int64
	var maxConcurrent int64

	// Mock upload function that tracks concurrency
	uploadFunc := func(ctx context.Context, op *planner.Operation) error {
		current := atomic.AddInt64(&concurrentOps, 1)
		defer atomic.AddInt64(&concurrentOps, -1)

		// Track maximum concurrent operations
		for {
			max := atomic.LoadInt64(&maxConcurrent)
			if current <= max || atomic.CompareAndSwapInt64(&maxConcurrent, max, current) {
				break
			}
		}

		// Simulate work
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	ctx := context.Background()
	err := executor.executeWithConcurrency(ctx, operations, uploadFunc)
	if err != nil {
		t.Fatalf("executeWithConcurrency failed: %v", err)
	}

	// Verify that concurrency was limited
	if maxConcurrent > 2 {
		t.Errorf("Expected max concurrent operations <= 2, got %d", maxConcurrent)
	}

	if maxConcurrent < 1 {
		t.Errorf("Expected at least 1 concurrent operation, got %d", maxConcurrent)
	}
}

func TestExecutorStats(t *testing.T) {
	mockClient := &mockS3Client{}
	executor := NewExecutor(mockClient, 3)

	stats := executor.GetStats()
	if stats.MaxConcurrency != 3 {
		t.Errorf("Expected MaxConcurrency=3, got %d", stats.MaxConcurrency)
	}

	if stats.CurrentConcurrency != 0 {
		t.Errorf("Expected CurrentConcurrency=0 initially, got %d", stats.CurrentConcurrency)
	}

	if stats.AvailableSlots != 3 {
		t.Errorf("Expected AvailableSlots=3 initially, got %d", stats.AvailableSlots)
	}
}

func TestExecutorValidateConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		wantErr     bool
	}{
		{"valid concurrency", 5, false},
		{"zero concurrency defaults to 5", 0, false},
		{"negative concurrency defaults to 5", -1, false},
		{"too high concurrency", 101, true},
		{"valid high concurrency", 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockS3Client{}
			executor := NewExecutor(mockClient, tt.concurrency)

			err := executor.ValidateConcurrency()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConcurrency() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecutorGoroutineLeakPrevention(t *testing.T) {
	mockClient := &mockS3Client{}
	executor := NewExecutor(mockClient, 1)

	// Create operations that will take some time
	operations := make([]*planner.Operation, 5)
	for i := range operations {
		operations[i] = &planner.Operation{
			Type:      planner.OperationUpload,
			LocalPath: "/tmp/test" + string(rune(i)),
			RemoteKey: "test" + string(rune(i)),
			Size:      int64(100),
		}
	}

	// Use a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	uploadFunc := func(ctx context.Context, op *planner.Operation) error {
		// Simulate longer work than context timeout
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	// This should not hang due to goroutine leaks
	err := executor.executeWithConcurrency(ctx, operations, uploadFunc)

	// We expect a context cancellation error
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}
}
