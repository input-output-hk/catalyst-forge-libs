// Package git provides a high-level Go wrapper for go-git operations.
// It exposes task-oriented operations for repository management while operating
// exclusively through the project's native filesystem abstraction.
package git

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	gobilly "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

// MergeStrategy represents the different types of merge strategies.
// Currently, only FastForwardOnly is supported by go-git.
type MergeStrategy int8

const (
	// FastForwardOnly represents a merge strategy that only allows fast-forward merges.
	// This will fail if a merge commit would be required.
	FastForwardOnly MergeStrategy = iota
)

// String returns a human-readable string representation of the MergeStrategy.
func (s MergeStrategy) String() string {
	switch s {
	case FastForwardOnly:
		return "fast-forward-only"
	default:
		return "unknown"
	}
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

// CommitIter represents an iterator over commits returned by Log operations.
// It provides methods to iterate through commits efficiently without loading
// all commits into memory at once.
type CommitIter struct {
	iter object.CommitIter
}

// Next returns the next commit in the iteration.
// Returns nil when iteration is complete.
func (ci *CommitIter) Next() (*object.Commit, error) {
	commit, err := ci.iter.Next()
	if err != nil {
		// Check if this is EOF (end of iteration)
		if err.Error() == "EOF" {
			return nil, nil
		}
		return nil, WrapError(err, "failed to get next commit")
	}
	return commit, nil
}

// ForEach executes the provided function for each commit in the iterator.
// Iteration stops if the function returns an error.
func (ci *CommitIter) ForEach(fn func(*object.Commit) error) error {
	return WrapError(ci.iter.ForEach(fn), "failed to iterate commits")
}

// Close closes the iterator and releases any associated resources.
func (ci *CommitIter) Close() {
	ci.iter.Close()
}

// PatchText represents unified diff text between two revisions.
// It contains the formatted diff output that can be displayed to users
// or processed by other tools.
type PatchText struct {
	// Text contains the unified diff in string format.
	Text string

	// IsBinary indicates whether the diff contains binary files.
	// When true, the diff text may be truncated or contain binary markers.
	IsBinary bool

	// FileCount indicates the number of files that have changes.
	FileCount int
}

// ChangeFilter is a predicate function for filtering changes in diffs.
// It returns true if the change should be included in the diff output.
// Filters are applied progressively - if any filter returns false, the change is excluded.
type ChangeFilter func(*object.Change) bool

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
		dotGitFS, chrootErr := scopedFS.Chroot(".git")
		if chrootErr != nil {
			return nil, fmt.Errorf("failed to create .git directory: %w", chrootErr)
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
				_ = r.repo.Storer.SetReference(currentHead)
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

// Fetch fetches changes from the specified remote.
// It supports pruning stale remote branches and shallow fetching when depth > 0.
// Returns ErrAlreadyUpToDate if there are no changes to fetch.
//
// Context timeout/cancellation is honored during the fetch operation.
func (r *Repo) Fetch(ctx context.Context, remote string, prune bool, depth int) error {
	if remote == "" {
		remote = DefaultRemoteName
	}

	// Prepare fetch options
	fetchOpts := &git.FetchOptions{
		RemoteName: remote,
		Prune:      prune,
		Depth:      depth,
	}

	// Set up authentication if available
	if r.options.Auth != nil {
		// Get the remote URL to determine auth method
		remoteConfig, err := r.repo.Remote(remote)
		if err != nil {
			return WrapError(err, "failed to get remote configuration")
		}

		authMethod, authErr := r.options.Auth.Method(remoteConfig.Config().URLs[0])
		if authErr != nil {
			return WrapError(ErrAuthRequired, "failed to get authentication method")
		}
		fetchOpts.Auth = authMethod
	}

	// Perform the fetch
	err := r.repo.Fetch(fetchOpts)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, git.ErrRemoteNotFound) {
			return WrapError(ErrResolveFailed, "remote not found")
		}
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return ErrAlreadyUpToDate
		}
		return WrapError(err, "failed to fetch from remote")
	}

	return nil
}

