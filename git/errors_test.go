package git

import (
	"errors"
	"testing"
)

func TestSentinelErrors_Is(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		target   error
		expected bool
	}{
		{"ErrAlreadyUpToDate direct", ErrAlreadyUpToDate, ErrAlreadyUpToDate, true},
		{"ErrAuthRequired direct", ErrAuthRequired, ErrAuthRequired, true},
		{"ErrAuthFailed direct", ErrAuthFailed, ErrAuthFailed, true},
		{"ErrBranchExists direct", ErrBranchExists, ErrBranchExists, true},
		{"ErrBranchMissing direct", ErrBranchMissing, ErrBranchMissing, true},
		{"ErrTagExists direct", ErrTagExists, ErrTagExists, true},
		{"ErrTagMissing direct", ErrTagMissing, ErrTagMissing, true},
		{"ErrNotFastForward direct", ErrNotFastForward, ErrNotFastForward, true},
		{"ErrMergeConflict direct", ErrMergeConflict, ErrMergeConflict, true},
		{"ErrInvalidRef direct", ErrInvalidRef, ErrInvalidRef, true},
		{"ErrResolveFailed direct", ErrResolveFailed, ErrResolveFailed, true},

		// Test wrapped errors
		{"ErrAlreadyUpToDate wrapped", WrapError(ErrAlreadyUpToDate, "context"), ErrAlreadyUpToDate, true},
		{"ErrAuthRequired wrapped", WrapError(ErrAuthRequired, "context"), ErrAuthRequired, true},
		{"ErrBranchExists wrapped", WrapErrorf(ErrBranchExists, "context %s", "arg"), ErrBranchExists, true},

		// Test non-matching errors
		{"ErrAlreadyUpToDate vs ErrAuthRequired", ErrAlreadyUpToDate, ErrAuthRequired, false},
		{"ErrBranchExists vs ErrTagExists", ErrBranchExists, ErrTagExists, false},

		// Test nil handling
		{"WrapError with nil", WrapError(nil, "context"), ErrAlreadyUpToDate, false},
		{"WrapErrorf with nil", WrapErrorf(nil, "context"), ErrAlreadyUpToDate, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.Is(tt.err, tt.target)
			if result != tt.expected {
				t.Errorf("errors.Is(%v, %v) = %v; want %v", tt.err, tt.target, result, tt.expected)
			}
		})
	}
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		msg      string
		expected string
	}{
		{
			"wrap ErrAlreadyUpToDate",
			ErrAlreadyUpToDate,
			"operation failed",
			"operation failed: already up to date",
		},
		{
			"wrap ErrAuthRequired",
			ErrAuthRequired,
			"authentication needed",
			"authentication needed: authentication required",
		},
		{"wrap nil error", nil, "context", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := WrapError(tt.err, tt.msg)
			if tt.err == nil {
				if wrapped != nil {
					t.Errorf("WrapError(nil, %q) = %v; want nil", tt.msg, wrapped)
				}
				return
			}

			if wrapped == nil {
				t.Errorf("WrapError(%v, %q) = nil; want non-nil", tt.err, tt.msg)
				return
			}

			if wrapped.Error() != tt.expected {
				t.Errorf("WrapError(%v, %q).Error() = %q; want %q",
					tt.err, tt.msg, wrapped.Error(), tt.expected)
			}

			// Verify the original error is still detectable
			if !errors.Is(wrapped, tt.err) {
				t.Errorf("errors.Is(wrapped, original) = false; want true")
			}
		})
	}
}

func TestWrapErrorf(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		format   string
		args     []interface{}
		expected string
	}{
		{
			"wrap with format",
			ErrBranchExists,
			"branch %s",
			[]interface{}{"main"},
			"branch main: branch already exists",
		},
		{
			"wrap with multiple args",
			ErrTagMissing,
			"tag %s in %s",
			[]interface{}{"v1.0", "repo"},
			"tag v1.0 in repo: tag does not exist",
		},
		{"wrap nil error", nil, "context %s", []interface{}{"arg"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := WrapErrorf(tt.err, tt.format, tt.args...)
			if tt.err == nil {
				if wrapped != nil {
					t.Errorf("WrapErrorf(nil, %q, %v) = %v; want nil", tt.format, tt.args, wrapped)
				}
				return
			}

			if wrapped == nil {
				t.Errorf("WrapErrorf(%v, %q, %v) = nil; want non-nil", tt.err, tt.format, tt.args)
				return
			}

			if wrapped.Error() != tt.expected {
				t.Errorf("WrapErrorf(%v, %q, %v).Error() = %q; want %q",
					tt.err, tt.format, tt.args, wrapped.Error(), tt.expected)
			}

			// Verify the original error is still detectable
			if !errors.Is(wrapped, tt.err) {
				t.Errorf("errors.Is(wrapped, original) = false; want true")
			}
		})
	}
}
