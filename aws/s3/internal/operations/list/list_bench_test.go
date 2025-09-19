package list

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
	totalObjects int
	delay        time.Duration
	objectsSent  int
}

func (m *mockS3Client) ListObjectsV2(
	ctx context.Context,
	input *s3.ListObjectsV2Input,
	opts ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	// Simulate API call delay
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	pageSize := int(aws.ToInt32(input.MaxKeys))
	if pageSize > 1000 {
		pageSize = 1000
	}

	remaining := m.totalObjects - m.objectsSent
	if remaining <= 0 {
		return &s3.ListObjectsV2Output{
			IsTruncated: aws.Bool(false),
		}, nil
	}

	objectCount := pageSize
	if objectCount > remaining {
		objectCount = remaining
	}

	contents := make([]types.Object, objectCount)
	for i := 0; i < objectCount; i++ {
		contents[i] = types.Object{
			Key:          aws.String(fmt.Sprintf("object-%d", m.objectsSent+i)),
			Size:         aws.Int64(1024),
			LastModified: aws.Time(time.Now()),
			ETag:         aws.String("etag"),
		}
	}

	m.objectsSent += objectCount
	isTruncated := m.objectsSent < m.totalObjects

	result := &s3.ListObjectsV2Output{
		Contents:    contents,
		IsTruncated: aws.Bool(isTruncated),
		KeyCount:    aws.Int32(int32(objectCount)),
	}

	if isTruncated {
		result.NextContinuationToken = aws.String(fmt.Sprintf("token-%d", m.objectsSent))
	}

	return result, nil
}

// BenchmarkList tests single page listing performance.
func BenchmarkList(b *testing.B) {
	testCases := []struct {
		name     string
		pageSize int32
		objects  int
	}{
		{"SmallPage", 100, 100},
		{"MediumPage", 500, 500},
		{"LargePage", 1000, 1000},
		{"MaxPage", 1000, 1000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{
				totalObjects: tc.objects,
			}
			lister := New(client)

			config := &Config{
				Bucket:  "test-bucket",
				Prefix:  "test-prefix",
				MaxKeys: tc.pageSize,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.objectsSent = 0
				_, err := lister.List(context.Background(), config)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkListAll tests streaming all objects performance.
func BenchmarkListAll(b *testing.B) {
	testCases := []struct {
		name         string
		totalObjects int
		pageSize     int32
	}{
		{"Small-100", 100, 100},
		{"Medium-1000", 1000, 1000},
		{"Large-10000", 10000, 1000},
		{"VeryLarge-100000", 100000, 1000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{
				totalObjects: tc.totalObjects,
			}
			lister := New(client)

			config := &Config{
				Bucket:   "test-bucket",
				Prefix:   "test-prefix",
				PageSize: tc.pageSize,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.objectsSent = 0
				count := 0

				for range lister.ListAll(context.Background(), config) {
					count++
				}

				if count != tc.totalObjects {
					b.Fatalf("expected %d objects, got %d", tc.totalObjects, count)
				}
			}
		})
	}
}

// BenchmarkPaginator tests paginator performance.
func BenchmarkPaginator(b *testing.B) {
	testCases := []struct {
		name         string
		totalObjects int
		pageSize     int32
	}{
		{"SmallDataset", 1000, 100},
		{"MediumDataset", 10000, 500},
		{"LargeDataset", 100000, 1000},
		{"OptimalPageSize", 50000, 1000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{
				totalObjects: tc.totalObjects,
			}
			lister := New(client)

			config := &Config{
				Bucket:   "test-bucket",
				Prefix:   "test-prefix",
				PageSize: tc.pageSize,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.objectsSent = 0
				paginator := lister.ListWithPaginator(context.Background(), config)
				totalObjects := 0

				for paginator.HasMorePages() {
					page, err := paginator.NextPage(context.Background())
					if err != nil {
						b.Fatal(err)
					}
					totalObjects += len(page.Objects)
				}

				if totalObjects != tc.totalObjects {
					b.Fatalf("expected %d objects, got %d", tc.totalObjects, totalObjects)
				}
			}
		})
	}
}

// BenchmarkListPrefixes tests parallel prefix listing performance.
func BenchmarkListPrefixes(b *testing.B) {
	testCases := []struct {
		name         string
		prefixCount  int
		parallelism  int
		objPerPrefix int
	}{
		{"Serial", 10, 1, 100},
		{"Parallel-3", 10, 3, 100},
		{"Parallel-5", 10, 5, 100},
		{"Parallel-10", 10, 10, 100},
		{"ManyPrefixes-Serial", 100, 1, 10},
		{"ManyPrefixes-Parallel", 100, 10, 10},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Create prefixes
			prefixes := make([]string, tc.prefixCount)
			for i := 0; i < tc.prefixCount; i++ {
				prefixes[i] = fmt.Sprintf("prefix-%d", i)
			}

			client := &mockS3Client{
				totalObjects: tc.objPerPrefix,
			}
			lister := New(client)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				count := 0
				for range lister.ListPrefixes(context.Background(), "test-bucket", prefixes, tc.parallelism) {
					count++
				}

				expectedTotal := tc.prefixCount * tc.objPerPrefix
				if count != expectedTotal {
					b.Fatalf("expected %d objects, got %d", expectedTotal, count)
				}
			}
		})
	}
}