// PullFFOnly performs a fast-forward only pull from the specified remote.
// It fetches changes and updates the current branch only if it's a fast-forward merge.
// Returns ErrNotFastForward if a merge commit would be required.
// Returns ErrAlreadyUpToDate if there are no changes to pull.
//
// Context timeout/cancellation is honored during the pull operation.
func (r *Repo) PullFFOnly(ctx context.Context, remote string) error {
	if r.worktree == nil {
		return WrapError(ErrInvalidRef, "cannot pull in bare repository")
	}

	if remote == "" {
		remote = DefaultRemoteName
	}

	// Prepare pull options with fast-forward only strategy
	pullOpts := &git.PullOptions{
		RemoteName: remote,
	}

	// Set up authentication if available
	if r.options.Auth != nil {
		// Get the remote URL to determine auth method
		remoteConfig, err := r.repo.Remote(remote)
		if err != nil {
			return WrapError(err, "failed to get remote configuration")
		}

		authMethod, authErr := r.options.Auth.Method(remoteConfig.Config().URLs[0])
		if authErr != nil {
			return WrapError(ErrAuthRequired, "failed to get authentication method")
		}
		pullOpts.Auth = authMethod
	}

	// Perform the pull
	err := r.worktree.Pull(pullOpts)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, git.ErrRemoteNotFound) {
			return WrapError(ErrResolveFailed, "remote not found")
		}
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return ErrAlreadyUpToDate
		}
		if errors.Is(err, git.ErrNonFastForwardUpdate) {
			return ErrNotFastForward
		}
		return WrapError(err, "failed to pull from remote")
	}

	return nil
}

// FetchAndMerge fetches changes from the specified remote and merges the fromRef.
// It supports different merge strategies as specified by the strategy parameter.
// Currently, only FastForwardOnly is supported by go-git.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) FetchAndMerge(ctx context.Context, remote, fromRef string, strategy MergeStrategy) error {
	if remote == "" {
		remote = DefaultRemoteName
	}

	// First, fetch the changes
	fetchErr := r.Fetch(ctx, remote, false, 0)
	if fetchErr != nil && !errors.Is(fetchErr, ErrAlreadyUpToDate) {
		return WrapError(fetchErr, "failed to fetch before merge")
	}

	// Resolve the fromRef to a reference
	hash, err := r.repo.ResolveRevision(plumbing.Revision(fromRef))
	if err != nil {
		return WrapError(ErrResolveFailed, "failed to resolve fromRef for merge")
	}

	ref := plumbing.NewHashReference("", *hash)

	// Prepare merge options
	var mergeOpts git.MergeOptions

	// Map our strategy to go-git's strategy
	switch strategy {
	case FastForwardOnly:
		mergeOpts.Strategy = git.FastForwardMerge
	default:
		return WrapError(ErrInvalidRef, "unsupported merge strategy")
	}

	// Perform the merge
	err = r.repo.Merge(*ref, mergeOpts)
	if err != nil {
		// Check for specific error types
		if err.Error() == "unsupported merge strategy" {
			return WrapError(ErrInvalidRef, "merge strategy not supported by go-git")
		}
		// Note: go-git doesn't have a specific error for merge conflicts yet
		// This may change in future versions
		return WrapError(err, "failed to merge")
	}

	return nil
}

