package cache

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	billyfs "github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Coordinator coordinates multiple cache types and implements comprehensive cache management.
// It provides a unified interface for manifest and blob caching with size limits, eviction,
// and metrics collection.
type Coordinator struct {
	mu            sync.RWMutex
	config        Config
	storage       *Storage
	manifestCache ManifestCache
	blobCache     BlobCache
	tagCache      TagCache
	index         *Index
	eviction      EvictionStrategy
	metrics       *Metrics
	cleanupTicker *time.Ticker
	cleanupDone   chan struct{}
	initialized   bool
}

// NewCoordinator creates a new cache coordinator with the specified configuration.
// It initializes all cache components and starts background cleanup processes.
func NewCoordinator(
	ctx context.Context,
	config Config,
	fs *billyfs.FS,
	cachePath string,
) (*Coordinator, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid cache config: %w", err)
	}

	// Apply defaults
	config.SetDefaults()

	// Initialize storage layer
	storage, err := NewStorage(fs, cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Create coordinator instance
	coordinator := &Coordinator{
		config:      config,
		storage:     storage,
		metrics:     &Metrics{},
		cleanupDone: make(chan struct{}),
	}

	// Initialize caches
	if err := coordinator.initializeCaches(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize caches: %w", err)
	}

	// Start background cleanup
	coordinator.startCleanupScheduler(ctx)

	coordinator.initialized = true
	return coordinator, nil
}

// initializeCaches sets up all cache components.
func (cm *Coordinator) initializeCaches(ctx context.Context) error {
	// Initialize index for fast lookups
	indexPath := filepath.Join(cm.storage.rootPath, "index.db")
	cm.index = NewIndex(indexPath, 10000) // Max 10k entries before compaction

	// Load existing index if available
	if err := cm.index.Load(ctx); err != nil {
		return fmt.Errorf("failed to load cache index: %w", err)
	}

	// Initialize eviction strategy (composite of LRU and size-based)
	lruEviction := NewLRUEviction()
	sizeEviction := NewSizeEviction(cm.config.MaxSizeBytes)
	cm.eviction = NewCompositeEviction(
		[]EvictionStrategy{lruEviction, sizeEviction},
		[]int{1, 2}, // Size-based eviction has higher priority
	)

	// Initialize manifest cache
	cm.manifestCache = NewManifestCache(cm.storage, cm) // Pass self as manager

	// Initialize blob cache
	blobCache, err := NewBlobCache(cm.storage, cm.config.DefaultTTL)
	if err != nil {
		return fmt.Errorf("failed to initialize blob cache: %w", err)
	}
	cm.blobCache = blobCache

	// Initialize tag cache - simplified for now
	cm.tagCache = &tagResolver{} // Placeholder implementation

	return nil
}

// startCleanupScheduler starts a background goroutine for periodic cache cleanup.
func (cm *Coordinator) startCleanupScheduler(ctx context.Context) {
	cm.mu.Lock()
	cm.cleanupTicker = time.NewTicker(30 * time.Minute) // Cleanup every 30 minutes
	ticker := cm.cleanupTicker
	cm.mu.Unlock()

	go func() {
		defer func() {
			cm.mu.Lock()
			if cm.cleanupTicker != nil {
				cm.cleanupTicker.Stop()
			}
			cm.mu.Unlock()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-cm.cleanupDone:
				return
			case <-ticker.C:
				if err := cm.performCleanup(ctx); err != nil {
					// Log error but don't crash - cleanup is best-effort
					cm.mu.Lock()
					cm.metrics.RecordError()
					cm.mu.Unlock()
				}
			}
		}
	}()
}

// performCleanup runs maintenance operations on all cache components.
func (cm *Coordinator) performCleanup(ctx context.Context) error {
	// Clean up expired entries
	if err := cm.cleanupExpiredEntries(ctx); err != nil {
		return fmt.Errorf("failed to cleanup expired entries: %w", err)
	}

	// Evict entries if over size limit
	if err := cm.performEviction(ctx); err != nil {
		return fmt.Errorf("failed to perform eviction: %w", err)
	}

	// Compact index if needed
	if err := cm.index.Cleanup(ctx, false); err != nil {
		return fmt.Errorf("failed to cleanup index: %w", err)
	}

	// Persist current state
	return cm.index.Persist()
}

