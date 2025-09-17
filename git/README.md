# Git Library for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/input-output-hk/catalyst-forge-libs/git.svg)](https://pkg.go.dev/github.com/input-output-hk/catalyst-forge-libs/git)
[![Go Report Card](https://goreportcard.com/badge/github.com/input-output-hk/catalyst-forge-libs/git)](https://goreportcard.com/report/github.com/input-output-hk/catalyst-forge-libs/git)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A high-level, idiomatic Go wrapper for Git operations built on [go-git](https://github.com/go-git/go-git). This library provides a clean, task-oriented API for common Git workflows while enforcing filesystem abstraction for both on-disk and in-memory repositories.

## Features

- 🎯 **Simple, task-oriented API** - Focus on what you want to do, not how Git does it
- 🧪 **Testable by design** - Full in-memory filesystem support for testing
- 🔒 **Secure defaults** - Built-in authentication, host verification, and credential safety
- ⚡ **Performance optimized** - LRU caching, shallow operations, and efficient diffs
- 🔧 **Go idiomatic** - Accepts interfaces, returns concrete types, follows Go patterns
- ⏱️ **Context support** - All operations support timeouts and cancellation
- 📦 **Zero external dependencies** - Only depends on go-git and the filesystem abstraction

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [API Documentation](#api-documentation)
  - [Repository Operations](#repository-operations)
  - [Branch Management](#branch-management)
  - [Staging and Commits](#staging-and-commits)
  - [Synchronization](#synchronization)
  - [Tags](#tags)
  - [History and Diffs](#history-and-diffs)
  - [References](#references)
- [Authentication](#authentication)
- [In-Memory Operations](#in-memory-operations)
- [Error Handling](#error-handling)
- [Examples](#examples)
- [Testing](#testing)
- [Performance](#performance)
- [Contributing](#contributing)
- [License](#license)

## Installation

```bash
go get github.com/input-output-hk/catalyst-forge-libs/git
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    billyfs "github.com/input-output-hk/catalyst-forge-libs/fs/billy"
    "github.com/input-output-hk/catalyst-forge-libs/git"
)

func main() {
    // Create filesystem
    fs := billyfs.NewOSFS("./my-repo")

    // Initialize a new repository
    repo, err := git.Init(context.Background(), &git.Options{
        FS:      fs,
        Workdir: ".",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create and commit a file
    err = fs.WriteFile("README.md", []byte("# My Project\n"), 0644)
    if err != nil {
        log.Fatal(err)
    }

    // Stage the file
    err = repo.Add(context.Background(), "README.md")
    if err != nil {
        log.Fatal(err)
    }

    // Commit
    sha, err := repo.Commit(context.Background(), "Initial commit", git.Signature{
        Name:  "John Doe",
        Email: "john@example.com",
    }, git.CommitOpts{})
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Created commit: %s\n", sha)
}
```

## Core Concepts

### Filesystem Abstraction

This library **requires** the use of the project's filesystem abstraction (`fs.Filesystem`). This design enables:
- Complete testability with in-memory repositories
- Consistent I/O patterns across the codebase
- Easy mocking and testing without touching disk
- Sandboxed operations for security

### Options Pattern

All repository operations start with an `Options` struct that configures the repository:

```go
type Options struct {
    FS              fs.Filesystem  // Required: filesystem to use
    Workdir         string         // Working directory (default: ".")
    Bare            bool           // Create/use bare repository
    StorerCacheSize int            // LRU cache size (default: 1000)
    Auth            AuthProvider   // Authentication provider
    HTTPClient      *http.Client   // Custom HTTP client
    ShallowDepth    int            // Shallow clone depth (0 = full)
}
```

## API Documentation

### Repository Operations

#### Initialize a Repository

```go
repo, err := git.Init(ctx, &git.Options{
    FS:      fs,
    Workdir: ".",
    Bare:    false,  // Set true for bare repository
})
```

#### Open Existing Repository

```go
repo, err := git.Open(ctx, &git.Options{
    FS:      fs,
    Workdir: ".",
})
```

#### Clone Repository

```go
repo, err := git.Clone(ctx, "https://github.com/user/repo.git", &git.Options{
    FS:           fs,
    Workdir:      ".",
    ShallowDepth: 1,    // Shallow clone with depth 1
    Auth:         auth,  // Optional authentication
})
```

### Branch Management

#### Create and Switch Branches

```go
// Get current branch
branch, err := repo.CurrentBranch(ctx)

// Create new branch
err = repo.CreateBranch(ctx, "feature/new", "HEAD", false, false)

// Checkout branch
err = repo.CheckoutBranch(ctx, "feature/new", false, false)

// Create and checkout in one step
err = repo.CheckoutBranch(ctx, "feature/another", true, false)

// Delete branch
err = repo.DeleteBranch(ctx, "old-feature")

// Checkout remote branch
err = repo.CheckoutRemoteBranch(ctx, "origin", "main", "main", true)
```

### Staging and Commits

#### Working with Files

```go
// Stage files
err = repo.Add(ctx, "file1.go", "file2.go")

// Stage all changes
err = repo.Add(ctx, ".")

// Unstage files
err = repo.Unstage(ctx, "file1.go")

// Remove files
err = repo.Remove(ctx, "deleted.go")
```

#### Creating Commits

```go
sha, err := repo.Commit(ctx, "feat: add new feature", git.Signature{
    Name:  "Jane Doe",
    Email: "jane@example.com",
}, git.CommitOpts{
    AllowEmpty: false,  // Prevent empty commits
})
```

### Synchronization

#### Fetch, Pull, and Push

```go
// Fetch from remote
err = repo.Fetch(ctx, "origin", true, 0)  // true = prune stale branches

// Pull with fast-forward only
err = repo.PullFFOnly(ctx, "origin")

// Fetch and merge
err = repo.FetchAndMerge(ctx, "origin", "main", git.FastForwardOnly)

// Push current branch
err = repo.Push(ctx, "origin", false)  // false = no force push
```

### Tags

#### Tag Management

```go
// Create annotated tag
err = repo.CreateTag(ctx, "v1.0.0", "HEAD", "Release v1.0.0", true)

// Create lightweight tag
err = repo.CreateTag(ctx, "latest", "HEAD", "", false)

// List tags
tags, err := repo.Tags(ctx)

// List tags with filter
tags, err := repo.Tags(ctx, git.TagPatternFilter("v*"))

// Delete tag
err = repo.DeleteTag(ctx, "old-tag")
```

### History and Diffs

#### Query Commit History

```go
// Get commit log with filters
iter, err := repo.Log(ctx, git.LogFilter{
    Since:    &sinceTime,
    Until:    &untilTime,
    Author:   "Jane",
    Path:     []string{"src/"},
    MaxCount: 100,
})
defer iter.Close()

// Iterate commits
err = iter.ForEach(func(c *object.Commit) error {
    fmt.Printf("%s: %s\n", c.Hash, c.Message)
    return nil
})
```

#### Compute Diffs

```go
// Diff between commits
diff, err := repo.Diff(ctx, "HEAD~1", "HEAD", nil)

// Diff with filters
diff, err := repo.Diff(ctx, "v1.0.0", "v2.0.0",
    git.ExtensionFilter(".go", ".mod"),
    git.PathPrefixFilter("src/"),
)

fmt.Printf("Files changed: %d\n", diff.FileCount)
fmt.Println(diff.Text)
```

### References

#### Working with References

```go
// List references by type
branches, err := repo.Refs(ctx, git.RefBranch, "")
tags, err := repo.Refs(ctx, git.RefTag, "")
remotes, err := repo.Refs(ctx, git.RefRemoteBranch, "")

// Resolve any reference
resolved, err := repo.Resolve(ctx, "HEAD~2")
fmt.Printf("Type: %s, Hash: %s\n", resolved.Kind, resolved.Hash)
```

## Authentication

The library supports authentication through the `AuthProvider` interface. You can implement custom providers or use the built-in ones:

```go
// Custom auth provider
type TokenAuth struct {
    token string
}

func (a *TokenAuth) Method(ctx context.Context, url string) (transport.AuthMethod, error) {
    return &http.BasicAuth{
        Username: "token",
        Password: a.token,
    }, nil
}

// Use with repository
repo, err := git.Clone(ctx, remoteURL, &git.Options{
    FS:   fs,
    Auth: &TokenAuth{token: "github_pat_..."},
})
```

## In-Memory Operations

Perfect for testing and temporary operations:

```go
// Create in-memory filesystem
memFS := billyfs.NewInMemoryFS()

// All operations work exactly the same
repo, err := git.Init(ctx, &git.Options{
    FS:      memFS,
    Workdir: "/",
})

// Create files in memory
err = memFS.WriteFile("test.txt", []byte("content"), 0644)
err = repo.Add(ctx, "test.txt")
sha, err := repo.Commit(ctx, "test", sig, git.CommitOpts{})
```

## Error Handling

The library provides sentinel errors for common conditions:

```go
import "errors"

err := repo.Push(ctx, "origin", false)
switch {
case errors.Is(err, git.ErrNotFastForward):
    // Handle non-fast-forward
case errors.Is(err, git.ErrAuthRequired):
    // Handle missing auth
case errors.Is(err, git.ErrAlreadyUpToDate):
    // Nothing to do
default:
    // Handle other errors
}
```

Available sentinel errors:
- `ErrAlreadyUpToDate` - No changes to fetch/pull/push
- `ErrAuthRequired` - Authentication needed but not provided
- `ErrAuthFailed` - Authentication failed
- `ErrBranchExists` - Branch already exists
- `ErrBranchMissing` - Branch not found
- `ErrTagExists` - Tag already exists
- `ErrTagMissing` - Tag not found
- `ErrNotFastForward` - Merge would not be fast-forward
- `ErrMergeConflict` - Merge has conflicts
- `ErrInvalidRef` - Invalid reference format
- `ErrResolveFailed` - Cannot resolve reference

## Examples

### Complete Workflow Example

```go
func completeWorkflow(ctx context.Context) error {
    // Initialize repository
    fs := billyfs.NewOSFS("./project")
    repo, err := git.Init(ctx, &git.Options{FS: fs})
    if err != nil {
        return err
    }

    // Create initial structure
    files := map[string]string{
        "README.md":   "# Project\n",
        "main.go":     "package main\n",
        ".gitignore":  "*.tmp\n",
    }

    for path, content := range files {
        if err := fs.WriteFile(path, []byte(content), 0644); err != nil {
            return err
        }
    }

    // Initial commit
    if err := repo.Add(ctx, "."); err != nil {
        return err
    }

    sha, err := repo.Commit(ctx, "Initial commit", git.Signature{
        Name:  "Bot",
        Email: "bot@example.com",
    }, git.CommitOpts{})
    if err != nil {
        return err
    }

    // Create feature branch
    if err := repo.CreateBranch(ctx, "feature/awesome", sha, false, false); err != nil {
        return err
    }

    if err := repo.CheckoutBranch(ctx, "feature/awesome", false, false); err != nil {
        return err
    }

    // Make changes on feature branch
    if err := fs.WriteFile("feature.go", []byte("package main\n"), 0644); err != nil {
        return err
    }

    if err := repo.Add(ctx, "feature.go"); err != nil {
        return err
    }

    _, err = repo.Commit(ctx, "Add feature", git.Signature{
        Name:  "Developer",
        Email: "dev@example.com",
    }, git.CommitOpts{})

    return err
}
```

### More Examples

See the [example_test.go](example_test.go) file for more comprehensive examples including:
- Repository initialization and cloning
- Branch operations
- Commit workflows
- Tag management
- History traversal
- Diff computation
- Remote synchronization

## Testing

The library is designed for testability:

```go
func TestMyGitOperation(t *testing.T) {
    // Use in-memory filesystem for fast, isolated tests
    fs := billyfs.NewInMemoryFS()

    repo, err := git.Init(context.Background(), &git.Options{
        FS:      fs,
        Workdir: "/",
    })
    require.NoError(t, err)

    // Test your git operations without touching disk
    err = fs.WriteFile("test.txt", []byte("test"), 0644)
    require.NoError(t, err)

    err = repo.Add(context.Background(), "test.txt")
    require.NoError(t, err)

    // Assertions...
}
```

## Performance

### Optimizations

- **LRU Object Cache**: Configure `StorerCacheSize` in Options (default: 1000)
- **Shallow Operations**: Use `ShallowDepth` for faster clones of large repositories
- **Selective Fetching**: Fetch only needed branches with pruning
- **Filtered Diffs**: Use path filters to compute diffs only for relevant files

### Benchmarks

```go
// Shallow clone for large repositories
repo, err := git.Clone(ctx, url, &git.Options{
    FS:           fs,
    ShallowDepth: 1,  // Only fetch latest commit
})

// Filtered diff for performance
diff, err := repo.Diff(ctx, "HEAD~100", "HEAD",
    git.PathPrefixFilter("src/"),  // Only diff src/ directory
    git.ExtensionFilter(".go"),    // Only Go files
)
```

## Thread Safety

- **Read operations** are safe for concurrent use (Log, Diff, Refs, CurrentBranch)
- **Write operations** must be serialized (Add, Commit, Push, etc.)
- Each `Repo` instance maintains its own state

## Limitations

This library intentionally does not support:
- Interactive operations (`rebase -i`, `add -i`)
- Complex merge conflict resolution
- Submodule management (planned for future)
- Direct git CLI invocation

For advanced use cases not covered by this facade, you can access the underlying go-git repository, though this is discouraged for maintainability.

## Contributing

Contributions are welcome! Please ensure:

1. All code follows the [Go Coding Standards](../docs/guides/go/style.md)
2. Tests are written for new functionality (test-first development)
3. Documentation is updated for API changes
4. `golangci-lint` passes with zero errors
5. Maintain backward compatibility

### Development Setup

```bash
# Clone the repository
git clone https://github.com/input-output-hk/catalyst-forge-libs
cd catalyst-forge-libs/git

# Install dependencies
go mod download

# Run tests
go test -v -race ./...

# Run linter
golangci-lint run ./...

# Check coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](../LICENSE) file for details.

## Acknowledgments

- Built on top of the excellent [go-git](https://github.com/go-git/go-git) library
- Filesystem abstraction from [catalyst-forge-libs/fs](https://github.com/input-output-hk/catalyst-forge-libs/tree/main/fs)

## Support

For issues, questions, or contributions, please visit:
- [GitHub Issues](https://github.com/input-output-hk/catalyst-forge-libs/issues)
- [Discussions](https://github.com/input-output-hk/catalyst-forge-libs/discussions)