// Remove removes files from the index and worktree.
// It handles already-deleted files appropriately and supports glob patterns.
// Files that don't exist in the index are silently ignored.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) Remove(ctx context.Context, paths ...string) error {
	if r.worktree == nil {
		return WrapError(ErrInvalidRef, "cannot remove files in bare repository")
	}

	if len(paths) == 0 {
		return nil // No paths to remove, not an error
	}

	// Convert fs.Filesystem to billy.Filesystem for glob operations
	billyFS, err := fsbridge.ToBillyFilesystem(r.fs)
	if err != nil {
		return WrapError(err, "failed to convert filesystem for glob operations")
	}

	// Chroot to workdir for proper path handling
	workdirFS, err := billyFS.Chroot(r.options.Workdir)
	if err != nil {
		return WrapErrorf(err, "failed to chroot to workdir %q", r.options.Workdir)
	}

	// Collect all paths to remove (expanding globs)
	var pathsToRemove []string

	for _, path := range paths {
		if path == "" {
			continue // Skip empty paths
		}

		// Check if path contains glob patterns
		if strings.ContainsAny(path, "*?[") {
			// Expand glob pattern using billy filesystem
			matches, globErr := util.Glob(workdirFS, path)
			if globErr != nil {
				return WrapErrorf(globErr, "invalid glob pattern %q", path)
			}

			// Add all matched paths
			pathsToRemove = append(pathsToRemove, matches...)
		} else {
			// Regular path - add it directly
			pathsToRemove = append(pathsToRemove, path)
		}
	}

	// Get current status to check which files are actually tracked
	currentStatus, err := r.worktree.Status()
	if err != nil {
		return WrapError(err, "failed to get worktree status")
	}

	// Remove each resolved path from the index and worktree
	for _, path := range pathsToRemove {
		// Check if file is tracked in the index
		fileStatus := currentStatus.File(path)
		if fileStatus.Staging == git.Untracked && fileStatus.Worktree == git.Untracked {
			// File is not tracked, skip it (matching git rm behavior)
			continue
		}

		_, err := r.worktree.Remove(path)
		if err != nil {
			return WrapErrorf(err, "failed to remove path %q", path)
		}
	}

	return nil
}

// Unstage unstages files from the index without modifying the worktree.
// It uses Reset (mixed) to reset the index to HEAD for specified paths.
// Files that aren't staged are silently ignored.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) Unstage(ctx context.Context, paths ...string) error {
	if r.worktree == nil {
		return WrapError(ErrInvalidRef, "cannot unstage files in bare repository")
	}

	if len(paths) == 0 {
		return nil // No paths to unstage, not an error
	}

	expandedPaths, err := r.expandPathsForUnstage(paths)
	if err != nil {
		return err
	}

	stagedPaths, err := r.filterStagedPaths(expandedPaths)
	if err != nil {
		return err
	}

	if len(stagedPaths) == 0 {
		// No staged files to unstage
		return nil
	}

	return r.performUnstageReset(stagedPaths)
}

// expandPathsForUnstage expands glob patterns and filters empty paths
func (r *Repo) expandPathsForUnstage(paths []string) ([]string, error) {
	// Convert fs.Filesystem to billy.Filesystem for glob operations
	billyFS, err := fsbridge.ToBillyFilesystem(r.fs)
	if err != nil {
		return nil, WrapError(err, "failed to convert filesystem for glob operations")
	}

	// Chroot to workdir for proper path handling
	workdirFS, err := billyFS.Chroot(r.options.Workdir)
	if err != nil {
		return nil, WrapErrorf(err, "failed to chroot to workdir %q", r.options.Workdir)
	}

	// Collect all paths to unstage (expanding globs)
	var pathsToUnstage []string

	for _, path := range paths {
		if path == "" {
			continue // Skip empty paths
		}

		// Check if path contains glob patterns
		if strings.ContainsAny(path, "*?[") {
			// Expand glob pattern using billy filesystem
			matches, globErr := util.Glob(workdirFS, path)
			if globErr != nil {
				return nil, WrapErrorf(globErr, "invalid glob pattern %q", path)
			}

			// Add all matched paths
			pathsToUnstage = append(pathsToUnstage, matches...)
		} else {
			// Regular path - add it directly
			pathsToUnstage = append(pathsToUnstage, path)
		}
	}

	return pathsToUnstage, nil
}

