// Package git provides a high-level Go wrapper for go-git operations.
// It exposes task-oriented operations for repository management while operating
// exclusively through the project's native filesystem abstraction.
package git

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gobilly "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/input-output-hk/catalyst-forge-libs/fs"

	"github.com/input-output-hk/catalyst-forge-libs/git/internal/fsbridge"
)

const (
	// DefaultStorerCacheSize is the default size for the LRU object cache.
	DefaultStorerCacheSize = 1000

	// DefaultWorkdir is the default worktree directory name.
	DefaultWorkdir = "."

	// DefaultRemoteName is the default remote name used for operations.
	DefaultRemoteName = "origin"
)

// Options configures repository discovery/creation and performance.
type Options struct {
	// FS is the REQUIRED native filesystem root (OS or in-memory).
	// All repository state lives within this filesystem.
	FS fs.Filesystem

	// Workdir is the path within FS for the worktree root.
	// Defaults to "." (current directory in FS).
	Workdir string

	// Bare indicates if this should be a bare repository (.git only, no worktree).
	// Defaults to false (non-bare repository with worktree).
	Bare bool

	// StorerCacheSize sets the LRU objects cache entries.
	// Higher values improve performance but use more memory.
	// Defaults to DefaultStorerCacheSize.
	StorerCacheSize int

	// Auth is an optional provider that resolves per-URL AuthMethod.
	// If nil, no authentication will be available.
	Auth AuthProvider

	// HTTPClient is an optional custom transport for network operations.
	// If nil, a default client with reasonable timeouts is used.
	HTTPClient *http.Client

	// ShallowDepth sets the depth for shallow clone/fetch operations.
	// If > 0, operations will be shallow with the specified depth.
	// If 0, full clone/fetch operations are performed.
	ShallowDepth int
}

// Validate checks that the Options are properly configured.
// It returns an error if required fields are missing or invalid.
func (o *Options) Validate() error {
	if o.FS == nil {
		return WrapError(ErrInvalidRef, "FS is required")
	}

	if o.StorerCacheSize < 0 {
		return WrapError(ErrInvalidRef, "StorerCacheSize cannot be negative")
	}

	if o.ShallowDepth < 0 {
		return WrapError(ErrInvalidRef, "ShallowDepth cannot be negative")
	}

	return nil
}

// applyDefaults sets default values for any unset fields in Options.
func (o *Options) applyDefaults() {
	if o.Workdir == "" {
		o.Workdir = DefaultWorkdir
	}

	if o.StorerCacheSize == 0 {
		o.StorerCacheSize = DefaultStorerCacheSize
	}

	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
}

// Init creates a new git repository at the specified location.
// It initializes both bare and non-bare repositories with proper storage and worktree setup.
func Init(ctx context.Context, opts *Options) (*Repo, error) {
	if err := opts.Validate(); err != nil {
		return nil, WrapError(err, "invalid options")
	}

	opts.applyDefaults()

	// Convert fs.Filesystem to billy.Filesystem
	billyFS, err := fsbridge.ToBillyFilesystem(opts.FS)
	if err != nil {
		return nil, fmt.Errorf("filesystem conversion failed: %w", err)
	}

	// Chroot to the workdir to scope the repository location
	scopedFS, err := billyFS.Chroot(opts.Workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to chroot to workdir %q: %w", opts.Workdir, err)
	}

	var storage *filesystem.Storage
	var worktreeFS gobilly.Filesystem

	if opts.Bare {
		// For bare repos, storage is at the root
		storage = fsbridge.NewStorage(scopedFS, opts.StorerCacheSize)
		worktreeFS = nil
	} else {
		// For non-bare repos, storage goes in .git subdirectory
		dotGitFS, chrootErr := scopedFS.Chroot(".git")
		if chrootErr != nil {
			return nil, fmt.Errorf("failed to create .git directory: %w", chrootErr)
		}
		storage = fsbridge.NewStorage(dotGitFS, opts.StorerCacheSize)
		worktreeFS = scopedFS
	}

	// Initialize new repository
	repo, err := git.Init(storage, worktreeFS)
	if err != nil {
		return nil, WrapError(err, "failed to initialize repository")
	}

	r := &Repo{
		repo:    repo,
		fs:      opts.FS,
		options: *opts,
	}

	// Set up worktree for non-bare repositories
	if !opts.Bare {
		worktree, err := repo.Worktree()
		if err != nil {
			return nil, WrapError(err, "failed to get worktree")
		}
		r.worktree = worktree
	}

	return r, nil
}

