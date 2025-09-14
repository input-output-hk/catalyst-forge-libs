package cache

import (
	"fmt"
	"time"
)

// Config holds configuration for cache behavior.
type Config struct {
	// MaxSizeBytes is the maximum size of the cache in bytes.
	MaxSizeBytes int64
	// DefaultTTL is the default time-to-live for cache entries.
	DefaultTTL time.Duration
}

// Validate checks that the cache configuration is valid.
func (c *Config) Validate() error {
	if c.MaxSizeBytes <= 0 {
		return fmt.Errorf("max size must be greater than 0")
	}
	if c.DefaultTTL <= 0 {
		return fmt.Errorf("default TTL must be greater than 0")
	}
	return nil
}

// SetDefaults applies default values to unset fields in the configuration.
func (c *Config) SetDefaults() {
	// MaxSizeBytes and DefaultTTL should already be set by the caller
	// No defaults needed currently since compression has been removed
}

// Entry represents a single entry in the cache.
type Entry struct {
	// Key is the unique identifier for this cache entry.
	Key string
	// Data contains the cached data.
	Data []byte
	// Metadata contains additional metadata about the entry.
	Metadata map[string]string
	// CreatedAt is when this entry was first created.
	CreatedAt time.Time
	// AccessedAt is when this entry was last accessed.
	AccessedAt time.Time
	// TTL is the time-to-live for this entry. Zero means no expiration.
	TTL time.Duration
	// AccessCount tracks how many times this entry has been accessed.
	AccessCount int64
}

// IsExpired returns true if the cache entry has expired based on its TTL.
func (e *Entry) IsExpired() bool {
	if e.TTL <= 0 {
		return false // Zero TTL means never expires
	}
	return time.Since(e.CreatedAt) > e.TTL
}

// Size returns the approximate size of the cache entry in bytes.
func (e *Entry) Size() int64 {
	size := int64(len(e.Key))
	size += int64(len(e.Data))

	// Estimate metadata size
	for k, v := range e.Metadata {
		size += int64(len(k) + len(v))
	}

	// Add overhead for struct fields and time values
	size += 8 * 3  // 3 int64 fields
	size += 16 * 2 // 2 time.Time fields (approximate)
	size += 8      // map overhead

	return size
}

// Metrics tracks cache performance and usage statistics.
type Metrics struct {
	// Hits is the number of cache hits.
	Hits int64
	// Misses is the number of cache misses.
	Misses int64
	// Evictions is the number of entries evicted from the cache.
	Evictions int64
	// Errors is the number of cache operation errors.
	Errors int64
	// BytesStored is the total number of bytes currently stored in the cache.
	BytesStored int64
	// EntriesStored is the number of entries currently stored in the cache.
	EntriesStored int64
}

// HitRate returns the cache hit rate as a value between 0.0 and 1.0.
func (m *Metrics) HitRate() float64 {
	total := m.Hits + m.Misses
	if total == 0 {
		return 0.0
	}
	return float64(m.Hits) / float64(total)
}

// RecordHit increments the hit counter.
func (m *Metrics) RecordHit() {
	m.Hits++
}

// RecordMiss increments the miss counter.
func (m *Metrics) RecordMiss() {
	m.Misses++
}

// RecordEviction increments the eviction counter.
func (m *Metrics) RecordEviction() {
	m.Evictions++
}

// RecordError increments the error counter.
func (m *Metrics) RecordError() {
	m.Errors++
}

// AddBytesStored increases the bytes stored counter.
func (m *Metrics) AddBytesStored(bytes int64) {
	m.BytesStored += bytes
}

// RemoveBytesStored decreases the bytes stored counter.
func (m *Metrics) RemoveBytesStored(bytes int64) {
	m.BytesStored -= bytes
	if m.BytesStored < 0 {
		m.BytesStored = 0
	}
}

// TagMapping represents a mapping from a tag reference to a digest.
// This enables efficient tag resolution with history tracking and TTL management.
type TagMapping struct {
	// Reference is the tag reference (e.g., "myregistry.com/myimage:latest")
	Reference string `json:"reference"`
	// Digest is the content digest this tag currently points to
	Digest string `json:"digest"`
	// CreatedAt is when this mapping was first created
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when this mapping was last updated
	UpdatedAt time.Time `json:"updated_at"`
	// AccessCount tracks how many times this mapping has been accessed
	AccessCount int64 `json:"access_count"`
	// History contains previous digests this tag has pointed to
	History []TagHistoryEntry `json:"history,omitempty"`
}

// TagHistoryEntry represents a historical mapping of a tag to a digest.
type TagHistoryEntry struct {
	// Digest the tag previously pointed to
	Digest string `json:"digest"`
	// ChangedAt when this change occurred
	ChangedAt time.Time `json:"changed_at"`
}

// TagResolverConfig contains configuration for tag resolution behavior.
type TagResolverConfig struct {
	// DefaultTTL is the default TTL for tag mappings
	DefaultTTL time.Duration
	// MaxHistorySize is the maximum number of historical entries to keep per tag
	MaxHistorySize int
	// EnableHistory enables tracking of tag history (defaults to true)
	EnableHistory bool
}

// Validate checks that the tag resolver configuration is valid.
func (c *TagResolverConfig) Validate() error {
	if c.DefaultTTL <= 0 {
		return fmt.Errorf("default TTL must be greater than 0")
	}
	if c.MaxHistorySize < 0 {
		return fmt.Errorf("max history size cannot be negative")
	}
	return nil
}

// SetDefaults applies default values to unset fields in the tag resolver configuration.
func (c *TagResolverConfig) SetDefaults() {
	if c.MaxHistorySize == 0 {
		c.MaxHistorySize = 10 // Default to keeping 10 historical entries
	}
	if !c.EnableHistory {
		c.EnableHistory = true // Enable history by default
	}
}

// Manager coordinates the overall cache system and manages multiple cache types.
type Manager struct {
	config    Config
	metrics   *Metrics
	createdAt time.Time
}

// NewManager creates a new cache manager with the given configuration.
func NewManager(config Config) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid cache config: %w", err)
	}

	// Set defaults on a copy of the config
	configCopy := config
	configCopy.SetDefaults()

	return &Manager{
		config:    configCopy,
		metrics:   &Metrics{},
		createdAt: time.Now(),
	}, nil
}

// Config returns the cache configuration.
func (cm *Manager) Config() Config {
	return cm.config
}

// Metrics returns the cache metrics.
func (cm *Manager) Metrics() *Metrics {
	return cm.metrics
}

// CreatedAt returns when the cache manager was created.
func (cm *Manager) CreatedAt() time.Time {
	return cm.createdAt
}
