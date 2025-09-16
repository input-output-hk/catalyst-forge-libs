// Package fsbridge provides adapters between fs.Filesystem and billy.Filesystem.
// This enables git operations to work with the project's native filesystem abstraction.
package fsbridge

import (
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// NewStorage creates a new git storage with LRU cache for object storage.
//
// The LRU cache improves performance by keeping frequently accessed objects in memory.
// The cache size is configurable and defaults to a reasonable value for most use cases.
func NewStorage(billyFS billy.Filesystem, cacheSize int) *filesystem.Storage {
	if cacheSize <= 0 {
		// Use a minimal cache size if invalid value provided
		cacheSize = 100
	}

	objCache := cache.NewObjectLRU(cache.FileSize(cacheSize))
	return filesystem.NewStorage(billyFS, objCache)
}

// NewStorageWithDefaultCache creates a new git storage with default LRU cache size.
// The default cache size is optimized for typical git operations.
func NewStorageWithDefaultCache(billyFS billy.Filesystem) *filesystem.Storage {
	return NewStorage(billyFS, 1000)
}