// Open discovers and opens an existing git repository.
// The repository must already exist at the specified workdir within the filesystem.
// For non-bare repositories, both .git directory and worktree must be present.
// For bare repositories, only the .git directory structure is expected.
//
// Context timeout/cancellation is honored during repository validation.
func Open(ctx context.Context, opts *Options) (*Repo, error) {
	if err := opts.Validate(); err != nil {
		return nil, WrapError(err, "invalid options")
	}

	opts.applyDefaults()

	// Convert fs.Filesystem to billy.Filesystem
	billyFS, err := fsbridge.ToBillyFilesystem(opts.FS)
	if err != nil {
		return nil, fmt.Errorf("filesystem conversion failed: %w", err)
	}

	// Chroot to the workdir to scope the repository location
	scopedFS, err := billyFS.Chroot(opts.Workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to chroot to workdir %q: %w", opts.Workdir, err)
	}

	var storage *filesystem.Storage
	var worktreeFS gobilly.Filesystem

	if opts.Bare {
		// For bare repos, storage is at the root
		storage = fsbridge.NewStorage(scopedFS, opts.StorerCacheSize)
		worktreeFS = nil
	} else {
		// For non-bare repos, storage goes in .git subdirectory
		dotGitFS, chrootErr := scopedFS.Chroot(".git")
		if chrootErr != nil {
			return nil, fmt.Errorf("failed to access .git directory: %w", chrootErr)
		}
		storage = fsbridge.NewStorage(dotGitFS, opts.StorerCacheSize)
		worktreeFS = scopedFS
	}

	// Open existing repository
	repo, err := git.Open(storage, worktreeFS)
	if err != nil {
		return nil, WrapError(err, "failed to open repository")
	}

	r := &Repo{
		repo:    repo,
		fs:      opts.FS,
		options: *opts,
	}

	// Set up worktree for non-bare repositories
	if !opts.Bare {
		worktree, err := repo.Worktree()
		if err != nil {
			return nil, WrapError(err, "failed to get worktree")
		}
		r.worktree = worktree
	}

	return r, nil
}

// Clone creates a new repository by cloning from a remote URL.
// It supports both bare and non-bare repositories, shallow cloning, and authentication.
//
// The remoteURL should be a valid git URL (https://, ssh://, or file:// for local repos).
// For shallow clones, set ShallowDepth > 0 to limit the clone depth.
// Authentication is handled via the AuthProvider if credentials are required.
//
// Context timeout/cancellation is honored during the clone operation.
func Clone(ctx context.Context, remoteURL string, opts *Options) (*Repo, error) {
	if remoteURL == "" {
		return nil, WrapError(ErrInvalidRef, "remote URL cannot be empty")
	}

	if err := opts.Validate(); err != nil {
		return nil, WrapError(err, "invalid options")
	}

	opts.applyDefaults()

	// Convert fs.Filesystem to billy.Filesystem
	billyFS, err := fsbridge.ToBillyFilesystem(opts.FS)
	if err != nil {
		return nil, fmt.Errorf("filesystem conversion failed: %w", err)
	}

	// Chroot to the workdir to scope the repository location
	scopedFS, err := billyFS.Chroot(opts.Workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to chroot to workdir %q: %w", opts.Workdir, err)
	}

	var storage *filesystem.Storage
	var worktreeFS gobilly.Filesystem

	if opts.Bare {
		// For bare repos, storage is at the root
		storage = fsbridge.NewStorage(scopedFS, opts.StorerCacheSize)
		worktreeFS = nil
	} else {
		// For non-bare repos, storage goes in .git subdirectory
		dotGitFS, chrootErr := scopedFS.Chroot(".git")
		if chrootErr != nil {
			return nil, fmt.Errorf("failed to create .git directory: %w", chrootErr)
		}
		storage = fsbridge.NewStorage(dotGitFS, opts.StorerCacheSize)
		worktreeFS = scopedFS
	}

	// Prepare clone options
	cloneOpts := &git.CloneOptions{
		URL:          remoteURL,
		Depth:        opts.ShallowDepth,
		SingleBranch: opts.ShallowDepth > 0, // Single branch for shallow clones
	}

	// Set up authentication if provided
	if opts.Auth != nil {
		authMethod, authErr := opts.Auth.Method(remoteURL)
		if authErr != nil {
			return nil, WrapError(authErr, "failed to get authentication method")
		}
		cloneOpts.Auth = authMethod
	}

	// Clone the repository
	repo, err := git.Clone(storage, worktreeFS, cloneOpts)
	if err != nil {
		return nil, WrapError(err, "failed to clone repository")
	}

	r := &Repo{
		repo:    repo,
		fs:      opts.FS,
		options: *opts,
	}

	// Set up worktree for non-bare repositories
	if !opts.Bare {
		worktree, err := repo.Worktree()
		if err != nil {
			return nil, WrapError(err, "failed to get worktree")
		}
		r.worktree = worktree
	}

	return r, nil
}

// AuthProvider resolves authentication methods for git operations.
// Implementations should handle different URL schemes and credential sources.
//
//go:generate mockery --name=AuthProvider --output=../mocks
type AuthProvider interface {
	// Method returns the appropriate transport.AuthMethod for the given remote URL.
	// Returns nil if no authentication is needed/available for this URL.
	// Returns an error if authentication cannot be resolved for the URL.
	Method(remoteURL string) (transport.AuthMethod, error)
}

// Signature represents an author/committer signature for commits and tags.
// This is used when creating commits and annotated tags to identify the author.
type Signature struct {
	// Name is the author's or committer's name.
	Name string

	// Email is the author's or committer's email address.
	Email string

	// When is the timestamp for the signature.
	When time.Time
}

// CommitOpts configures commit creation behavior.
// These options control how commits are created and what files are included.
type CommitOpts struct {
	// AllowEmpty allows creating commits with no changes.
	// By default, empty commits are not allowed.
	AllowEmpty bool

	// All adds all modified and untracked files to the index before committing.
	// Equivalent to running 'git add .' before commit.
	All bool

	// Amend amends the tip of the current branch with this commit.
	// This replaces the current commit rather than creating a new one.
	Amend bool
}

// Repo represents a git repository and provides high-level operations.
// It wraps a go-git Repository and Worktree, operating exclusively through
// the project's native filesystem abstraction.
type Repo struct {
	repo     *git.Repository
	worktree *git.Worktree
	fs       fs.Filesystem
	options  Options
}
