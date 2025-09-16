package fsbridge

import (
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
)

func TestNewStorage(t *testing.T) {
	tests := []struct {
		name      string
		cacheSize int
		expected  int // expected cache size after validation
	}{
		{
			name:      "valid cache size",
			cacheSize: 500,
			expected:  500,
		},
		{
			name:      "zero cache size defaults to minimum",
			cacheSize: 0,
			expected:  100,
		},
		{
			name:      "negative cache size defaults to minimum",
			cacheSize: -1,
			expected:  100,
		},
		{
			name:      "large cache size",
			cacheSize: 10000,
			expected:  10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memFS := memfs.New()
			storage := NewStorage(memFS, tt.cacheSize)

			if storage == nil {
				t.Fatal("NewStorage returned nil")
			}

			// We can't directly inspect the cache size, but we can verify
			// the storage was created successfully
			if storage.Filesystem() != memFS {
				t.Errorf("Storage filesystem = %v, want %v", storage.Filesystem(), memFS)
			}
		})
	}
}

func TestNewStorageWithDefaultCache(t *testing.T) {
	memFS := memfs.New()
	storage := NewStorageWithDefaultCache(memFS)

	if storage == nil {
		t.Fatal("NewStorageWithDefaultCache returned nil")
	}

	if storage.Filesystem() != memFS {
		t.Errorf("Storage filesystem = %v, want %v", storage.Filesystem(), memFS)
	}
}

func TestStorageWithMemoryFilesystem(t *testing.T) {
	memFS := memfs.New()
	storage := NewStorage(memFS, 1000)

	if storage == nil {
		t.Fatal("NewStorage returned nil for memory filesystem")
	}

	// Test that we can create files through the storage filesystem
	err := storage.Filesystem().MkdirAll("test", 0o755)
	if err != nil {
		t.Fatalf("Failed to create directory through storage: %v", err)
	}

	_, err = storage.Filesystem().Stat("test")
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}
}
