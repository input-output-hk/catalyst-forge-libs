// Package secrets provides tests for the caching functionality.
package secrets

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInMemoryCache(t *testing.T) {
	tests := []struct {
		name       string
		defaultTTL time.Duration
		maxSize    int
		want       *InMemoryCache
	}{
		{
			name:       "valid cache creation",
			defaultTTL: 5 * time.Minute,
			maxSize:    100,
			want: &InMemoryCache{
				maxSize:    100,
				defaultTTL: 5 * time.Minute,
			},
		},
		{
			name:       "unlimited size",
			defaultTTL: 10 * time.Minute,
			maxSize:    0,
			want: &InMemoryCache{
				maxSize:    0,
				defaultTTL: 10 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewInMemoryCache(tt.defaultTTL, tt.maxSize)

			assert.NotNil(t, cache)
			assert.Equal(t, tt.want.maxSize, cache.maxSize)
			assert.Equal(t, tt.want.defaultTTL, cache.defaultTTL)
			assert.NotNil(t, cache.entries)
		})
	}
}

func TestInMemoryCache_Get(t *testing.T) {
	cache := NewInMemoryCache(5*time.Minute, 10)

	t.Run("get non-existent key", func(t *testing.T) {
		value, found := cache.Get("non-existent")
		assert.False(t, found)
		assert.Nil(t, value)
	})

	t.Run("get existing key", func(t *testing.T) {
		cache.Set("test-key", "test-value", time.Minute)
		value, found := cache.Get("test-key")
		assert.True(t, found)
		assert.Equal(t, "test-value", value)
	})

	t.Run("get expired key", func(t *testing.T) {
		cache.Clear() // Clear previous test entries
		cache.Set("expired-key", "expired-value", 10*time.Millisecond)
		time.Sleep(15 * time.Millisecond) // Wait for expiration

		value, found := cache.Get("expired-key")
		assert.False(t, found)
		assert.Nil(t, value)

		// Verify entry was cleaned up
		assert.Equal(t, 0, cache.Size())
	})
}

func TestInMemoryCache_Set(t *testing.T) {
	cache := NewInMemoryCache(5*time.Minute, 10)

	t.Run("set with default TTL", func(t *testing.T) {
		cache.Set("key1", "value1", 0) // Use default TTL
		value, found := cache.Get("key1")
		assert.True(t, found)
		assert.Equal(t, "value1", value)
	})

	t.Run("set with custom TTL", func(t *testing.T) {
		cache.Set("key2", "value2", time.Minute)
		value, found := cache.Get("key2")
		assert.True(t, found)
		assert.Equal(t, "value2", value)
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		cache.Set("key3", "value3", time.Minute)
		cache.Set("key3", "new-value3", time.Minute)

		value, found := cache.Get("key3")
		assert.True(t, found)
		assert.Equal(t, "new-value3", value)
	})
}

func TestInMemoryCache_Size(t *testing.T) {
	cache := NewInMemoryCache(5*time.Minute, 10)

	t.Run("empty cache size", func(t *testing.T) {
		assert.Equal(t, 0, cache.Size())
	})

	t.Run("size after adding entries", func(t *testing.T) {
		cache.Set("key1", "value1", time.Minute)
		cache.Set("key2", "value2", time.Minute)
		assert.Equal(t, 2, cache.Size())
	})

	t.Run("size after expiration", func(t *testing.T) {
		cache.Clear() // Clear previous test entries
		cache.Set("expired", "value", 10*time.Millisecond)
		time.Sleep(15 * time.Millisecond)
		assert.Equal(t, 0, cache.Size())
	})
}

func TestInMemoryCache_Delete(t *testing.T) {
	cache := NewInMemoryCache(5*time.Minute, 10)

	t.Run("delete existing key", func(t *testing.T) {
		cache.Set("key1", "value1", time.Minute)
		assert.Equal(t, 1, cache.Size())

		cache.Delete("key1")
		assert.Equal(t, 0, cache.Size())

		value, found := cache.Get("key1")
		assert.False(t, found)
		assert.Nil(t, value)
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		cache.Delete("non-existent") // Should not panic
		assert.Equal(t, 0, cache.Size())
	})
}

func TestInMemoryCache_Clear(t *testing.T) {
	cache := NewInMemoryCache(5*time.Minute, 10)

	// Add some entries
	cache.Set("key1", "value1", time.Minute)
	cache.Set("key2", "value2", time.Minute)
	cache.Set("key3", "value3", time.Minute)

	assert.Equal(t, 3, cache.Size())

	cache.Clear()
	assert.Equal(t, 0, cache.Size())

	// Verify all entries are gone
	value, found := cache.Get("key1")
	assert.False(t, found)
	assert.Nil(t, value)
}

func TestInMemoryCache_MaxSize(t *testing.T) {
	cache := NewInMemoryCache(5*time.Minute, 2) // Max size of 2

	t.Run("add entries within limit", func(t *testing.T) {
		cache.Set("key1", "value1", time.Minute)
		cache.Set("key2", "value2", time.Minute)
		assert.Equal(t, 2, cache.Size())
	})

	t.Run("add entry beyond limit triggers eviction", func(t *testing.T) {
		// Set different expiration times to test LRU eviction
		cache.Set("key1", "value1", time.Hour)   // Expires later
		cache.Set("key2", "value2", time.Minute) // Expires sooner
		cache.Set("key3", "value3", time.Minute) // This should trigger eviction

		// Should have evicted key2 (oldest expiration)
		assert.Equal(t, 2, cache.Size())

		// key1 should still exist (latest expiration)
		value, found := cache.Get("key1")
		assert.True(t, found)
		assert.Equal(t, "value1", value)

		// key3 should exist (newest)
		value, found = cache.Get("key3")
		assert.True(t, found)
		assert.Equal(t, "value3", value)
	})
}