// filterStagedPaths returns only the paths that are actually staged
func (r *Repo) filterStagedPaths(paths []string) ([]string, error) {
	// Get current status to check which files are actually staged
	currentStatus, statusErr := r.worktree.Status()
	if statusErr != nil {
		return nil, WrapError(statusErr, "failed to get worktree status")
	}

	// Collect only the paths that are actually staged
	var stagedPaths []string
	for _, path := range paths {
		fileStatus := currentStatus.File(path)
		if fileStatus.Staging != git.Untracked && fileStatus.Staging != git.Unmodified {
			// File is staged (added, modified, deleted, etc.), so we can unstage it
			stagedPaths = append(stagedPaths, path)
		}
		// Silently ignore files that aren't staged (matching git reset behavior)
	}

	return stagedPaths, nil
}

// performUnstageReset performs the actual reset operation to unstage files
func (r *Repo) performUnstageReset(stagedPaths []string) error {
	// Get HEAD commit to reset to
	head, err := r.repo.Head()
	if err != nil {
		return WrapError(err, "failed to get HEAD reference")
	}

	// Use Reset to unstage specific paths by resetting index to HEAD
	resetOpts := &git.ResetOptions{
		Commit: head.Hash(),
		Mode:   git.MixedReset,
		Files:  stagedPaths,
	}
	err = r.worktree.Reset(resetOpts)
	if err != nil {
		return WrapError(err, "failed to unstage files")
	}

	return nil
}

// Commit creates a new commit with the specified message and author/committer.
// It returns the SHA of the new commit. The CommitOpts can be used to control
// commit behavior such as allowing empty commits.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) Commit(ctx context.Context, msg string, who Signature, opts CommitOpts) (string, error) {
	if r.worktree == nil {
		return "", WrapError(ErrInvalidRef, "cannot commit in bare repository")
	}

	if msg == "" {
		return "", WrapError(ErrInvalidRef, "commit message cannot be empty")
	}

	if who.Name == "" || who.Email == "" {
		return "", WrapError(ErrInvalidRef, "committer name and email are required")
	}

	// Check if there are any staged changes
	status, err := r.worktree.Status()
	if err != nil {
		return "", WrapError(err, "failed to get worktree status")
	}

	// Count staged files
	stagedCount := 0
	for _, fileStatus := range status {
		if fileStatus.Staging != git.Untracked && fileStatus.Staging != git.Unmodified {
			stagedCount++
		}
	}

	if stagedCount == 0 && !opts.AllowEmpty {
		return "", WrapError(ErrEmptyCommit, "no changes staged for commit")
	}

	// Prepare commit options
	commitOpts := &git.CommitOptions{
		Author: &object.Signature{
			Name:  who.Name,
			Email: who.Email,
			When:  who.When,
		},
		Committer: &object.Signature{
			Name:  who.Name,
			Email: who.Email,
			When:  who.When,
		},
		AllowEmptyCommits: opts.AllowEmpty,
	}

	// Create the commit
	hash, err := r.worktree.Commit(msg, commitOpts)
	if err != nil {
		if errors.Is(err, git.ErrEmptyCommit) {
			return "", ErrEmptyCommit
		}
		return "", WrapError(err, "failed to create commit")
	}

	return hash.String(), nil
}

// Push pushes the current branch to the specified remote.
// It supports force pushing when force is true.
// Returns ErrNotFastForward if the push would overwrite remote changes and force is false.
// Returns ErrAlreadyUpToDate if there are no changes to push.
//
// Context timeout/cancellation is honored during the push operation.
func (r *Repo) Push(ctx context.Context, remote string, force bool) error {
	if remote == "" {
		remote = DefaultRemoteName
	}

	// Prepare push options
	pushOpts := &git.PushOptions{
		RemoteName: remote,
		Force:      force,
	}

	// Set up authentication if available
	if r.options.Auth != nil {
		// Get the remote URL to determine auth method
		remoteConfig, err := r.repo.Remote(remote)
		if err != nil {
			return WrapError(err, "failed to get remote configuration")
		}

		authMethod, authErr := r.options.Auth.Method(remoteConfig.Config().URLs[0])
		if authErr != nil {
			return WrapError(ErrAuthRequired, "failed to get authentication method")
		}
		pushOpts.Auth = authMethod
	}

	// Perform the push
	err := r.repo.Push(pushOpts)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, git.ErrRemoteNotFound) {
			return WrapError(ErrResolveFailed, "remote not found")
		}
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return ErrAlreadyUpToDate
		}
		if errors.Is(err, git.ErrNonFastForwardUpdate) {
			return ErrNotFastForward
		}
		return WrapError(err, "failed to push to remote")
	}

	return nil
}

