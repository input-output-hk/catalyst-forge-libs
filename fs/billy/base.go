package billy

import (
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
)

// BaseOSFS is a billy.Filesystem that acts like the native filesystem.
type BaseOSFS struct {
	osfs.ChrootOS
}

// Chroot returns a new filesystem rooted at the provided path.
//
//nolint:ireturn // billy.Filesystem is an interface; signature is dictated by upstream.
func (b *BaseOSFS) Chroot(path string) (billy.Filesystem, error) {
	return osfs.New(path), nil
}

// Root returns the root path for this filesystem.
func (b *BaseOSFS) Root() string {
	return "/"
}

// NewBaseOSFS creates a new OS filesystem that acts like the native filesystem.
func NewBaseOSFS() *FS {
	return &FS{
		fs: &BaseOSFS{},
	}
}

// Backward compatibility aliases.
//
//nolint:revive // Keep alias and constructor name for compatibility with previous API; stutter is intentional.
type BillyBaseOsFs = BaseOSFS

//nolint:revive // Keep exported wrapper to retain old constructor name.
func NewBaseOsFS() *FS { return NewBaseOSFS() }
