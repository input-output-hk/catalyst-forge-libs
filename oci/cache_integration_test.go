package ocibundle

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/oci/internal/cache"
)

// mockCache implements the cache.Cache interface for testing
type mockCache struct {
	data map[string]*cache.Entry
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string]*cache.Entry),
	}
}

func (m *mockCache) Get(ctx context.Context, key string) (*cache.Entry, error) {
	if entry, exists := m.data[key]; exists {
		return entry, nil
	}
	return nil, cache.ErrCacheExpired // Simulate cache miss
}

func (m *mockCache) Put(ctx context.Context, key string, entry *cache.Entry) error {
	m.data[key] = entry
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockCache) Clear(ctx context.Context) error {
	m.data = make(map[string]*cache.Entry)
	return nil
}

func (m *mockCache) Size(ctx context.Context) (int64, error) {
	var totalSize int64
	for _, entry := range m.data {
		totalSize += entry.Size()
	}
	return totalSize, nil
}

// TestPullWithCacheDisabled tests that PullWithCache falls back to regular Pull when caching is disabled
func TestPullWithCacheDisabled(t *testing.T) {
	// Create client without cache configuration
	client, err := New()
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Verify cache is not configured
	assert.Nil(t, client.options.CacheConfig)

	// This test would require a mock ORAS client to fully test
	// For now, we verify the client can be created without cache
}

// TestWithCacheOption tests the WithCache functional option
func TestWithCacheOption(t *testing.T) {
	mockCache := newMockCache()

	client, err := NewWithOptions(
		WithCache(mockCache, "/tmp/test-cache", 100*1024*1024, 24*time.Hour),
	)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.options.CacheConfig)
	assert.Equal(t, mockCache, client.options.CacheConfig.Coordinator)
	assert.Equal(t, "/tmp/test-cache", client.options.CacheConfig.CachePath)
	assert.Equal(t, CachePolicyEnabled, client.options.CacheConfig.Policy)
	assert.Equal(t, int64(100*1024*1024), client.options.CacheConfig.MaxSizeBytes)
	assert.Equal(t, 24*time.Hour, client.options.CacheConfig.DefaultTTL)
}

// TestWithCachePolicyOption tests the WithCachePolicy functional option
func TestWithCachePolicyOption(t *testing.T) {
	client, err := NewWithOptions(
		WithCachePolicy(CachePolicyPull),
	)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.options.CacheConfig)
	assert.Equal(t, CachePolicyPull, client.options.CacheConfig.Policy)
}

// TestCacheBypassOptions tests cache bypass options
func TestCacheBypassOptions(t *testing.T) {
	// Test pull cache bypass
	pullOpts := DefaultPullOptions()
	WithPullCacheBypass(true)(pullOpts)
	assert.True(t, pullOpts.CacheBypass)

	// Test push cache bypass
	pushOpts := DefaultPushOptions()
	WithPushCacheBypass(true)(pushOpts)
	assert.True(t, pushOpts.CacheBypass)

	// Test convenience alias
	pullOpts2 := DefaultPullOptions()
	WithCacheBypass(true)(pullOpts2)
	assert.True(t, pullOpts2.CacheBypass)
}

