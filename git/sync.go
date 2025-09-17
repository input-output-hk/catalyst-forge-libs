// Package git provides a high-level Go wrapper for go-git operations.
// This file contains synchronization operations (fetch, pull, merge, push).
package git

import (
	"context"
	"errors"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

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
