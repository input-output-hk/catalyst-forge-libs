package earthfile

import (
	"bytes"
	"fmt"
	"io"
)

// namedReader implements the ast.NamedReader interface required by the AST parser.
type namedReader struct {
	*bytes.Reader
	name string
}

// newNamedReader creates a new NamedReader from bytes and a name.
func newNamedReader(content []byte, name string) *namedReader {
	return &namedReader{
		Reader: bytes.NewReader(content),
		name:   name,
	}
}

// Name returns the name of the reader (typically the file path).
func (r *namedReader) Name() string {
	return r.name
}

// Seek implements io.Seeker for the NamedReader interface.
func (r *namedReader) Seek(offset int64, whence int) (int64, error) {
	pos, err := r.Reader.Seek(offset, whence)
	if err != nil {
		return 0, fmt.Errorf("seek error: %w", err)
	}
	return pos, nil
}

// Read implements io.Reader for the NamedReader interface.
func (r *namedReader) Read(buff []byte) (n int, err error) {
	n, err = r.Reader.Read(buff)
	if err != nil && err != io.EOF {
		return n, fmt.Errorf("read error: %w", err)
	}
	// Return io.EOF directly without wrapping (expected behavior)
	return n, err //nolint:wrapcheck // io.EOF should be returned unwrapped
}

// Interface assertions to ensure we implement the required interfaces.
var (
	_ io.Reader = (*namedReader)(nil)
	_ io.Seeker = (*namedReader)(nil)
)
