package billy

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/go-git/go-billy/v5"
)

// File wraps a go-billy File and satisfies the parent fs.File interface.
type File struct {
	file billy.File
	fs   *FS
}

// Close implements File.Close.
func (f *File) Close() error {
	if err := f.file.Close(); err != nil {
		return fmt.Errorf("billy: close %q: %w", f.file.Name(), err)
	}
	return nil
}

// Name implements File.Name.
func (f *File) Name() string {
	return f.file.Name()
}

// Read implements File.Read.
func (f *File) Read(p []byte) (n int, err error) {
	n, err = f.file.Read(p)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return n, io.EOF
		}
		return n, fmt.Errorf("billy: read %q: %w", f.file.Name(), err)
	}
	return n, nil
}

// ReadAt implements File.ReadAt.
func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	n, err = f.file.ReadAt(p, off)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return n, io.EOF
		}
		return n, fmt.Errorf("billy: readat %q off=%d: %w", f.file.Name(), off, err)
	}
	return n, nil
}

// Seek implements File.Seek.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	pos, err := f.file.Seek(offset, whence)
	if err != nil {
		return pos, fmt.Errorf("billy: seek %q off=%d whence=%d: %w", f.file.Name(), offset, whence, err)
	}
	return pos, nil
}

// Stat implements File.Stat.
func (f *File) Stat() (fs.FileInfo, error) {
	info, err := f.fs.Stat(f.file.Name())
	if err != nil {
		return nil, fmt.Errorf("billy: stat %q: %w", f.file.Name(), err)
	}
	return info, nil
}

// Write implements File.Write.
func (f *File) Write(p []byte) (n int, err error) {
	n, err = f.file.Write(p)
	if err != nil {
		return n, fmt.Errorf("billy: write %q: %w", f.file.Name(), err)
	}
	return n, nil
}
