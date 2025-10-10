package minio

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/input-output-hk/catalyst-forge-libs/fs/core"
	"github.com/minio/minio-go/v7"
)

// File represents a MinIO object handle.
// Behavior differs based on open mode (read vs write).
type File struct {
	fs   *MinioFS
	key  string // Full S3 key (including prefix)
	name string // Original name provided to Open/Create
	mode int    // Open flags (O_RDONLY, O_WRONLY, etc.)

	// Read mode fields
	// reader wraps downloaded object data. We use interface{} to hold a type
	// that implements both io.ReadSeeker and io.ReaderAt (like *bytes.Reader).
	reader interface {
		io.ReadSeeker
		io.ReaderAt
	}
	size    int64     // Object size
	modTime time.Time // Last modified time

	// Write mode fields
	buffer *bytes.Buffer // Accumulates writes
	closed bool          // Prevent double-close
}

// newFileRead creates a File in read mode by downloading the object.
func newFileRead(ctx context.Context, mfs *MinioFS, key, name string) (*File, error) {
	// Download the object
	obj, err := mfs.client.GetObject(ctx, mfs.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, translateError(err)
	}
	defer func() {
		_ = obj.Close()
	}()

	// Read the entire object into memory
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, translateError(err)
	}

	// Get object info for metadata
	stat, err := obj.Stat()
	if err != nil {
		return nil, translateError(err)
	}

	return &File{
		fs:      mfs,
		key:     key,
		name:    name,
		mode:    os.O_RDONLY,
		reader:  bytes.NewReader(data),
		size:    stat.Size,
		modTime: stat.LastModified,
	}, nil
}

// newFileWrite creates a File in write mode with an empty buffer.
func newFileWrite(mfs *MinioFS, key, name string, flag int) *File {
	return &File{
		fs:     mfs,
		key:    key,
		name:   name,
		mode:   flag,
		buffer: new(bytes.Buffer),
		closed: false,
	}
}

// Read reads up to len(p) bytes into p. It returns the number of bytes read
// and any error encountered. At end of file, Read returns 0, io.EOF.
// Read is only supported in read mode (O_RDONLY).
func (f *File) Read(p []byte) (int, error) {
	if f.mode&os.O_WRONLY != 0 {
		return 0, &fs.PathError{Op: "read", Path: f.name, Err: fs.ErrInvalid}
	}
	if f.reader == nil {
		return 0, &fs.PathError{Op: "read", Path: f.name, Err: fs.ErrInvalid}
	}
	return f.reader.Read(p)
}

// Write writes len(p) bytes from p to the underlying data stream.
// It returns the number of bytes written and any error encountered.
// Write is only supported in write mode (O_WRONLY, O_CREATE).
func (f *File) Write(p []byte) (int, error) {
	if f.closed {
		return 0, &fs.PathError{Op: "write", Path: f.name, Err: fs.ErrClosed}
	}
	if f.mode&(os.O_WRONLY|os.O_RDWR) == 0 {
		return 0, &fs.PathError{Op: "write", Path: f.name, Err: fs.ErrInvalid}
	}
	if f.buffer == nil {
		return 0, &fs.PathError{Op: "write", Path: f.name, Err: fs.ErrInvalid}
	}
	return f.buffer.Write(p)
}

// Seek sets the offset for the next Read operation. It returns the new offset
// and an error, if any. Seek is only supported in read mode.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.mode&os.O_WRONLY != 0 {
		return 0, &fs.PathError{Op: "seek", Path: f.name, Err: core.ErrUnsupported}
	}
	if f.reader == nil {
		return 0, &fs.PathError{Op: "seek", Path: f.name, Err: fs.ErrInvalid}
	}
	return f.reader.Seek(offset, whence)
}

// ReadAt reads len(p) bytes from the File starting at byte offset off.
// It returns the number of bytes read and any error encountered.
// ReadAt is only supported in read mode.
func (f *File) ReadAt(p []byte, off int64) (int, error) {
	if f.mode&os.O_WRONLY != 0 {
		return 0, &fs.PathError{Op: "readat", Path: f.name, Err: core.ErrUnsupported}
	}
	if f.reader == nil {
		return 0, &fs.PathError{Op: "readat", Path: f.name, Err: fs.ErrInvalid}
	}
	return f.reader.ReadAt(p, off)
}

// Stat returns the FileInfo structure describing the file.
// In read mode, returns the size and modTime from the downloaded object.
// In write mode, returns the current buffer size and current time.
func (f *File) Stat() (fs.FileInfo, error) {
	if f.mode&os.O_WRONLY != 0 {
		// Write mode: return current buffer size
		return &fileInfo{
			name:    f.name,
			size:    int64(f.buffer.Len()),
			modTime: time.Now(),
			mode:    0644,
		}, nil
	}
	// Read mode: return downloaded object info
	return &fileInfo{
		name:    f.name,
		size:    f.size,
		modTime: f.modTime,
		mode:    0644,
	}, nil
}

// Close closes the file, releasing any resources.
// In write mode, Close uploads the buffer contents to S3.
// In read mode, Close is a no-op.
func (f *File) Close() error {
	if f.closed {
		return nil // Already closed, idempotent
	}
	f.closed = true

	// If in write mode, upload the buffer
	if f.mode&(os.O_WRONLY|os.O_RDWR) != 0 && f.buffer != nil {
		return f.sync(context.Background())
	}

	return nil
}

// Sync commits the current contents of the file to S3 storage.
// In write mode, uploads the buffer contents via PutObject.
// In read mode, Sync is a no-op.
// Sync can be called multiple times (idempotent).
func (f *File) Sync() error {
	if f.mode&(os.O_WRONLY|os.O_RDWR) != 0 && f.buffer != nil {
		return f.sync(context.Background())
	}
	return nil
}

// sync is the internal implementation that performs the actual upload.
func (f *File) sync(ctx context.Context) error {

	// Upload the buffer contents
	reader := bytes.NewReader(f.buffer.Bytes())
	_, err := f.fs.client.PutObject(
		ctx,
		f.fs.bucket,
		f.key,
		reader,
		int64(f.buffer.Len()),
		minio.PutObjectOptions{
			ContentType: "application/octet-stream",
		},
	)
	if err != nil {
		return translateError(err)
	}

	return nil
}

// Name returns the name of the file as provided to Open or Create.
func (f *File) Name() string {
	return f.name
}

// fileInfo implements fs.FileInfo for MinIO objects.
type fileInfo struct {
	name    string
	size    int64
	modTime time.Time
	mode    fs.FileMode
}

func (fi *fileInfo) Name() string       { return fi.name }
func (fi *fileInfo) Size() int64        { return fi.size }
func (fi *fileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi *fileInfo) ModTime() time.Time { return fi.modTime }
func (fi *fileInfo) IsDir() bool        { return fi.mode&fs.ModeDir != 0 }
func (fi *fileInfo) Sys() interface{}   { return nil }

// Compile-time interface checks.
var (
	_ core.File   = (*File)(nil)
	_ fs.File     = (*File)(nil)
	_ io.Seeker   = (*File)(nil)
	_ io.ReaderAt = (*File)(nil)
	_ core.Syncer = (*File)(nil)
)