// cleanupExpiredEntries removes all expired entries from all caches.
func (cm *Coordinator) cleanupExpiredEntries(ctx context.Context) error {
	cm.mu.RLock()
	// Get expired keys from index (this acquires index mutex internally)
	expiredKeys := cm.index.ExpiredKeys()
	cm.mu.RUnlock()

	for _, key := range expiredKeys {
		if err := cm.deleteEntry(ctx, key); err != nil {
			// Continue with other entries even if one fails
			continue
		}
		cm.mu.Lock()
		cm.metrics.RecordEviction()
		cm.mu.Unlock()
	}

	return nil
}

// performEviction evicts entries when cache size exceeds limits.
func (cm *Coordinator) performEviction(ctx context.Context) error {
	cm.mu.RLock()
	// Get current cache size
	size, err := cm.Size(ctx)
	if err != nil {
		cm.mu.RUnlock()
		return fmt.Errorf("failed to get cache size: %w", err)
	}

	// If under limit, no eviction needed
	if size <= cm.config.MaxSizeBytes {
		cm.mu.RUnlock()
		return nil
	}

	// Get all current entries for eviction decision
	allKeys := cm.index.Keys(nil)
	cm.mu.RUnlock()

	entries := make(map[string]*Entry)

	for _, key := range allKeys {
		if indexEntry, exists := cm.index.Get(key); exists {
			entries[key] = &Entry{
				Key:        key,
				Data:       []byte{}, // Empty data for eviction decision
				AccessedAt: indexEntry.AccessedAt,
				TTL:        indexEntry.TTL,
			}
		}
	}

	// Select entries for eviction
	toEvict := cm.eviction.SelectForEviction(entries)

	// Evict selected entries
	for _, key := range toEvict {
		if err := cm.deleteEntry(ctx, key); err != nil {
			continue // Continue with other entries
		}
		cm.mu.Lock()
		cm.metrics.RecordEviction()
		cm.mu.Unlock()
	}

	return nil
}

// GetManifest retrieves a manifest from the cache.
func (cm *Coordinator) GetManifest(
	ctx context.Context,
	digest string,
) (*ocispec.Manifest, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	manifest, err := cm.manifestCache.GetManifest(ctx, digest)
	if err != nil {
		cm.metrics.RecordMiss()
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	cm.metrics.RecordHit()

	// Update access tracking
	if indexEntry, exists := cm.index.Get(digest); exists {
		cm.eviction.OnAccess(&Entry{
			Key:        digest,
			AccessedAt: time.Now(),
		})
		indexEntry.AccessedAt = time.Now()
		indexEntry.AccessCount++
	}

	return manifest, nil
}

// PutManifest stores a manifest in the cache.
func (cm *Coordinator) PutManifest(
	ctx context.Context,
	digest string,
	manifest *ocispec.Manifest,
) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if err := cm.manifestCache.PutManifest(ctx, digest, manifest); err != nil {
		cm.metrics.RecordError()
		return fmt.Errorf("failed to put manifest: %w", err)
	}

	// Add to index
	size := int64(len(digest)) // Approximate size for manifest keys
	indexEntry := &IndexEntry{
		Key:         digest,
		Size:        size,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		TTL:         cm.config.DefaultTTL,
		AccessCount: 1,
		FilePath:    "manifests/" + digest,
	}

	if err := cm.index.Put(digest, indexEntry); err != nil {
		cm.metrics.RecordError()
		return fmt.Errorf("failed to index manifest: %w", err)
	}

	// Notify eviction strategy
	entry := &Entry{
		Key:        digest,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        cm.config.DefaultTTL,
	}
	cm.eviction.OnAdd(entry)

	return nil
}

// GetBlob retrieves a blob from the cache.
func (cm *Coordinator) GetBlob(
	ctx context.Context,
	digest string,
) (io.ReadCloser, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	reader, err := cm.blobCache.GetBlob(ctx, digest)
	if err != nil {
		cm.metrics.RecordMiss()
		return nil, fmt.Errorf("failed to get blob: %w", err)
	}

	cm.metrics.RecordHit()

	// Update access tracking
	if indexEntry, exists := cm.index.Get(digest); exists {
		cm.eviction.OnAccess(&Entry{
			Key:        digest,
			AccessedAt: time.Now(),
		})
		indexEntry.AccessedAt = time.Now()
		indexEntry.AccessCount++
	}

	return reader, nil
}

