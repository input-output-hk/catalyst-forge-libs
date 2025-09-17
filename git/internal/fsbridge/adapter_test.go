package fsbridge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/input-output-hk/catalyst-forge-libs/fs"
	"github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToBillyFilesystem(t *testing.T) {
	t.Run("success with billy.FS", func(t *testing.T) {
		// Create a billy.FS wrapping an in-memory filesystem
		memFS := memfs.New()
		billyFS := billy.NewFS(memFS)

		// Convert it back to billy.Filesystem
		result, err := ToBillyFilesystem(billyFS)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify it's the same underlying filesystem
		assert.Equal(t, memFS, result)
	})

	t.Run("error with non-billy.FS", func(t *testing.T) {
		// Create a mock filesystem that's not a billy.FS
		var mockFS fs.Filesystem = &mockFilesystem{}

		// Should return an error
		result, err := ToBillyFilesystem(mockFS)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "filesystem must be a billy.FS")
	})
}

// mockFilesystem is a minimal mock that satisfies fs.Filesystem but is not a billy.FS
type mockFilesystem struct{}

//nolint:ireturn // tests can return interfaces for mocks
func (m *mockFilesystem) Create(name string) (fs.File, error) { return nil, nil }

//nolint:ireturn // tests can return interfaces for mocks
func (m *mockFilesystem) Open(name string) (fs.File, error) { return nil, nil }

//nolint:ireturn // tests can return interfaces for mocks
func (m *mockFilesystem) OpenFile(name string, flag int, perm os.FileMode) (fs.File, error) {
	return nil, nil
}
func (m *mockFilesystem) ReadFile(name string) ([]byte, error)                       { return nil, nil }
func (m *mockFilesystem) WriteFile(name string, data []byte, perm os.FileMode) error { return nil }
func (m *mockFilesystem) Stat(name string) (os.FileInfo, error)                      { return nil, nil }
func (m *mockFilesystem) Rename(oldname, newname string) error                       { return nil }
func (m *mockFilesystem) Remove(name string) error                                   { return nil }
func (m *mockFilesystem) RemoveAll(path string) error                                { return nil }
func (m *mockFilesystem) ReadDir(name string) ([]os.FileInfo, error)                 { return nil, nil }
func (m *mockFilesystem) MkdirAll(path string, perm os.FileMode) error               { return nil }
func (m *mockFilesystem) Walk(root string, fn filepath.WalkFunc) error               { return nil }
func (m *mockFilesystem) TempDir(dir, pattern string) (string, error)                { return "", nil }
func (m *mockFilesystem) GetAbs(path string) (string, error)                         { return "", nil }
func (m *mockFilesystem) Exists(path string) (bool, error)                           { return false, nil }
func (m *mockFilesystem) Symlink(target, link string) error                          { return nil }
