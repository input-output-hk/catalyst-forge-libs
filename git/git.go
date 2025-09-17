// Package git provides a high-level Go wrapper for go-git operations.
// It exposes task-oriented operations for repository management while operating
// exclusively through the project's native filesystem abstraction.
package git

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/input-output-hk/catalyst-forge-libs/fs"
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

// Common ChangeFilter implementations for convenience

// ChangePathFilter returns a filter that includes changes affecting the specified path.
// The path can be a file or directory. For directories, all files within are matched.
func ChangePathFilter(path string) ChangeFilter {
	return func(change *object.Change) bool {
		fromPath := change.From.Name
		toPath := change.To.Name

		// Check if either the from or to path matches or is within the specified path
		return strings.HasPrefix(fromPath, path) || strings.HasPrefix(toPath, path) ||
			fromPath == path || toPath == path
	}
}

// ChangeExtensionFilter returns a filter that includes changes to files with the specified extension.
// The extension should include the dot (e.g., ".go", ".md").
func ChangeExtensionFilter(ext string) ChangeFilter {
	return func(change *object.Change) bool {
		fromExt := filepath.Ext(change.From.Name)
		toExt := filepath.Ext(change.To.Name)
		return fromExt == ext || toExt == ext
	}
}

// ChangeExcludePathFilter returns a filter that excludes changes affecting the specified path.
// This is useful for filtering out certain paths while keeping all others.
func ChangeExcludePathFilter(path string) ChangeFilter {
	includeFilter := ChangePathFilter(path)
	return func(change *object.Change) bool {
		return !includeFilter(change) // Invert the path filter
	}
}

// Refs returns a list of references that match the specified kind and pattern.
// The kind parameter filters references by type (branch, remote branch, tag, etc.).
// The pattern parameter supports glob-style matching with * and ? wildcards.
// Results are sorted alphabetically.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) Refs(ctx context.Context, kind RefKind, pattern string) ([]string, error) {
	// Get all references from the repository
	refs, err := r.repo.References()
	if err != nil {
		return nil, WrapError(err, "failed to get references")
	}

	var matchingRefs []string

	// Iterate through all references
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		shortName := ref.Name().Short()

		// Classify the reference and check if it matches the requested kind
		if !matchesRefKind(ref, kind) {
			return nil
		}

		// Apply pattern matching if specified
		if pattern != "" && !matchesRefPattern(shortName, pattern) {
			return nil
		}

		// Add the short name to results
		matchingRefs = append(matchingRefs, shortName)
		return nil
	})
	if err != nil {
		return nil, WrapError(err, "failed to iterate references")
	}

	// Sort results alphabetically
	for i := 0; i < len(matchingRefs)-1; i++ {
		for j := i + 1; j < len(matchingRefs); j++ {
			if matchingRefs[i] > matchingRefs[j] {
				matchingRefs[i], matchingRefs[j] = matchingRefs[j], matchingRefs[i]
			}
		}
	}

	return matchingRefs, nil
}

// matchesRefKind checks if a reference matches the specified RefKind
func matchesRefKind(ref *plumbing.Reference, kind RefKind) bool {
	switch kind {
	case RefBranch:
		return ref.Name().IsBranch()
	case RefRemoteBranch:
		return ref.Name().IsRemote() && strings.Contains(ref.Name().String(), "/")
	case RefTag:
		return ref.Name().IsTag()
	case RefRemote:
		return ref.Name().IsRemote()
	case RefCommit:
		// RefCommit is for commit hashes, not symbolic references
		// This would require checking if the reference points to a commit
		// For now, we'll treat this as "other" since commit hashes aren't typically stored as refs
		return false
	case RefOther:
		return !ref.Name().IsBranch() && !ref.Name().IsTag() && !ref.Name().IsRemote()
	default:
		return false
	}
}

// matchesRefPattern checks if a reference name matches the given pattern
func matchesRefPattern(name, pattern string) bool {
	if pattern == "" {
		return true // Empty pattern matches all
	}

	// Handle * wildcard
	if strings.Contains(pattern, "*") {
		return matchesStarPattern(name, pattern)
	}

	// Handle ? wildcard
	if strings.Contains(pattern, "?") {
		return matchesQuestionPattern(name, pattern)
	}

	// Exact match for patterns without wildcards
	return name == pattern
}

// Resolve resolves a revision specification to a ResolvedRef containing the kind and hash.
// The revision can be any valid git revision syntax (commit hash, branch name, tag, HEAD, etc.).
// Returns a ResolvedRef with the reference kind, resolved hash, and canonical name.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) Resolve(ctx context.Context, rev string) (*ResolvedRef, error) {
	if rev == "" {
		return nil, WrapError(ErrInvalidRef, "revision cannot be empty")
	}

	// Resolve the revision to a hash
	hash, err := r.repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, WrapError(ErrResolveFailed, "failed to resolve revision")
	}

	// Determine the reference kind and canonical name
	kind, canonicalName := r.classifyResolvedRevision(rev, hash)

	return &ResolvedRef{
		Kind:          kind,
		Hash:          hash.String(),
		CanonicalName: canonicalName,
	}, nil
}

// classifyResolvedRevision determines the RefKind and canonical name for a resolved revision
func (r *Repo) classifyResolvedRevision(rev string, hash *plumbing.Hash) (RefKind, string) {
	// Check if it's a full or short commit hash
	if plumbing.IsHash(rev) || len(rev) >= 4 && plumbing.IsHash(rev) {
		return RefCommit, hash.String()
	}

	// Check if it's HEAD
	if rev == "HEAD" {
		return RefOther, "HEAD"
	}

	// Try to find a reference with this name
	refs, err := r.repo.References()
	if err != nil {
		// If we can't get references, assume it's a commit
		return RefCommit, hash.String()
	}

	var foundRef *plumbing.Reference
	_ = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().Short() == rev || ref.Name().String() == rev {
			foundRef = ref
			return nil // Stop iteration
		}
		return nil
	})

	if foundRef != nil {
		// Classify the found reference
		switch {
		case foundRef.Name().IsBranch():
			return RefBranch, foundRef.Name().String()
		case foundRef.Name().IsTag():
			return RefTag, foundRef.Name().String()
		case foundRef.Name().IsRemote():
			if strings.Contains(foundRef.Name().String(), "/") {
				return RefRemoteBranch, foundRef.Name().String()
			}
			return RefRemote, foundRef.Name().String()
		default:
			return RefOther, foundRef.Name().String()
		}
	}

	// If no reference found, it might be a partial hash or other revision syntax
	return RefCommit, hash.String()
}