func TestInMemoryCache_ThreadSafety(t *testing.T) {
	cache := NewInMemoryCache(5*time.Minute, 0) // Unlimited size

	t.Run("concurrent reads and writes", func(t *testing.T) {
		done := make(chan bool, 10)

		// Start multiple goroutines doing reads and writes
		for i := 0; i < 10; i++ {
			go func(id int) {
				key := fmt.Sprintf("key-%d", id)
				cache.Set(key, fmt.Sprintf("value-%d", id), time.Minute)

				value, found := cache.Get(key)
				require.True(t, found)
				require.Equal(t, fmt.Sprintf("value-%d", id), value.(string))

				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		assert.Equal(t, 10, cache.Size())
	})
}

func TestCacheEntry_IsExpired(t *testing.T) {
	now := time.Now()

	t.Run("entry not expired", func(t *testing.T) {
		entry := &cacheEntry{
			expiration: now.Add(time.Minute),
		}
		assert.False(t, entry.isExpired())
	})

	t.Run("entry expired", func(t *testing.T) {
		entry := &cacheEntry{
			expiration: now.Add(-time.Minute),
		}
		assert.True(t, entry.isExpired())
	})
}

// Performance comparison tests

func BenchmarkInMemoryCache_Get(b *testing.B) {
	cache := NewInMemoryCache(5*time.Minute, 1000)

	// Pre-populate cache
	for i := 0; i < 100; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i), time.Minute)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%100)
			cache.Get(key)
			i++
		}
	})
}

func BenchmarkInMemoryCache_Set(b *testing.B) {
	cache := NewInMemoryCache(5*time.Minute, 0) // Unlimited size

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			value := fmt.Sprintf("value-%d", i)
			cache.Set(key, value, time.Minute)
			i++
		}
	})
}

func TestCachePerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Setup clients
	mockAPI := &mockManagerAPI{}
	ctx := context.Background()
	secretName := "perf-test-secret"
	secretValue := "performance-test-value"

	// Mock AWS API with small delay to simulate network latency
	mockAPI.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
		// Simulate network latency
		time.Sleep(10 * time.Millisecond)
		return &secretsmanager.GetSecretValueOutput{
			SecretString: &secretValue,
		}, nil
	}

	// Test non-cached client
	nonCachedClient := &Client{
		api:    mockAPI,
		cache:  nil, // No cache
		logger: nil,
	}

	// Test cached client
	cachedClient := &Client{
		api:    mockAPI,
		cache:  NewInMemoryCache(5*time.Minute, 100),
		logger: nil,
	}

	// Warm up cache
	_, err := cachedClient.GetSecretCached(ctx, secretName)
	require.NoError(t, err)

	// Performance test parameters
	numIterations := 50

	// Test non-cached performance
	start := time.Now()
	for i := 0; i < numIterations; i++ {
		_, err := nonCachedClient.GetSecretCached(ctx, secretName)
		require.NoError(t, err)
	}
	nonCachedDuration := time.Since(start)

	// Test cached performance
	start = time.Now()
	for i := 0; i < numIterations; i++ {
		_, err := cachedClient.GetSecretCached(ctx, secretName)
		require.NoError(t, err)
	}
	cachedDuration := time.Since(start)

	// Calculate performance improvement
	improvement := float64(nonCachedDuration) / float64(cachedDuration)

	t.Logf("Non-cached duration: %v", nonCachedDuration)
	t.Logf("Cached duration: %v", cachedDuration)
	t.Logf("Performance improvement: %.2fx", improvement)

	// Verify cache is working (cached should be significantly faster)
	assert.True(t, improvement > 5.0, "Cache should provide at least 5x performance improvement")

	// Verify cache hit ratio
	assert.Equal(t, 1, cachedClient.GetCacheSize())
}

func TestCacheMemoryEfficiency(t *testing.T) {
	cache := NewInMemoryCache(5*time.Minute, 3) // Small cache for testing eviction

	// Fill cache to capacity
	cache.Set("key1", "value1", time.Minute)
	cache.Set("key2", "value2", time.Minute)
	cache.Set("key3", "value3", time.Minute)

	assert.Equal(t, 3, cache.Size())

	// Add one more entry - should trigger eviction
	cache.Set("key4", "value4", time.Minute)

	// Cache should still have max size
	assert.Equal(t, 3, cache.Size())

	// Verify the oldest entry (key1) was evicted
	_, found := cache.Get("key1")
	assert.False(t, found, "Oldest entry should have been evicted")

	// Verify newer entries are still there
	_, found = cache.Get("key2")
	assert.True(t, found, "key2 should still be in cache")

	_, found = cache.Get("key3")
	assert.True(t, found, "key3 should still be in cache")

	_, found = cache.Get("key4")
	assert.True(t, found, "key4 should be in cache")
}
