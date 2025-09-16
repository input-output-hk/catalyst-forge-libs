// Package fsbridge provides adapters between fs.Filesystem and billy.Filesystem.
// This enables git operations to work with the project's native filesystem abstraction.
package fsbridge

import (
	"fmt"

	"github.com/go-git/go-billy/v5"
	"github.com/input-output-hk/catalyst-forge-libs/fs"
	fsb "github.com/input-output-hk/catalyst-forge-libs/fs/billy"
)

// ToBillyFilesystem converts an fs.Filesystem to a billy.Filesystem.
// The passed filesystem must be a billy.FS wrapper from the fs/billy package.
// If not, an error is returned.
//
//nolint:ireturn // returns interface as required by billy.Filesystem interface
func ToBillyFilesystem(fsys fs.Filesystem) (billy.Filesystem, error) {
	// Type assert to billy.FS which wraps a billy.Filesystem
	billyFS, ok := fsys.(*fsb.FS)
	if !ok {
		return nil, fmt.Errorf("filesystem must be a billy.FS from fs/billy package, got %T", fsys)
	}

	// Extract the underlying billy.Filesystem using Raw()
	return billyFS.Raw(), nil
}
