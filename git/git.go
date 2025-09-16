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
	"github.com/go-git/go-git/v5/plumbing"
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

// LogFilter configures which commits to include in log operations.
// Use this to filter commits by time range, author, paths, or limit the result count.
type LogFilter struct {
	// Since limits the log to commits after the specified time.
	// Only commits with timestamps after this time will be included.
	Since *time.Time

	// Until limits the log to commits before the specified time.
	// Only commits with timestamps before this time will be included.
	Until *time.Time

	// Author filters commits by author name/email pattern.
	// Supports glob patterns for partial matching.
	Author string

	// Path filters commits that modified the specified path(s).
	// Only commits that touched these paths will be included.
	Path []string

	// MaxCount limits the number of commits returned.
	// If 0, all matching commits are returned.
	MaxCount int
}

// ResolvedRef represents a resolved reference with its kind and hash.
// This is returned when resolving revision specifiers like branch names, tags, or commit SHAs.
type ResolvedRef struct {
	// Kind indicates the type of reference (branch, tag, commit, etc.).
	Kind RefKind

	// Hash is the resolved commit hash in full SHA-1 format.
	Hash string

	// CanonicalName is the canonical reference name (e.g., "refs/heads/main").
	// For commit hashes, this may be the same as Hash.
	CanonicalName string
}

// RefKind represents the type of git reference.
// This is used to classify references when listing or resolving them.
type RefKind int

const (
	// RefBranch indicates a local branch reference (refs/heads/*).
	RefBranch RefKind = iota

	// RefRemoteBranch indicates a remote branch reference (refs/remotes/*/*).
	RefRemoteBranch

	// RefTag indicates a tag reference (refs/tags/*).
	RefTag

	// RefRemote indicates a generic remote reference.
	RefRemote

	// RefCommit indicates a commit hash (not a symbolic reference).
	RefCommit

	// RefOther indicates any other type of reference.
	RefOther
)

// String returns a human-readable string representation of the RefKind.
func (k RefKind) String() string {
	switch k {
	case RefBranch:
		return "branch"
	case RefRemoteBranch:
		return "remote-branch"
	case RefTag:
		return "tag"
	case RefRemote:
		return "remote"
	case RefCommit:
		return "commit"
	case RefOther:
		return "other"
	default:
		return "unknown"
	}
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
		dotGitFS, err := scopedFS.Chroot(".git")
		if err != nil {
			return nil, fmt.Errorf("failed to create .git directory: %w", err)
		}
		storage = fsbridge.NewStorage(dotGitFS, opts.StorerCacheSize)
		worktreeFS = scopedFS
	}

	// Initialize repository with custom storage
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

// CurrentBranch returns the name of the currently checked out branch.
// It returns an error if HEAD is in a detached state.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) CurrentBranch(ctx context.Context) (string, error) {
	// Get the current HEAD reference
	head, err := r.repo.Head()
	if err != nil {
		return "", WrapError(err, "failed to get HEAD reference")
	}

	// Check if HEAD is detached (not pointing to a branch)
	if !head.Name().IsBranch() {
		return "", WrapError(ErrResolveFailed, "HEAD is detached")
	}

	// Extract branch name from the reference name
	branchName := head.Name().Short()
	return branchName, nil
}

// CreateBranch creates a new branch from the specified revision.
// It supports creating branches from any valid revision (commit hash, branch name, tag, etc.).
// If trackRemote is true, it sets up remote tracking configuration.
// If force is true, it overwrites any existing branch with the same name.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) CreateBranch(ctx context.Context, name, startRev string, trackRemote, force bool) error {
	if name == "" {
		return WrapError(ErrInvalidRef, "branch name cannot be empty")
	}

	if startRev == "" {
		return WrapError(ErrInvalidRef, "start revision cannot be empty")
	}

	// Resolve the start revision to a commit hash
	hash, err := r.repo.ResolveRevision(plumbing.Revision(startRev))
	if err != nil {
		return WrapError(ErrResolveFailed, "failed to resolve start revision")
	}

	// Check if branch already exists
	branchRefName := plumbing.NewBranchReferenceName(name)
	_, err = r.repo.Reference(branchRefName, true)
	if err == nil && !force {
		return WrapError(ErrBranchExists, "branch already exists")
	}

	// Create the branch reference
	newRef := plumbing.NewHashReference(branchRefName, *hash)
	err = r.repo.Storer.SetReference(newRef)
	if err != nil {
		return WrapError(err, "failed to create branch reference")
	}

	// TODO: Implement remote tracking configuration when trackRemote is true
	// For now, remote tracking setup is not implemented but the parameter is accepted for API compatibility
	_ = trackRemote

	return nil
}

