// Package secrets provides caching functionality for AWS Secrets Manager operations.
package secrets

import (
	"sync"
	"time"
)

// cacheEntry represents a single cached item with expiration time.
type cacheEntry struct {
	value      any
	expiration time.Time
}

// isExpired checks if the cache entry has expired.
func (e *cacheEntry) isExpired() bool {
	return time.Now().After(e.expiration)
}

// InMemoryCache provides a thread-safe in-memory cache implementation with TTL support.
// It uses a map to store cache entries and a mutex for concurrent access protection.
type InMemoryCache struct {
	// entries holds the cached values with their expiration times
	entries map[string]*cacheEntry

	// maxSize limits the number of entries in the cache (0 = unlimited)
	maxSize int

	// defaultTTL is the default time-to-live for cache entries
	defaultTTL time.Duration

	// mu protects concurrent access to the entries map
	mu sync.RWMutex
}

// NewInMemoryCache creates a new in-memory cache with the specified default TTL and maximum size.
// If maxSize is 0, the cache has no size limit.
func NewInMemoryCache(defaultTTL time.Duration, maxSize int) *InMemoryCache {
	return &InMemoryCache{
		entries:    make(map[string]*cacheEntry),
		maxSize:    maxSize,
		defaultTTL: defaultTTL,
	}
}

// Get retrieves a value from the cache by key.
// Returns the value and true if found and not expired, nil and false if not found or expired.
func (c *InMemoryCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if entry.isExpired() {
		// Clean up expired entry
		delete(c.entries, key)
		return nil, false
	}

	return entry.value, true
}

// Set stores a value in the cache with the specified key and TTL.
// If ttl is 0, the default TTL is used.
// If the cache is at maximum capacity, the oldest entry is evicted.
func (c *InMemoryCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	// Calculate expiration time
	expiration := time.Now().Add(ttl)

	// Check if we need to evict entries when at max capacity
	if c.maxSize > 0 && len(c.entries) >= c.maxSize {
		// Find the oldest entry to evict
		var oldestKey string
		oldestTime := time.Now().Add(time.Hour) // Far future

		for k, entry := range c.entries {
			if entry.expiration.Before(oldestTime) {
				oldestTime = entry.expiration
				oldestKey = k
			}
		}

		// Evict the oldest entry if we found one
		if oldestKey != "" {
			delete(c.entries, oldestKey)
		}
	}

	// Store the new entry
	c.entries[key] = &cacheEntry{
		value:      value,
		expiration: expiration,
	}
}

// Delete removes a specific key from the cache.
func (c *InMemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Clear removes all entries from the cache.
func (c *InMemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

// Size returns the current number of entries in the cache (excluding expired entries).
func (c *InMemoryCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up expired entries while counting
	count := 0
	for key, entry := range c.entries {
		if entry.isExpired() {
			delete(c.entries, key)
		} else {
			count++
		}
	}

	return count
}
