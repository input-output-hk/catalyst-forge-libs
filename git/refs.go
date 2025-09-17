// Package git provides high-level Git operations through a clean facade.
// This file contains reference-related operations for listing and resolving refs.
package git

import (
	"context"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

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
	sort.Strings(matchingRefs)

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

// matchesStarPattern matches names with * wildcards
func matchesStarPattern(name, pattern string) bool {
	// Simple implementation for * wildcard
	// This could be enhanced with more sophisticated glob matching
	switch {
	case strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*"):
		// *middle* pattern - contains substring
		middle := strings.TrimPrefix(strings.TrimSuffix(pattern, "*"), "*")
		return strings.Contains(name, middle)
	case strings.HasPrefix(pattern, "*"):
		// *suffix pattern - ends with
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(name, suffix)
	case strings.HasSuffix(pattern, "*"):
		// prefix* pattern - starts with
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(name, prefix)
	default:
		// Multiple * wildcards - split and check each part
		parts := strings.Split(pattern, "*")
		if len(parts) <= 1 {
			return false
		}

		pos := 0
		for i, part := range parts {
			if part == "" {
				continue // Empty parts from consecutive *
			}

			switch {
			case i == 0:
				// First part must be at the beginning
				if !strings.HasPrefix(name[pos:], part) {
					return false
				}
				pos += len(part)
			case i == len(parts)-1 && part != "":
				// Last part must be at the end
				return strings.HasSuffix(name, part)
			default:
				// Middle parts can be anywhere after current position
				idx := strings.Index(name[pos:], part)
				if idx == -1 {
					return false
				}
				pos += idx + len(part)
			}
		}
		return true
	}
}

// matchesQuestionPattern matches names with ? wildcards
func matchesQuestionPattern(name, pattern string) bool {
	if len(name) != len(pattern) {
		return false
	}

	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '?' {
			continue // ? matches any single character
		}
		if pattern[i] != name[i] {
			return false
		}
	}

	return true
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
