package delete

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// mockS3Client implements S3Interface for benchmarking.
type mockS3Client struct {
	deleteDelay  time.Duration
	failureRate  float32
	deletedCount int
}

func (m *mockS3Client) DeleteObjects(
	ctx context.Context,
	input *s3.DeleteObjectsInput,
	opts ...func(*s3.Options),
) (*s3.DeleteObjectsOutput, error) {
	// Simulate API call delay
	if m.deleteDelay > 0 {
		time.Sleep(m.deleteDelay)
	}

	output := &s3.DeleteObjectsOutput{
		Deleted: make([]types.DeletedObject, 0),
		Errors:  make([]types.Error, 0),
	}

	for _, obj := range input.Delete.Objects {
		m.deletedCount++
		// Simulate some failures based on failure rate
		if m.failureRate > 0 && float32(m.deletedCount%100)/100 < m.failureRate {
			output.Errors = append(output.Errors, types.Error{
				Key:     obj.Key,
				Code:    aws.String("AccessDenied"),
				Message: aws.String("Simulated error"),
			})
		} else {
			output.Deleted = append(output.Deleted, types.DeletedObject{
				Key: obj.Key,
			})
		}
	}

	return output, nil
}

func (m *mockS3Client) DeleteObject(
	ctx context.Context,
	input *s3.DeleteObjectInput,
	opts ...func(*s3.Options),
) (*s3.DeleteObjectOutput, error) {
	// Simulate API call delay
	if m.deleteDelay > 0 {
		time.Sleep(m.deleteDelay)
	}
	m.deletedCount++
	return &s3.DeleteObjectOutput{}, nil
}