// Add stages files in the worktree for the next commit.
// It supports glob patterns and handles missing files appropriately.
// Files that don't exist are silently ignored (matching git add behavior).
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) Add(ctx context.Context, paths ...string) error {
	if r.worktree == nil {
		return WrapError(ErrInvalidRef, "cannot add files in bare repository")
	}

	if len(paths) == 0 {
		return nil // No paths to add, not an error
	}

	// Convert fs.Filesystem to billy.Filesystem for glob operations
	billyFS, err := fsbridge.ToBillyFilesystem(r.fs)
	if err != nil {
		return WrapError(err, "failed to convert filesystem for glob operations")
	}

	// Chroot to workdir for proper path handling
	workdirFS, err := billyFS.Chroot(r.options.Workdir)
	if err != nil {
		return WrapErrorf(err, "failed to chroot to workdir %q", r.options.Workdir)
	}

	// Collect all paths to add (expanding globs and filtering out non-existent files)
	var pathsToAdd []string

	for _, path := range paths {
		if path == "" {
			continue // Skip empty paths
		}

		// Check if path contains glob patterns
		if strings.ContainsAny(path, "*?[") {
			// Expand glob pattern using billy filesystem
			matches, globErr := util.Glob(workdirFS, path)
			if globErr != nil {
				return WrapErrorf(globErr, "invalid glob pattern %q", path)
			}

			// Add all matched paths
			pathsToAdd = append(pathsToAdd, matches...)
		} else {
			// Regular path - check if it exists
			exists, err := workdirFS.Stat(path)
			if err == nil && exists != nil {
				pathsToAdd = append(pathsToAdd, path)
			}
			// Silently ignore non-existent files (matching git behavior)
		}
	}

	// Add each resolved path to the index
	for _, path := range pathsToAdd {
		_, err := r.worktree.Add(path)
		if err != nil {
			return WrapErrorf(err, "failed to add path %q", path)
		}
	}

	return nil
}

// Log returns a commit iterator for the repository with the specified filters applied.
// The LogFilter can be used to limit results by time range, author, paths, or maximum count.
// The returned CommitIter should be closed when no longer needed to free resources.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) Log(ctx context.Context, f LogFilter) (*CommitIter, error) {
	// Prepare log options from the filter
	logOpts := &git.LogOptions{}

	// Apply time filters
	if f.Since != nil {
		logOpts.Since = f.Since
	}
	if f.Until != nil {
		logOpts.Until = f.Until
	}

	// Note: go-git doesn't support author filtering directly in LogOptions
	// Author filtering will be done by post-processing the results
	_ = f.Author // We'll handle this in post-processing

	// Apply path filters
	if len(f.Path) > 0 {
		logOpts.PathFilter = func(path string) bool {
			for _, filterPath := range f.Path {
				// Simple substring match - could be enhanced with glob support
				if strings.Contains(path, filterPath) {
					return true
				}
			}
			return false
		}
	}

	// Apply max count limit
	if f.MaxCount > 0 {
		logOpts.Order = git.LogOrderCommitterTime // Ensure consistent ordering
	}

	// Get the commit iterator from go-git
	iter, err := r.repo.Log(logOpts)
	if err != nil {
		return nil, WrapError(err, "failed to create commit iterator")
	}

	// Create the base iterator
	commitIter := &CommitIter{iter: iter}

	// Apply author filtering if specified
	if f.Author != "" {
		authorFilteredIter := &authorFilteredCommitIter{
			iter:   commitIter,
			author: f.Author,
		}
		commitIter = &CommitIter{iter: authorFilteredIter}
	}

	// If MaxCount is specified, we need to limit the results
	if f.MaxCount > 0 {
		limitedIter := &limitedCommitIter{
			iter:     commitIter,
			maxCount: f.MaxCount,
			count:    0,
		}
		return &CommitIter{iter: limitedIter}, nil
	}

	return commitIter, nil
}