// PutBlob stores a blob in the cache.
func (cm *Coordinator) PutBlob(
	ctx context.Context,
	digest string,
	reader io.Reader,
) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if err := cm.blobCache.PutBlob(ctx, digest, reader); err != nil {
		cm.metrics.RecordError()
		return fmt.Errorf("failed to put blob: %w", err)
	}

	// Get blob size (this is approximate since we already consumed the reader)
	size := int64(len(digest)) * 10 // Rough estimate

	indexEntry := &IndexEntry{
		Key:         digest,
		Size:        size,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		TTL:         24 * time.Hour, // Blobs have longer TTL
		AccessCount: 1,
		FilePath:    "blobs/" + digest,
	}

	if err := cm.index.Put(digest, indexEntry); err != nil {
		cm.metrics.RecordError()
		return fmt.Errorf("failed to index blob: %w", err)
	}

	// Notify eviction strategy
	entry := &Entry{
		Key:        digest,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        24 * time.Hour,
	}
	cm.eviction.OnAdd(entry)

	return nil
}

// Size returns the total size of all cached entries.
func (cm *Coordinator) Size(ctx context.Context) (int64, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.index.Stats().TotalSize, nil
}

// Clear removes all entries from all caches.
func (cm *Coordinator) Clear(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Clear index - this will effectively clear everything since we rely on the index
	for _, key := range cm.index.Keys(nil) {
		if err := cm.index.Delete(key); err != nil {
			// Log error but continue clearing other entries
			continue
		}
	}

	return cm.index.Persist()
}

// deleteEntry removes an entry from the index.
// Note: Individual cache implementations handle their own cleanup.
func (cm *Coordinator) deleteEntry(ctx context.Context, key string) error {
	// Remove from index only - individual caches handle their own cleanup
	return cm.index.Delete(key)
}

// GetMetrics returns current cache metrics.
func (cm *Coordinator) GetMetrics() *Metrics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.metrics
}

// Config returns the cache configuration.
func (cm *Coordinator) Config() Config {
	return cm.config
}

// GetStats returns comprehensive cache statistics.
func (cm *Coordinator) GetStats() Stats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	indexStats := cm.index.Stats()

	return Stats{
		TotalEntries:       indexStats.TotalEntries,
		ExpiredEntries:     indexStats.ExpiredEntries,
		TotalSize:          indexStats.TotalSize,
		MaxSize:            cm.config.MaxSizeBytes,
		HitRate:            cm.metrics.HitRate(),
		Evictions:          cm.metrics.Evictions,
		Errors:             cm.metrics.Errors,
		LastCompaction:     indexStats.LastCompaction,
		AverageAccessCount: indexStats.AverageAccessCount,
	}
}

// Close shuts down the cache manager and cleans up resources.
func (cm *Coordinator) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.initialized {
		return nil
	}

	// Stop cleanup scheduler
	if cm.cleanupTicker != nil {
		// Signal the goroutine to stop
		select {
		case <-cm.cleanupDone:
			// Channel already closed
		default:
			close(cm.cleanupDone)
		}
		// Wait a bit for the goroutine to exit
		time.Sleep(10 * time.Millisecond)
		// Now it's safe to stop the ticker and set it to nil
		cm.cleanupTicker.Stop()
		cm.cleanupTicker = nil
	}

	// Mark as not initialized to prevent double-close
	// This is protected by the mutex from the beginning of Close()

	// Persist final state
	return cm.index.Persist()
}

// Stats provides comprehensive statistics about the cache state.
type Stats struct {
	TotalEntries       int       `json:"total_entries"`
	ExpiredEntries     int       `json:"expired_entries"`
	TotalSize          int64     `json:"total_size_bytes"`
	MaxSize            int64     `json:"max_size_bytes"`
	HitRate            float64   `json:"hit_rate"`
	Evictions          int64     `json:"total_evictions"`
	Errors             int64     `json:"total_errors"`
	LastCompaction     time.Time `json:"last_compaction"`
	AverageAccessCount float64   `json:"average_access_count"`
}

// tagResolver is a placeholder implementation of TagCache for the manager.
// TODO: Replace with proper implementation when TagResolver is available.
type tagResolver struct{}

func (tr *tagResolver) GetTagMapping(ctx context.Context, reference string) (*TagMapping, error) {
	return nil, fmt.Errorf("tag cache not implemented")
}

func (tr *tagResolver) PutTagMapping(ctx context.Context, reference, digest string) error {
	return fmt.Errorf("tag cache not implemented")
}

func (tr *tagResolver) HasTagMapping(ctx context.Context, reference string) (bool, error) {
	return false, nil
}

func (tr *tagResolver) DeleteTagMapping(ctx context.Context, reference string) error {
	return fmt.Errorf("tag cache not implemented")
}

func (tr *tagResolver) GetTagHistory(ctx context.Context, reference string) ([]TagHistoryEntry, error) {
	return nil, fmt.Errorf("tag cache not implemented")
}

func (tr *tagResolver) Clear(ctx context.Context) error {
	return nil
}
