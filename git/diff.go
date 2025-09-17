// Package git provides high-level Git operations through a clean facade.
// This file contains diff-related operations for comparing revisions.
package git

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

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
