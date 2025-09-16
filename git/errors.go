// Package git provides sentinel errors for common git operations.
// All errors can be checked using errors.Is() for programmatic handling.
package git

import (
	"errors"
	"fmt"
)

// Common sentinel errors that can be checked with errors.Is().
// These wrap underlying go-git errors while providing a stable API for consumers.

// ErrAlreadyUpToDate is returned when fetch, pull, or push operations
// result in no changes because the local and remote states are already synchronized.
var ErrAlreadyUpToDate = errors.New("already up to date")

// ErrAuthRequired is returned when an operation requires authentication
// but no credentials were provided or available.
var ErrAuthRequired = errors.New("authentication required")

// ErrAuthFailed is returned when authentication was attempted but failed
// (invalid credentials, expired tokens, etc.).
var ErrAuthFailed = errors.New("authentication failed")

// ErrBranchExists is returned when attempting to create a branch that already exists
// and force creation was not requested.
var ErrBranchExists = errors.New("branch already exists")

// ErrBranchMissing is returned when attempting to operate on a branch that does not exist.
var ErrBranchMissing = errors.New("branch does not exist")

// ErrTagExists is returned when attempting to create a tag that already exists
// and force creation was not requested.
var ErrTagExists = errors.New("tag already exists")

// ErrTagMissing is returned when attempting to operate on a tag that does not exist.
var ErrTagMissing = errors.New("tag does not exist")

// ErrNotFastForward is returned when a push or pull operation cannot be performed
// as a fast-forward merge and requires manual conflict resolution.
var ErrNotFastForward = errors.New("not a fast-forward")

// ErrMergeConflict is returned when a merge operation encounters conflicts
// that cannot be automatically resolved.
var ErrMergeConflict = errors.New("merge conflict")

// ErrInvalidRef is returned when a reference name or revision specification
// is malformed or invalid according to git's reference naming rules.
var ErrInvalidRef = errors.New("invalid reference")

// ErrResolveFailed is returned when a revision specification cannot be resolved
// to a valid commit hash (e.g., branch/tag doesn't exist, invalid SHA).
var ErrResolveFailed = errors.New("cannot resolve revision")

// WrapError wraps an error with additional context while preserving
// the ability to check against sentinel errors using errors.Is().
func WrapError(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// WrapErrorf wraps an error with formatted additional context while preserving
// the ability to check against sentinel errors using errors.Is().
func WrapErrorf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}
