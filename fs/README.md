## fs — Filesystem Abstractions and Utilities

The `fs` package provides a small, focused abstraction over filesystem operations, plus practical helpers for common tasks. It includes:

- `Filesystem` interface: minimal, OS-like filesystem surface
- `File` interface: basic file handle operations
- Utilities: `GetAbs`, `Exists`
- `billy` implementation: an adapter backed by [`go-billy`](https://github.com/go-git/go-billy) supporting in-memory and OS filesystems

### Installation

Use Go modules to add the package to your project:

```bash
go get github.com/input-output-hk/catalyst-forge/lib/tools/fs
```

Billy-backed implementation:

```bash
go get github.com/input-output-hk/catalyst-forge/lib/tools/fs/billy
```

### Interfaces

```go
// Filesystem defines an abstract filesystem interface for common file and
// directory operations. Implementations should follow OS filesystem semantics.
type Filesystem interface {
    Create(name string) (File, error)
    Exists(path string) (bool, error)
    MkdirAll(path string, perm os.FileMode) error
    Open(name string) (File, error)
    OpenFile(name string, flag int, perm os.FileMode) (File, error)
    ReadDir(dirname string) ([]os.FileInfo, error)
    ReadFile(path string) ([]byte, error)
    Remove(name string) error
    Stat(name string) (os.FileInfo, error)
    TempDir(dir string, prefix string) (name string, err error)
    Walk(root string, walkFn filepath.WalkFunc) error
    WriteFile(filename string, data []byte, perm os.FileMode) error
}

// File represents an open file handle supporting basic I/O operations.
type File interface {
    Close() error
    Name() string
    Read(p []byte) (n int, err error)
    ReadAt(p []byte, off int64) (n int, err error)
    Seek(offset int64, whence int) (int64, error)
    Stat() (fs.FileInfo, error)
    Write(p []byte) (n int, err error)
}
```

### Utilities

```go
// GetAbs returns the absolute path of a given path.
func GetAbs(path string) (string, error)

// Exists checks if a given path exists.
func Exists(path string) (bool, error)
```

### Implementations (billy)

The `billy` subpackage adapts `go-billy` to the `Filesystem` interface.

- `billy.NewFS(b billy.Filesystem) *billy.FS`: wrap an existing go-billy filesystem
- `billy.NewInMemoryFS() *billy.FS`: in-memory filesystem
- `billy.NewOSFS(path string) *billy.FS`: OS-backed filesystem rooted at `path`
- `billy.BaseOSFS`: `go-billy` `ChrootOS`-compatible base that behaves like native OS FS

Backward compatibility aliases are provided for earlier names: `NewFs`, `NewInMemoryFs`, `NewOsFs`, and `BillyFs`.

### Quick Start

Create and write a file using an in-memory filesystem:

```go
package main

import (
    stdfs "io/fs"
    "log"
    "os"
    "path/filepath"

    cffs "github.com/input-output-hk/catalyst-forge/lib/tools/fs"
    billyfs "github.com/input-output-hk/catalyst-forge/lib/tools/fs/billy"
)

func main() {
    // Choose an implementation
    mem := billyfs.NewInMemoryFS()

    // Ensure a directory exists
    if err := mem.MkdirAll("/data", 0o755); err != nil {
        log.Fatal(err)
    }

    // Create and write a file
    if err := mem.WriteFile("/data/hello.txt", []byte("hello"), 0o644); err != nil {
        log.Fatal(err)
    }

    // Read it back
    b, err := mem.ReadFile("/data/hello.txt")
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("content=%q", string(b))

    // Walk directory
    _ = mem.Walk("/data", func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        log.Printf("%s (%d bytes)", path, info.Size())
        return nil
    })

    // Utilities
    abs, err := cffs.GetAbs("./relative/path")
    if err != nil { log.Fatal(err) }
    log.Printf("abs=%s", abs)

    exists, err := cffs.Exists(abs)
    if err != nil { log.Fatal(err) }
    log.Printf("exists=%v", exists)
}
```

### Using an OS-backed filesystem

```go
root := "/tmp/my-root"
fs := billyfs.NewOSFS(root)
if err := fs.MkdirAll("logs", 0o755); err != nil { /* handle */ }
```
