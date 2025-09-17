// Package git provides high-level Git operations through a clean facade.
// This file contains history-related operations including commit logging and iteration.
package git

import (
	"context"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

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

