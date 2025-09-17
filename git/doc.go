// Package git provides a high-level, idiomatic Go wrapper for git operations.
//
// This package offers a clean facade over go-git, exposing task-oriented operations
// for common git workflows while enforcing the use of the project's native filesystem
// abstraction. All operations work with both on-disk and in-memory repositories.
//
// # Design Principles
//
// The package follows these core principles:
//   - Minimal surface area - easy to learn and extend
//   - Testability by construction - in-memory FS, controlled side effects
//   - Security & performance - context timeouts, auth integration, object caching
//   - Go idioms - accepts interfaces, returns concrete types
//
// # Basic Usage
//
// Initialize or open a repository:
//
//	import (
//	    "context"
//	    billyfs "github.com/input-output-hk/catalyst-forge-libs/fs/billy"
//	    "github.com/input-output-hk/catalyst-forge-libs/git"
//	)
//
//	// Create filesystem (can be OS-backed or in-memory)
//	fs := billyfs.NewOSFS("/path/to/repo")
//
//	// Open existing repository
//	repo, err := git.Open(context.Background(), &git.Options{
//	    FS: fs,
//	    Workdir: ".",
//	})
//
//	// Or initialize new repository
//	repo, err := git.Init(context.Background(), &git.Options{
//	    FS: fs,
//	    Workdir: ".",
//	})
//
// # Working with Branches
//
// Create and switch branches:
//
//	// Create new branch from current HEAD
//	err = repo.CreateBranch(ctx, "feature/new", "HEAD", false, false)
//
//	// Checkout the branch
//	err = repo.CheckoutBranch(ctx, "feature/new", false, false)
//
//	// Get current branch
//	branch, err := repo.CurrentBranch(ctx)
//
// # Making Commits
//
// Stage files and create commits:
//
//	// Stage files
//	err = repo.Add(ctx, "file1.go", "file2.go")
//
//	// Or stage everything
//	err = repo.Add(ctx, ".")
//
//	// Create commit
//	sha, err := repo.Commit(ctx, "feat: add new feature", git.Signature{
//	    Name:  "John Doe",
//	    Email: "john@example.com",
//	}, git.CommitOpts{})
//
// # Synchronization
//
// Fetch, pull, and push changes:
//
//	// Fetch from remote
//	err = repo.Fetch(ctx, "origin", true, 0)
//
//	// Pull with fast-forward only
//	err = repo.PullFFOnly(ctx, "origin")
//
//	// Push current branch
//	err = repo.Push(ctx, "origin", false)
//
// # Working with Tags
//
// Create and manage tags:
//
//	// Create annotated tag
//	err = repo.CreateTag(ctx, "v1.0.0", "HEAD", "Release v1.0.0", true)
//
//	// List tags matching pattern
//	tags, err := repo.Tags(ctx, git.TagPatternFilter("v*"))
//
//	// Delete tag
//	err = repo.DeleteTag(ctx, "v1.0.0")
//
// # History and Diffs
//
// Query commit history and compute diffs:
//
//	// Get commit history with filters
//	iter, err := repo.Log(ctx, git.LogFilter{
//	    Author:   "John",
//	    MaxCount: 10,
//	})
//	defer iter.Close()
//
//	// Iterate through commits
//	err = iter.ForEach(func(c *object.Commit) error {
//	    fmt.Printf("%s: %s\n", c.Hash, c.Message)
//	    return nil
//	})
//
//	// Compute diff between revisions
//	diff, err := repo.Diff(ctx, "HEAD~1", "HEAD", git.ExtensionFilter(".go"))
//	fmt.Println(diff.Text)
//
// # Authentication
//
// The package supports authentication through the AuthProvider interface.
// Implementations for HTTPS (token/password) and SSH (key file, agent) are
// available in the internal/auth package. Users can implement their own
// AuthProvider for custom authentication needs:
//
//	type MyAuthProvider struct {
//	    token string
//	}
//
//	func (a *MyAuthProvider) Method(ctx context.Context, url string) (transport.AuthMethod, error) {
//	    // Return appropriate auth method based on URL
//	    return &http.BasicAuth{Username: "token", Password: a.token}, nil
//	}
//
//	// Use in options
//	opts := &git.Options{
//	    FS:   fs,
//	    Auth: &MyAuthProvider{token: "github_pat_..."},
//	}
//
// # In-Memory Operations
//
// All operations can run entirely in memory for testing:
//
//	// Create in-memory filesystem
//	memFS := billyfs.NewInMemoryFS()
//
//	// Initialize in-memory repository
//	repo, err := git.Init(ctx, &git.Options{
//	    FS:      memFS,
//	    Workdir: "/",
//	})
//
//	// All operations work the same
//	err = memFS.WriteFile("test.txt", []byte("content"), 0644)
//	err = repo.Add(ctx, "test.txt")
//	sha, err := repo.Commit(ctx, "test commit", sig, git.CommitOpts{})
//
// # Error Handling
//
// The package provides sentinel errors for common conditions:
//
//	err := repo.Push(ctx, "origin", false)
//	if errors.Is(err, git.ErrNotFastForward) {
//	    // Handle non-fast-forward push
//	}
//	if errors.Is(err, git.ErrAuthRequired) {
//	    // Handle missing authentication
//	}
//
// # Context Support
//
// All operations accept a context for timeout and cancellation:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	err = repo.Clone(ctx, "https://github.com/example/repo.git", opts)
//	if err != nil {
//	    // Operation was cancelled or timed out
//	}
//
// # Thread Safety
//
// A Repo instance is NOT safe for concurrent writes. Read operations
// (Log, Diff, Refs, CurrentBranch, etc.) can be called concurrently.
// Write operations (Add, Commit, Push, etc.) must be serialized.
//
// # Performance Considerations
//
// The package includes several performance optimizations:
//   - LRU object cache (configurable via StorerCacheSize)
//   - Shallow clone/fetch support (via ShallowDepth option)
//   - Path filtering for diffs to reduce computation
//   - Efficient ref iteration without loading full objects
//
// # Limitations
//
// This package intentionally does not support:
//   - Interactive operations (rebase -i, add -i)
//   - Complex merge conflict resolution
//   - Submodule management (may be added later)
//   - Direct git CLI invocation
//
// For advanced use cases not covered by this facade, the underlying
// go-git repository object can be accessed if needed (though this is
// discouraged for maintainability).
package git
