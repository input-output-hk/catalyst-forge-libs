// Package git provides a high-level Go wrapper for go-git operations.
// This file contains worktree operations (add, remove, unstage, commit).
package git

import (
	"context"
	"errors"
	"strings"

	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/input-output-hk/catalyst-forge-libs/git/internal/fsbridge"
)

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

	// Remove each resolved path from the index and worktree
	for _, path := range pathsToRemove {
		// Try to remove the file from the index and worktree
		// go-git's Remove will handle checking if the file is tracked
		_, err := r.worktree.Remove(path)
		if err != nil {
			// Only return error if it's not a "entry not found" type error
			// (matching git rm behavior - silently ignore untracked files)
			errMsg := err.Error()
			if !strings.Contains(errMsg, "entry not found") && !strings.Contains(errMsg, "does not exist") {
				return WrapErrorf(err, "failed to remove path %q", path)
			}
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