// BenchmarkBatchedList tests batched listing performance.
func BenchmarkBatchedList(b *testing.B) {
	testCases := []struct {
		name         string
		totalObjects int
		batchSize    int
	}{
		{"SmallBatch-10", 1000, 10},
		{"MediumBatch-100", 1000, 100},
		{"LargeBatch-500", 1000, 500},
		{"OptimalBatch", 10000, 100},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{
				totalObjects: tc.totalObjects,
			}
			lister := New(client)
			batcher := NewBatchedList(lister, tc.batchSize)

			config := &Config{
				Bucket:   "test-bucket",
				Prefix:   "test-prefix",
				PageSize: 1000,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.objectsSent = 0
				totalObjects := 0

				for batch := range batcher.ListBatches(context.Background(), config) {
					totalObjects += len(batch)
				}

				if totalObjects != tc.totalObjects {
					b.Fatalf("expected %d objects, got %d", tc.totalObjects, totalObjects)
				}
			}
		})
	}
}

// BenchmarkListWithDelay tests performance with simulated network latency.
func BenchmarkListWithDelay(b *testing.B) {
	testCases := []struct {
		name         string
		totalObjects int
		delay        time.Duration
		pageSize     int32
	}{
		{"NoDelay", 1000, 0, 1000},
		{"SmallDelay", 1000, 10 * time.Millisecond, 1000},
		{"MediumDelay", 1000, 50 * time.Millisecond, 1000},
		{"LargeDelay", 1000, 100 * time.Millisecond, 1000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{
				totalObjects: tc.totalObjects,
				delay:        tc.delay,
			}
			lister := New(client)

			config := &Config{
				Bucket:   "test-bucket",
				Prefix:   "test-prefix",
				PageSize: tc.pageSize,
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				client.objectsSent = 0
				count := 0

				for range lister.ListAll(context.Background(), config) {
					count++
				}
			}
		})
	}
}

// BenchmarkMemoryEfficiency tests memory usage for large listings.
func BenchmarkMemoryEfficiency(b *testing.B) {
	testCases := []struct {
		name         string
		totalObjects int
		method       string
	}{
		{"Channel-10K", 10000, "channel"},
		{"Channel-100K", 100000, "channel"},
		{"Batch-10K", 10000, "batch"},
		{"Batch-100K", 100000, "batch"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &mockS3Client{
				totalObjects: tc.totalObjects,
			}
			lister := New(client)

			config := &Config{
				Bucket:   "test-bucket",
				Prefix:   "test-prefix",
				PageSize: 1000,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				client.objectsSent = 0

				if tc.method == "channel" {
					count := 0
					for range lister.ListAll(context.Background(), config) {
						count++
					}
				} else {
					batcher := NewBatchedList(lister, 100)
					count := 0
					for batch := range batcher.ListBatches(context.Background(), config) {
						count += len(batch)
					}
				}
			}
		})
	}
}

// BenchmarkOptimalPageSize tests different page sizes to find optimal value.
func BenchmarkOptimalPageSize(b *testing.B) {
	pageSizes := []int32{10, 50, 100, 250, 500, 750, 1000}
	totalObjects := 10000

	for _, pageSize := range pageSizes {
		b.Run(fmt.Sprintf("PageSize-%d", pageSize), func(b *testing.B) {
			client := &mockS3Client{
				totalObjects: totalObjects,
				delay:        5 * time.Millisecond, // Simulate network latency
			}
			lister := New(client)

			config := &Config{
				Bucket:   "test-bucket",
				Prefix:   "test-prefix",
				PageSize: pageSize,
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				client.objectsSent = 0
				paginator := lister.ListWithPaginator(context.Background(), config)

				for paginator.HasMorePages() {
					_, err := paginator.NextPage(context.Background())
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
