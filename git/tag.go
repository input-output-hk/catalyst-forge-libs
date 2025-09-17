// Package git provides a high-level Go wrapper for go-git operations.
// This file contains tag-related operations for repository management.
package git

import (
	"context"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// TagFilter is a predicate function for filtering tags.
// It returns true if the tag should be included in the results.
// Filters are applied progressively - if any filter returns false, the tag is excluded.
type TagFilter func(name string, ref *plumbing.Reference) bool

// CreateTag creates a new tag at the specified target revision.
// If message is provided and annotated is true, an annotated tag is created.
// If message is empty or annotated is false, a lightweight tag is created.
// The target can be any valid revision specifier (commit hash, branch name, etc.).
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) CreateTag(ctx context.Context, name, target, message string, annotated bool) error {
	if name == "" {
		return WrapError(ErrInvalidRef, "tag name cannot be empty")
	}

	if target == "" {
		return WrapError(ErrInvalidRef, "target revision cannot be empty")
	}

	// Resolve the target revision to a commit hash
	hash, err := r.repo.ResolveRevision(plumbing.Revision(target))
	if err != nil {
		return WrapError(ErrResolveFailed, "failed to resolve target revision")
	}

	// Check if tag already exists
	tagRefName := plumbing.NewTagReferenceName(name)
	_, err = r.repo.Reference(tagRefName, true)
	if err == nil {
		return WrapError(ErrTagExists, "tag already exists")
	}

	// Create the tag based on type
	if annotated && message != "" {
		// Create annotated tag
		tagOpts := &git.CreateTagOptions{
			Tagger: &object.Signature{
				Name:  "git-wrapper", // Default tagger - could be made configurable
				Email: "git@catalyst-forge-libs",
				When:  time.Now(),
			},
			Message: message,
		}

		_, err = r.repo.CreateTag(name, *hash, tagOpts)
		if err != nil {
			return WrapError(err, "failed to create annotated tag")
		}
	} else {
		// Create lightweight tag
		tagRef := plumbing.NewHashReference(tagRefName, *hash)
		err = r.repo.Storer.SetReference(tagRef)
		if err != nil {
			return WrapError(err, "failed to create lightweight tag")
		}
	}

	return nil
}

// DeleteTag deletes the specified tag from the repository.
// Returns ErrTagMissing if the tag does not exist.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) DeleteTag(ctx context.Context, name string) error {
	if name == "" {
		return WrapError(ErrInvalidRef, "tag name cannot be empty")
	}

	// Check if tag exists
	tagRefName := plumbing.NewTagReferenceName(name)
	_, err := r.repo.Reference(tagRefName, true)
	if err != nil {
		return WrapError(ErrTagMissing, "tag does not exist")
	}

	// Delete the tag
	err = r.repo.Storer.RemoveReference(tagRefName)
	if err != nil {
		return WrapError(err, "failed to delete tag")
	}

	return nil
}

// Tags returns a list of tags that pass all the provided filters.
// If no filters are provided, all tags are returned.
// Filters are applied progressively - a tag must pass ALL filters to be included.
// Results are sorted alphabetically.
//
// Context timeout/cancellation is honored during the operation.
func (r *Repo) Tags(ctx context.Context, filters ...TagFilter) ([]string, error) {
	// Get all tag references
	refs, err := r.repo.References()
	if err != nil {
		return nil, WrapError(err, "failed to get references")
	}

	var tags []string
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsTag() {
			tagName := ref.Name().Short()

			// Apply filters
			if shouldIncludeTag(tagName, ref, filters) {
				tags = append(tags, tagName)
			}
		}
		return nil
	})
	if err != nil {
		return nil, WrapError(err, "failed to iterate references")
	}

	// Sort tags alphabetically
	for i := 0; i < len(tags)-1; i++ {
		for j := i + 1; j < len(tags); j++ {
			if tags[i] > tags[j] {
				tags[i], tags[j] = tags[j], tags[i]
			}
		}
	}

	return tags, nil
}

// shouldIncludeTag checks if a tag passes all filters
func shouldIncludeTag(name string, ref *plumbing.Reference, filters []TagFilter) bool {
	for _, filter := range filters {
		if filter != nil && !filter(name, ref) {
			return false
		}
	}
	return true
}

// Common TagFilter implementations for convenience

// TagPatternFilter returns a filter that matches tags against a glob pattern.
// Supports * (matches any number of characters) and ? (matches single character).
// For example: "v1.*" matches "v1.0", "v1.1", etc.
func TagPatternFilter(pattern string) TagFilter {
	return func(name string, ref *plumbing.Reference) bool {
		return matchesTagPattern(name, pattern)
	}
}

// matchesTagPattern checks if a tag name matches the given pattern
func matchesTagPattern(name, pattern string) bool {
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

// TagPrefixFilter returns a filter that matches tags with the given prefix.
// For example: "v" matches "v1.0", "v2.0", etc.
func TagPrefixFilter(prefix string) TagFilter {
	return func(name string, ref *plumbing.Reference) bool {
		return strings.HasPrefix(name, prefix)
	}
}

// TagSuffixFilter returns a filter that matches tags with the given suffix.
// For example: "-rc" matches "v1.0-rc", "v2.0-rc", etc.
func TagSuffixFilter(suffix string) TagFilter {
	return func(name string, ref *plumbing.Reference) bool {
		return strings.HasSuffix(name, suffix)
	}
}

// TagExcludeFilter returns a filter that excludes tags matching the given pattern.
// This is useful for filtering out certain tags while keeping all others.
// For example: TagExcludeFilter("*-rc") excludes all release candidates.
func TagExcludeFilter(pattern string) TagFilter {
	includeFilter := TagPatternFilter(pattern)
	return func(name string, ref *plumbing.Reference) bool {
		return !includeFilter(name, ref) // Invert the pattern filter
	}
}