// BenchmarkDeleteBatch tests batch deletion performance.
func BenchmarkDeleteBatch(b *testing.B) {
	testCases := []struct {
		name      string
		keyCount  int
		batchSize int
	}{
		{"Small-100", 100, 100},
		{"Medium-500", 500, 500},
		{"Large-1000", 1000, 1000},
		{"VeryLarge-5000", 5000, 1000},
		{"Huge-10000", 10000, 1000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{}
			deleter := New(client)

			// Generate keys
			keys := make([]string, tc.keyCount)
			for i := 0; i < tc.keyCount; i++ {
				keys[i] = fmt.Sprintf("object-%d", i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.deletedCount = 0
				_, err := deleter.DeleteBatch(context.Background(), "test-bucket", keys)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDeleteParallel tests parallel batch deletion performance.
func BenchmarkDeleteParallel(b *testing.B) {
	testCases := []struct {
		name        string
		keyCount    int
		parallelism int
	}{
		{"Serial-5000", 5000, 1},
		{"Parallel2-5000", 5000, 2},
		{"Parallel3-5000", 5000, 3},
		{"Parallel5-5000", 5000, 5},
		{"Parallel10-5000", 5000, 10},
		{"Serial-10000", 10000, 1},
		{"Parallel5-10000", 10000, 5},
		{"Parallel10-10000", 10000, 10},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{
				deleteDelay: 10 * time.Millisecond, // Simulate network latency
			}
			deleter := New(client)

			// Generate keys
			keys := make([]string, tc.keyCount)
			for i := 0; i < tc.keyCount; i++ {
				keys[i] = fmt.Sprintf("object-%d", i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.deletedCount = 0
				_, err := deleter.DeleteParallel(context.Background(), "test-bucket", keys, tc.parallelism)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDeleteStream tests streaming deletion performance.
func BenchmarkDeleteStream(b *testing.B) {
	testCases := []struct {
		name        string
		keyCount    int
		parallelism int
	}{
		{"Stream-1000", 1000, 3},
		{"Stream-5000", 5000, 3},
		{"Stream-10000", 10000, 3},
		{"StreamParallel5-10000", 10000, 5},
		{"StreamParallel10-10000", 10000, 10},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{}
			deleter := New(client)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.deletedCount = 0

				// Create channel and send keys
				keyChan := make(chan string, 100)
				go func() {
					for j := 0; j < tc.keyCount; j++ {
						keyChan <- fmt.Sprintf("object-%d", j)
					}
					close(keyChan)
				}()

				result, err := deleter.DeleteStream(context.Background(), "test-bucket", keyChan, tc.parallelism)
				if err != nil {
					b.Fatal(err)
				}

				if len(result.Deleted) != tc.keyCount {
					b.Fatalf("expected %d deleted, got %d", tc.keyCount, len(result.Deleted))
				}
			}
		})
	}
}

// BenchmarkOptimizedDeleter tests the optimized deleter with auto-flush.
func BenchmarkOptimizedDeleter(b *testing.B) {
	testCases := []struct {
		name      string
		keyCount  int
		flushSize int
	}{
		{"FlushSize-100", 1000, 100},
		{"FlushSize-500", 1000, 500},
		{"FlushSize-1000", 1000, 1000},
		{"LargeDataFlush-1000", 10000, 1000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{}
			deleter := NewOptimizedDeleter(client, tc.flushSize)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.deletedCount = 0

				// Add keys
				for j := 0; j < tc.keyCount; j++ {
					key := fmt.Sprintf("object-%d", j)
					_, _ = deleter.DeleteWithAutoFlush(context.Background(), "test-bucket", key)
				}

				// Final flush
				result, err := deleter.Flush(context.Background(), "test-bucket")
				if err != nil {
					b.Fatal(err)
				}

				if len(result.Deleted) > tc.keyCount {
					b.Fatalf("deleted more than expected: %d > %d", len(result.Deleted), tc.keyCount)
				}
			}
		})
	}
}

// BenchmarkDeleteWithLatency tests performance with network latency.
func BenchmarkDeleteWithLatency(b *testing.B) {
	testCases := []struct {
		name        string
		keyCount    int
		delay       time.Duration
		parallelism int
	}{
		{"NoDelay", 1000, 0, 1},
		{"Delay10ms", 1000, 10 * time.Millisecond, 1},
		{"Delay50ms", 1000, 50 * time.Millisecond, 1},
		{"Delay10ms-Parallel5", 1000, 10 * time.Millisecond, 5},
		{"Delay50ms-Parallel5", 1000, 50 * time.Millisecond, 5},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{
				deleteDelay: tc.delay,
			}
			deleter := New(client)

			// Generate keys
			keys := make([]string, tc.keyCount)
			for i := 0; i < tc.keyCount; i++ {
				keys[i] = fmt.Sprintf("object-%d", i)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				client.deletedCount = 0
				if tc.parallelism > 1 {
					_, err := deleter.DeleteParallel(context.Background(), "test-bucket", keys, tc.parallelism)
					if err != nil {
						b.Fatal(err)
					}
				} else {
					_, err := deleter.DeleteBatch(context.Background(), "test-bucket", keys)
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

// BenchmarkDeleteWithErrors tests performance when some deletions fail.
func BenchmarkDeleteWithErrors(b *testing.B) {
	testCases := []struct {
		name        string
		keyCount    int
		failureRate float32
	}{
		{"NoErrors", 1000, 0},
		{"Errors5Percent", 1000, 0.05},
		{"Errors10Percent", 1000, 0.10},
		{"Errors20Percent", 1000, 0.20},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{
				failureRate: tc.failureRate,
			}
			deleter := New(client)

			// Generate keys
			keys := make([]string, tc.keyCount)
			for i := 0; i < tc.keyCount; i++ {
				keys[i] = fmt.Sprintf("object-%d", i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.deletedCount = 0
				result, err := deleter.DeleteBatch(context.Background(), "test-bucket", keys)
				if err != nil {
					b.Fatal(err)
				}

				expectedErrors := int(float32(tc.keyCount) * tc.failureRate)
				if len(result.Errors) > expectedErrors*2 {
					b.Fatalf("too many errors: %d", len(result.Errors))
				}
			}
		})
	}
}

// BenchmarkBatchSplitting tests the efficiency of batch splitting.
func BenchmarkBatchSplitting(b *testing.B) {
	testCases := []struct {
		name      string
		keyCount  int
		batchSize int
	}{
		{"Split-2000-500", 2000, 500},
		{"Split-5000-1000", 5000, 1000},
		{"Split-10000-1000", 10000, 1000},
		{"Split-100000-1000", 100000, 1000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			deleter := &BatchDeleter{
				maxBatchSize: tc.batchSize,
			}

			// Generate keys
			keys := make([]string, tc.keyCount)
			for i := 0; i < tc.keyCount; i++ {
				keys[i] = fmt.Sprintf("object-%d", i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				batches := deleter.splitIntoBatches(keys, tc.batchSize)
				expectedBatches := (tc.keyCount + tc.batchSize - 1) / tc.batchSize
				if len(batches) != expectedBatches {
					b.Fatalf("expected %d batches, got %d", expectedBatches, len(batches))
				}
			}
		})
	}
}

// BenchmarkMemoryUsage tests memory efficiency of different deletion methods.
func BenchmarkMemoryUsage(b *testing.B) {
	testCases := []struct {
		name     string
		keyCount int
		method   string
	}{
		{"Direct-10000", 10000, "direct"},
		{"Parallel-10000", 10000, "parallel"},
		{"Stream-10000", 10000, "stream"},
		{"Optimized-10000", 10000, "optimized"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{}

			// Generate keys
			keys := make([]string, tc.keyCount)
			for i := 0; i < tc.keyCount; i++ {
				keys[i] = fmt.Sprintf("object-%d", i)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.deletedCount = 0

				switch tc.method {
				case "direct":
					deleter := New(client)
					_, _ = deleter.DeleteBatch(context.Background(), "test-bucket", keys)

				case "parallel":
					deleter := New(client)
					_, _ = deleter.DeleteParallel(context.Background(), "test-bucket", keys, 5)

				case "stream":
					deleter := New(client)
					keyChan := make(chan string, 100)
					go func() {
						for _, key := range keys {
							keyChan <- key
						}
						close(keyChan)
					}()
					_, _ = deleter.DeleteStream(context.Background(), "test-bucket", keyChan, 5)

				case "optimized":
					deleter := NewOptimizedDeleter(client, 1000)
					for _, key := range keys {
						_, _ = deleter.DeleteWithAutoFlush(context.Background(), "test-bucket", key)
					}
					_, _ = deleter.Flush(context.Background(), "test-bucket")
				}
			}
		})
	}
}
