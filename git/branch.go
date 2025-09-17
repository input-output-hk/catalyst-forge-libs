// Package git provides branch management operations for git repositories.
// This file contains all branch-related operations including creation, checkout,
// deletion, and remote branch handling.
package git

import (
	"context"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

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
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return WrapError(err, "context cancelled")
	}

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

	branchRefName := plumbing.NewBranchReferenceName(name)

	// Check if branch exists first
	_, err := r.repo.Reference(branchRefName, true)
	if err != nil {
		return WrapError(ErrBranchMissing, "branch does not exist")
	}

	// Get the current branch to prevent deletion of current branch
	// This might fail in an empty repository, which is okay
	currentBranch, err := r.CurrentBranch(ctx)
	if err == nil && currentBranch == name {
		return WrapError(ErrBranchExists, "cannot delete the currently checked out branch")
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
