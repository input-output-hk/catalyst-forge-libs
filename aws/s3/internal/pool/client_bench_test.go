package pool

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BenchmarkClientPool tests client pool performance.
func BenchmarkClientPool(b *testing.B) {
	testCases := []struct {
		name     string
		poolSize int
		workers  int
	}{
		{"SingleWorker-Pool10", 10, 1},
		{"Workers5-Pool10", 10, 5},
		{"Workers10-Pool10", 10, 10},
		{"Workers20-Pool10", 10, 20},
		{"Workers10-Pool5", 5, 10},
		{"Workers50-Pool20", 20, 50},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			factory := func() (*s3.Client, error) {
				// Mock client creation
				time.Sleep(time.Microsecond) // Simulate creation overhead
				return &s3.Client{}, nil
			}

			pool := NewClientPool(factory, tc.poolSize)
			defer pool.Close()

			b.ResetTimer()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					client, err := pool.Get(context.Background())
					if err != nil {
						b.Fatal(err)
					}
					// Simulate work
					time.Sleep(time.Microsecond)
					pool.Put(client)
				}
			})
		})
	}
}

// BenchmarkClientPoolVsNew compares pooled vs new client creation.
func BenchmarkClientPoolVsNew(b *testing.B) {
	creationDelay := 10 * time.Millisecond // Simulate real client creation overhead

	b.Run("WithPool", func(b *testing.B) {
		factory := func() (*s3.Client, error) {
			time.Sleep(creationDelay)
			return &s3.Client{}, nil
		}

		pool := NewClientPool(factory, 10)
		defer pool.Close()

		b.ResetTimer()
		b.ReportAllocs()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < b.N/10; j++ {
					client, _ := pool.Get(context.Background())
					time.Sleep(time.Millisecond) // Simulate work
					pool.Put(client)
				}
			}()
		}
		wg.Wait()
	})

	b.Run("WithoutPool", func(b *testing.B) {
		factory := func() (*s3.Client, error) {
			time.Sleep(creationDelay)
			return &s3.Client{}, nil
		}

		b.ResetTimer()
		b.ReportAllocs()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < b.N/10; j++ {
					client, _ := factory()
					time.Sleep(time.Millisecond) // Simulate work
					_ = client
				}
			}()
		}
		wg.Wait()
	})
}

// BenchmarkSharedClientManager tests shared client manager performance.
func BenchmarkSharedClientManager(b *testing.B) {
	client := &s3.Client{}
	manager := NewSharedClientManager(client, 5*time.Minute)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c := manager.Get()
			_ = c
		}
	})
}

// BenchmarkConnectionManager tests connection manager performance.
func BenchmarkConnectionManager(b *testing.B) {
	client := &s3.Client{}
	manager := NewConnectionManager(client, 100)

	operation := func(c *s3.Client) error {
		// Simulate S3 operation
		time.Sleep(time.Microsecond)
		return nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := manager.Execute(context.Background(), operation)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkRequestBatcher tests request batching performance.
func BenchmarkRequestBatcher(b *testing.B) {
	testCases := []struct {
		name      string
		batchSize int
		requests  int
	}{
		{"Batch10-Req100", 10, 100},
		{"Batch50-Req500", 50, 500},
		{"Batch100-Req1000", 100, 1000},
		{"Batch1000-Req10000", 1000, 10000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			client := &s3.Client{}
			batcher := NewRequestBatcher(client, tc.batchSize, 100*time.Millisecond)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				for j := 0; j < tc.requests; j++ {
					batcher.Add(struct{}{})
				}
				batcher.Flush()
			}
		})
	}
}

// BenchmarkPoolStats tests the overhead of tracking stats.
func BenchmarkPoolStats(b *testing.B) {
	factory := func() (*s3.Client, error) {
		return &s3.Client{}, nil
	}

	pool := NewClientPool(factory, 10)
	defer pool.Close()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats := pool.Stats()
			_ = stats
		}
	})
}

// BenchmarkConcurrentPoolAccess tests high concurrency scenarios.
func BenchmarkConcurrentPoolAccess(b *testing.B) {
	concurrencyLevels := []int{10, 50, 100, 500, 1000}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			factory := func() (*s3.Client, error) {
				return &s3.Client{}, nil
			}

			pool := NewClientPool(factory, concurrency/10)
			defer pool.Close()

			b.ResetTimer()
			b.ReportAllocs()

			var wg sync.WaitGroup
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < b.N/concurrency; j++ {
						client, err := pool.Get(context.Background())
						if err != nil {
							continue
						}
						pool.Put(client)
					}
				}()
			}
			wg.Wait()
		})
	}
}

// BenchmarkPoolMemoryUsage tests memory efficiency of pooling.
func BenchmarkPoolMemoryUsage(b *testing.B) {
	testCases := []struct {
		name       string
		poolSize   int
		iterations int
	}{
		{"SmallPool-ManyOps", 5, 10000},
		{"MediumPool-ManyOps", 20, 10000},
		{"LargePool-ManyOps", 100, 10000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			factory := func() (*s3.Client, error) {
				// Allocate some memory to simulate real client
				data := make([]byte, 1024)
				_ = data
				return &s3.Client{}, nil
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				pool := NewClientPool(factory, tc.poolSize)

				// Perform many operations
				for j := 0; j < tc.iterations; j++ {
					client, err := pool.Get(context.Background())
					if err != nil {
						continue
					}
					pool.Put(client)
				}

				pool.Close()
			}
		})
	}
}