// TestIsCachingEnabledForPull tests the cache enabling logic for pull operations
func TestIsCachingEnabledForPull(t *testing.T) {
	mockCache := newMockCache()

	tests := []struct {
		name     string
		client   *Client
		opts     *PullOptions
		expected bool
	}{
		{
			name:     "cache disabled by default",
			client:   &Client{options: DefaultClientOptions()},
			opts:     DefaultPullOptions(),
			expected: false,
		},
		{
			name: "cache enabled with policy",
			client: &Client{options: &ClientOptions{
				CacheConfig: &CacheConfig{
					Coordinator: mockCache,
					Policy:      CachePolicyEnabled,
				},
			}},
			opts:     DefaultPullOptions(),
			expected: true,
		},
		{
			name: "cache enabled with pull policy",
			client: &Client{options: &ClientOptions{
				CacheConfig: &CacheConfig{
					Coordinator: mockCache,
					Policy:      CachePolicyPull,
				},
			}},
			opts:     DefaultPullOptions(),
			expected: true,
		},
		{
			name: "cache disabled with push policy",
			client: &Client{options: &ClientOptions{
				CacheConfig: &CacheConfig{
					Coordinator: mockCache,
					Policy:      CachePolicyPush,
				},
			}},
			opts:     DefaultPullOptions(),
			expected: false,
		},
		{
			name: "cache bypassed",
			client: &Client{options: &ClientOptions{
				CacheConfig: &CacheConfig{
					Coordinator: mockCache,
					Policy:      CachePolicyEnabled,
				},
			}},
			opts: &PullOptions{
				CacheBypass: true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.client.isCachingEnabledForPull(tt.opts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsCachingEnabledForPush tests the cache enabling logic for push operations
func TestIsCachingEnabledForPush(t *testing.T) {
	mockCache := newMockCache()

	tests := []struct {
		name     string
		client   *Client
		opts     *PushOptions
		expected bool
	}{
		{
			name:     "cache disabled by default",
			client:   &Client{options: DefaultClientOptions()},
			opts:     DefaultPushOptions(),
			expected: false,
		},
		{
			name: "cache enabled with policy",
			client: &Client{options: &ClientOptions{
				CacheConfig: &CacheConfig{
					Coordinator: mockCache,
					Policy:      CachePolicyEnabled,
				},
			}},
			opts:     DefaultPushOptions(),
			expected: true,
		},
		{
			name: "cache enabled with push policy",
			client: &Client{options: &ClientOptions{
				CacheConfig: &CacheConfig{
					Coordinator: mockCache,
					Policy:      CachePolicyPush,
				},
			}},
			opts:     DefaultPushOptions(),
			expected: true,
		},
		{
			name: "cache disabled with pull policy",
			client: &Client{options: &ClientOptions{
				CacheConfig: &CacheConfig{
					Coordinator: mockCache,
					Policy:      CachePolicyPull,
				},
			}},
			opts:     DefaultPushOptions(),
			expected: false,
		},
		{
			name: "cache bypassed",
			client: &Client{options: &ClientOptions{
				CacheConfig: &CacheConfig{
					Coordinator: mockCache,
					Policy:      CachePolicyEnabled,
				},
			}},
			opts: &PushOptions{
				CacheBypass: true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.client.isCachingEnabledForPush(tt.opts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGenerateCacheKey tests cache key generation
func TestGenerateCacheKey(t *testing.T) {
	client, err := New()
	require.NoError(t, err)

	key := client.generateCacheKey("ghcr.io/org/repo:tag")
	assert.Equal(t, "pull:ghcr.io/org/repo:tag", key)

	key2 := client.generateCacheKey("different-reference")
	assert.Equal(t, "pull:different-reference", key2)
	assert.NotEqual(t, key, key2)
}

// TestEnsureCacheInitialized tests cache initialization
func TestEnsureCacheInitialized(t *testing.T) {
	mockCache := newMockCache()

	// Test with no cache configuration
	client := &Client{options: DefaultClientOptions()}
	err := client.ensureCacheInitialized(context.Background())
	assert.Error(t, err)
	assert.Nil(t, client.cache)

	// Test with cache configuration
	client = &Client{
		options: &ClientOptions{
			CacheConfig: &CacheConfig{
				Coordinator: mockCache,
			},
		},
	}
	err = client.ensureCacheInitialized(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, mockCache, client.cache)

	// Test subsequent calls (should be no-op)
	err = client.ensureCacheInitialized(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, mockCache, client.cache)
}

// TestCachePolicyConstants tests that cache policy constants are defined correctly
func TestCachePolicyConstants(t *testing.T) {
	assert.Equal(t, CachePolicy("disabled"), CachePolicyDisabled)
	assert.Equal(t, CachePolicy("enabled"), CachePolicyEnabled)
	assert.Equal(t, CachePolicy("pull"), CachePolicyPull)
	assert.Equal(t, CachePolicy("push"), CachePolicyPush)
}

// TestDefaultOptionsIncludeCacheFields tests that default options include cache fields
func TestDefaultOptionsIncludeCacheFields(t *testing.T) {
	clientOpts := DefaultClientOptions()
	assert.Nil(t, clientOpts.CacheConfig)

	pullOpts := DefaultPullOptions()
	assert.False(t, pullOpts.CacheBypass)

	pushOpts := DefaultPushOptions()
	assert.False(t, pushOpts.CacheBypass)
}

// TestPullWithCacheMethodExists tests that the PullWithCache method exists and can be called
// This is a basic smoke test - full functionality would require more complex mocking
func TestPullWithCacheMethodExists(t *testing.T) {
	client, err := New()
	require.NoError(t, err)

	// Create temporary directory for target
	tempDir, err := os.MkdirTemp("", "oci-cache-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test with empty reference (should fail validation)
	err = client.PullWithCache(context.Background(), "", tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reference cannot be empty")

	// Test with empty target directory (should fail validation)
	err = client.PullWithCache(context.Background(), "test:ref", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target directory cannot be empty")
}