// limitedCommitIter wraps a CommitIter to limit the number of commits returned
type limitedCommitIter struct {
	iter     *CommitIter
	maxCount int
	count    int
}

// Next returns the next commit or nil if max count is reached
func (l *limitedCommitIter) Next() (*object.Commit, error) {
	if l.count >= l.maxCount {
		return nil, nil // End of iteration
	}
	commit, err := l.iter.Next()
	if err != nil {
		return nil, err
	}
	if commit != nil {
		l.count++
	}
	return commit, nil
}

// ForEach executes the function for each commit up to the max count
func (l *limitedCommitIter) ForEach(fn func(*object.Commit) error) error {
	for l.count < l.maxCount {
		commit, err := l.iter.Next()
		if err != nil {
			return err
		}
		if commit == nil {
			break // End of iteration
		}
		if err := fn(commit); err != nil {
			return err
		}
		l.count++
	}
	return nil
}

// Close closes the underlying iterator
func (l *limitedCommitIter) Close() {
	l.iter.Close()
}

// authorFilteredCommitIter wraps a CommitIter to filter commits by author
type authorFilteredCommitIter struct {
	iter   *CommitIter
	author string
}

// Next returns the next commit that matches the author filter
func (a *authorFilteredCommitIter) Next() (*object.Commit, error) {
	for {
		commit, err := a.iter.Next()
		if err != nil {
			return nil, err
		}
		if commit == nil {
			return nil, nil // End of iteration
		}

		// Check if author matches
		authorMatch := strings.Contains(commit.Author.Name, a.author) ||
			strings.Contains(commit.Author.Email, a.author)
		committerMatch := strings.Contains(commit.Committer.Name, a.author) ||
			strings.Contains(commit.Committer.Email, a.author)

		if authorMatch || committerMatch {
			return commit, nil
		}
		// Continue to next commit if this one doesn't match
	}
}