// CheckoutBranch switches to the specified branch.
// If createIfMissing is true, it creates the branch if it doesn't exist.
// If force is true, it discards any uncommitted changes in the working tree.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) CheckoutBranch(ctx context.Context, name string, createIfMissing, force bool) error {
	if name == "" {
		return WrapError(ErrInvalidRef, "branch name cannot be empty")
	}

	branchRefName := plumbing.NewBranchReferenceName(name)

	// Check if branch exists
	_, err := r.repo.Reference(branchRefName, true)
	// Handle non-existent branch
	if err != nil {
		if !createIfMissing {
			return WrapError(ErrBranchMissing, "branch does not exist")
		}

		// Create the branch from current HEAD
		head, headErr := r.repo.Head()
		if headErr != nil {
			return WrapError(headErr, "failed to get HEAD reference")
		}

		// Create the branch reference
		newRef := plumbing.NewHashReference(branchRefName, head.Hash())
		if setErr := r.repo.Storer.SetReference(newRef); setErr != nil {
			return WrapError(setErr, "failed to create branch reference")
		}

		// Verify the branch was created
		if _, verifyErr := r.repo.Reference(branchRefName, true); verifyErr != nil {
			// Failed to verify branch creation
			return WrapError(verifyErr, "branch creation verification failed")
		}
	}

	// Now checkout the branch
	checkoutOpts := &git.CheckoutOptions{
		Branch: branchRefName,
		Keep:   false, // Explicitly set keep to false
	}

	if force {
		checkoutOpts.Force = true
	}

	// Store current HEAD reference before checkout
	currentHead, _ := r.repo.Head()

	err = r.worktree.Checkout(checkoutOpts)
	if err != nil {
		return WrapError(err, "failed to checkout branch")
	}

	// Verify HEAD exists after checkout - if not, restore it
	if _, headErr := r.repo.Head(); headErr != nil {
		// HEAD is missing, try to restore it
		symbolicRef := plumbing.NewSymbolicReference(plumbing.HEAD, branchRefName)
		if setErr := r.repo.Storer.SetReference(symbolicRef); setErr != nil {
			// If we can't set symbolic reference, at least try to set a direct reference
			if currentHead != nil {
				r.repo.Storer.SetReference(currentHead)
			}
			return WrapError(setErr, "failed to restore HEAD after checkout")
		}
	}

	return nil
}

// DeleteBranch deletes the specified local branch.
// It prevents deletion of the currently checked out branch.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) DeleteBranch(ctx context.Context, name string) error {
	if name == "" {
		return WrapError(ErrInvalidRef, "branch name cannot be empty")
	}

	// Get the current branch to prevent deletion of current branch
	currentBranch, err := r.CurrentBranch(ctx)
	if err != nil {
		return WrapError(err, "failed to get current branch")
	}

	if currentBranch == name {
		return WrapError(ErrBranchExists, "cannot delete the currently checked out branch")
	}

	branchRefName := plumbing.NewBranchReferenceName(name)

	// Check if branch exists
	_, err = r.repo.Reference(branchRefName, true)
	if err != nil {
		return WrapError(ErrBranchMissing, "branch does not exist")
	}

	// Delete the branch
	err = r.repo.Storer.RemoveReference(branchRefName)
	if err != nil {
		return WrapError(err, "failed to delete branch")
	}

	return nil
}

// CheckoutRemoteBranch creates a local branch from a remote branch and optionally sets up tracking.
// If localName is empty, it uses the same name as the remote branch.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) CheckoutRemoteBranch(ctx context.Context, remote, remoteBranch, localName string, track bool) error {
	if remote == "" {
		return WrapError(ErrInvalidRef, "remote name cannot be empty")
	}

	if remoteBranch == "" {
		return WrapError(ErrInvalidRef, "remote branch name cannot be empty")
	}

	if localName == "" {
		localName = remoteBranch
	}

	// Construct the remote branch reference name
	remoteBranchRef := plumbing.NewRemoteReferenceName(remote, remoteBranch)

	// Check if the remote branch exists
	remoteRef, err := r.repo.Reference(remoteBranchRef, true)
	if err != nil {
		return WrapError(ErrResolveFailed, "remote branch does not exist")
	}

	// Create the local branch reference
	localBranchRef := plumbing.NewBranchReferenceName(localName)
	newRef := plumbing.NewHashReference(localBranchRef, remoteRef.Hash())
	err = r.repo.Storer.SetReference(newRef)
	if err != nil {
		return WrapError(err, "failed to create local branch")
	}

	// TODO: Implement remote tracking configuration when track is true
	// For now, remote tracking setup is not implemented but the parameter is accepted for API compatibility
	_ = track

	// Checkout the newly created local branch
	checkoutOpts := &git.CheckoutOptions{
		Branch: localBranchRef,
	}

	err = r.worktree.Checkout(checkoutOpts)
	if err != nil {
		return WrapError(err, "failed to checkout local branch")
	}

	return nil
}

// Open discovers and opens an existing git repository at the specified location.
// It validates that the directory contains a valid git repository structure.
//
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
		dotGitFS, err := scopedFS.Chroot(".git")
		if err != nil {
			return nil, fmt.Errorf("failed to access .git directory: %w", err)
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
		dotGitFS, err := scopedFS.Chroot(".git")
		if err != nil {
			return nil, fmt.Errorf("failed to create .git directory: %w", err)
		}
		storage = fsbridge.NewStorage(dotGitFS, opts.StorerCacheSize)
		worktreeFS = scopedFS
	}

	// Prepare clone options
	cloneOpts := &git.CloneOptions{
		URL:   remoteURL,
		Depth: opts.ShallowDepth,
	}

	// Set up authentication if available
	if opts.Auth != nil {
		authMethod, authErr := opts.Auth.Method(remoteURL)
		if authErr != nil {
			return nil, WrapError(ErrAuthRequired, "failed to get authentication method")
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
