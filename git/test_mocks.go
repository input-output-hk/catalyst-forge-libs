package git

import (
	"os"
	"path/filepath"

	gitfs "github.com/input-output-hk/catalyst-forge-libs/fs"
)

// mockFilesystem implements fs.Filesystem for testing.
// This mock provides a minimal implementation for testing Options validation.
type mockFilesystem struct{}

//nolint:ireturn // mock implementations return interfaces
func (m *mockFilesystem) Create(name string) (gitfs.File, error) {
	return nil, nil
}

func (m *mockFilesystem) Exists(path string) (bool, error) {
	return true, nil
}

func (m *mockFilesystem) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

//nolint:ireturn // mock implementations return interfaces
func (m *mockFilesystem) Open(name string) (gitfs.File, error) {
	return nil, nil
}

//nolint:ireturn // mock implementations return interfaces
func (m *mockFilesystem) OpenFile(name string, flag int, perm os.FileMode) (gitfs.File, error) {
	return nil, nil
}

func (m *mockFilesystem) ReadDir(dirname string) ([]os.FileInfo, error) {
	return nil, nil
}

func (m *mockFilesystem) ReadFile(path string) ([]byte, error) {
	return nil, nil
}

func (m *mockFilesystem) Remove(name string) error {
	return nil
}

func (m *mockFilesystem) Rename(oldpath, newpath string) error {
	return nil
}

func (m *mockFilesystem) Stat(name string) (os.FileInfo, error) {
	return nil, nil
}

func (m *mockFilesystem) TempDir(dir, prefix string) (string, error) {
	return "", nil
}

func (m *mockFilesystem) Walk(root string, walkFn filepath.WalkFunc) error {
	return nil
}

func (m *mockFilesystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return nil
}

func (m *mockFilesystem) Symlink(oldname, newname string) error {
	return nil
}
