package billy

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"

	parentfs "github.com/input-output-hk/catalyst-forge-libs/fs"
)

// FS implements the Filesystem interface using go-billy.
type FS struct {
	fs billy.Filesystem
}

// BillyFs is an alias for FS for backward compatibility.
//
//nolint:revive // public alias name kept for compatibility with older imports.
type BillyFs = FS

// Create implements Filesystem.Create.
//
//nolint:ireturn // API returns the fs.File interface by design for flexibility.
func (b *FS) Create(name string) (parentfs.File, error) {
	f, err := b.fs.Create(name)
	if err != nil {
		return nil, fmt.Errorf("billy: create %q: %w", name, err)
	}
	return &File{
		file: f,
		fs:   b,
	}, nil
}

// Exists implements Filesystem.Exists.
func (b *FS) Exists(path string) (bool, error) {
	_, err := b.fs.Stat(path)
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, fmt.Errorf("billy: stat %q: %w", path, err)
	}
}

// MkdirAll implements Filesystem.MkdirAll.
func (b *FS) MkdirAll(path string, perm os.FileMode) error {
	if err := b.fs.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("billy: mkdirall %q: %w", path, err)
	}
	return nil
}

// Open implements Filesystem.Open.
//
//nolint:ireturn // API returns the fs.File interface by design for flexibility.
func (b *FS) Open(name string) (parentfs.File, error) {
	f, err := b.fs.Open(name)
	if err != nil {
		return nil, fmt.Errorf("billy: open %q: %w", name, err)
	}
	return &File{
		file: f,
		fs:   b,
	}, nil
}

// OpenFile implements Filesystem.OpenFile.
//
//nolint:ireturn // API returns the fs.File interface by design for flexibility.
func (b *FS) OpenFile(name string, flag int, perm os.FileMode) (parentfs.File, error) {
	f, err := b.fs.OpenFile(name, flag, perm)
	if err != nil {
		return nil, fmt.Errorf("billy: openfile %q: %w", name, err)
	}
	return &File{
		file: f,
		fs:   b,
	}, nil
}

// ReadDir implements Filesystem.ReadDir.
func (b *FS) ReadDir(dirname string) ([]os.FileInfo, error) {
	list, err := b.fs.ReadDir(dirname)
	if err != nil {
		return nil, fmt.Errorf("billy: readdir %q: %w", dirname, err)
	}
	return list, nil
}

// ReadFile implements Filesystem.ReadFile.
func (b *FS) ReadFile(path string) ([]byte, error) {
	bts, err := util.ReadFile(b.fs, path)
	if err != nil {
		return nil, fmt.Errorf("billy: readfile %q: %w", path, err)
	}
	return bts, nil
}

// Remove implements Filesystem.Remove.
func (b *FS) Remove(name string) error {
	if err := b.fs.Remove(name); err != nil {
		return fmt.Errorf("billy: remove %q: %w", name, err)
	}
	return nil
}

// Stat implements Filesystem.Stat.
func (b *FS) Stat(name string) (os.FileInfo, error) {
	info, err := b.fs.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("billy: stat %q: %w", name, err)
	}
	return info, nil
}

// TempDir implements Filesystem.TempDir.
func (b *FS) TempDir(dir, prefix string) (name string, err error) {
	name, err = util.TempDir(b.fs, dir, prefix)
	if err != nil {
		return "", fmt.Errorf("billy: tempdir dir=%q prefix=%q: %w", dir, prefix, err)
	}
	return name, nil
}

// Walk implements Filesystem.Walk.
func (b *FS) Walk(root string, walkFn filepath.WalkFunc) error {
	if err := util.Walk(b.fs, root, walkFn); err != nil {
		return fmt.Errorf("billy: walk %q: %w", root, err)
	}
	return nil
}

// WriteFile implements Filesystem.WriteFile.
func (b *FS) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if err := util.WriteFile(b.fs, filename, data, perm); err != nil {
		return fmt.Errorf("billy: writefile %q: %w", filename, err)
	}
	return nil
}

// Raw returns the underlying go-billy filesystem.
//
//nolint:ireturn // returning interface here is intentional to expose the adapter target.
func (b *FS) Raw() billy.Filesystem {
	return b.fs
}

// NewFS creates a new FS using the given go-billy filesystem.
func NewFS(fsys billy.Filesystem) *FS {
	return &FS{
		fs: fsys,
	}
}

// NewFs is kept for backward compatibility. Prefer NewFS.
func NewFs(fsys billy.Filesystem) *FS { return NewFS(fsys) }

// NewInMemoryFS creates a new in-memory filesystem.
func NewInMemoryFS() *FS {
	return &FS{
		fs: memfs.New(),
	}
}

// NewInMemoryFs is kept for backward compatibility. Prefer NewInMemoryFS.
func NewInMemoryFs() *FS { return NewInMemoryFS() }

// NewOSFS creates a new OS filesystem.
func NewOSFS(path string) *FS {
	return &FS{
		fs: osfs.New(path),
	}
}

// NewOsFs is kept for backward compatibility. Prefer NewOSFS.
func NewOsFs(path string) *FS { return NewOSFS(path) }