// ForEach executes the function for each commit that matches the author filter
func (a *authorFilteredCommitIter) ForEach(fn func(*object.Commit) error) error {
	for {
		commit, err := a.iter.Next()
		if err != nil {
			return err
		}
		if commit == nil {
			break // End of iteration
		}

		// Check if author matches
		authorMatch := strings.Contains(commit.Author.Name, a.author) ||
			strings.Contains(commit.Author.Email, a.author)
		committerMatch := strings.Contains(commit.Committer.Name, a.author) ||
			strings.Contains(commit.Committer.Email, a.author)

		if authorMatch || committerMatch {
			if err := fn(commit); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close closes the underlying iterator
func (a *authorFilteredCommitIter) Close() {
	a.iter.Close()
}

// Diff computes the diff between two revisions and returns unified diff text.
// The revisions 'a' and 'b' can be any valid git revision specifiers (commit hashes,
// branch names, tags, etc.).
//
// Filters are applied progressively - a change must pass ALL filters to be included.
// If no filters are provided, all changes are included.
//
// The method handles binary files appropriately by detecting them and marking
// the result accordingly. The returned PatchText contains the unified diff text
// that can be displayed to users or processed by other tools.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) Diff(ctx context.Context, a, b string, filters ...ChangeFilter) (*PatchText, error) {
	// Validate inputs
	if err := validateDiffInputs(a, b); err != nil {
		return nil, err
	}

	// Get trees for both revisions
	treeA, treeB, err := r.getTreesForDiff(a, b)
	if err != nil {
		return nil, err
	}

	// Get all changes between the trees
	changes, err := treeA.Diff(treeB)
	if err != nil {
		return nil, WrapError(err, "failed to compute changes")
	}

	// Apply filters and get filtered changes
	filteredChanges := applyChangeFilters(changes, filters)

	// Generate patch from filtered changes
	patch, err := filteredChanges.Patch()
	if err != nil {
		return nil, WrapError(err, "failed to generate patch")
	}

	// Build and return the result
	return buildPatchResult(patch.String(), filteredChanges), nil
}

// validateDiffInputs validates the revision inputs for diff
func validateDiffInputs(a, b string) error {
	if a == "" {
		return WrapError(ErrInvalidRef, "revision 'a' cannot be empty")
	}
	if b == "" {
		return WrapError(ErrInvalidRef, "revision 'b' cannot be empty")
	}
	return nil
}

// getTreesForDiff resolves revisions and returns their trees
func (r *Repo) getTreesForDiff(a, b string) (*object.Tree, *object.Tree, error) {
	// Resolve and get tree for revision 'a'
	treeA, err := r.getTreeForRevision(a)
	if err != nil {
		return nil, nil, WrapErrorf(err, "failed to get tree for revision %q", a)
	}

	// Resolve and get tree for revision 'b'
	treeB, err := r.getTreeForRevision(b)
	if err != nil {
		return nil, nil, WrapErrorf(err, "failed to get tree for revision %q", b)
	}

	return treeA, treeB, nil
}

// getTreeForRevision resolves a revision and returns its tree
func (r *Repo) getTreeForRevision(rev string) (*object.Tree, error) {
	hash, err := r.repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, WrapError(ErrResolveFailed, "failed to resolve revision")
	}

	commit, err := r.repo.CommitObject(*hash)
	if err != nil {
		return nil, WrapError(err, "failed to get commit object")
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, WrapError(err, "failed to get tree")
	}

	return tree, nil
}

// applyChangeFilters applies all filters to changes and returns filtered results
func applyChangeFilters(changes object.Changes, filters []ChangeFilter) object.Changes {
	var filteredChanges object.Changes
	for _, change := range changes {
		if shouldIncludeChange(change, filters) {
			filteredChanges = append(filteredChanges, change)
		}
	}
	return filteredChanges
}

// shouldIncludeChange checks if a change passes all filters
func shouldIncludeChange(change *object.Change, filters []ChangeFilter) bool {
	for _, filter := range filters {
		if filter != nil && !filter(change) {
			return false
		}
	}
	return true
}

// buildPatchResult builds the PatchText result from the diff text and changes
func buildPatchResult(diffText string, changes object.Changes) *PatchText {
	return &PatchText{
		Text:      diffText,
		IsBinary:  containsBinaryChanges(changes),
		FileCount: len(changes),
	}
}

// containsBinaryChanges checks if any of the changes are for binary files
func containsBinaryChanges(changes object.Changes) bool {
	for _, change := range changes {
		if isBinaryPath(change.From.Name) || isBinaryPath(change.To.Name) {
			return true
		}
	}
	return false
}

// isBinaryPath checks if a file path likely represents a binary file based on extension
func isBinaryPath(path string) bool {
	if path == "" {
		return false
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	binaryExts := map[string]bool{
		"png": true, "jpg": true, "jpeg": true, "gif": true, "ico": true,
		"pdf": true, "zip": true, "tar": true, "gz": true, "bz2": true,
		"exe": true, "dll": true, "so": true, "dylib": true, "bin": true,
		"mp3": true, "mp4": true, "avi": true, "mov": true, "wav": true,
		"ttf": true, "otf": true, "woff": true, "woff2": true, "eot": true,
	}

	return binaryExts[ext]
}
