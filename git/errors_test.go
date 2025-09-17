package git

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentinelErrors_Is(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		target   error
		expected bool
	}{
		// Direct sentinel errors
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

		// Wrapped errors
		{"ErrAlreadyUpToDate wrapped", WrapError(ErrAlreadyUpToDate, "context"), ErrAlreadyUpToDate, true},
		{"ErrAuthRequired wrapped", WrapError(ErrAuthRequired, "context"), ErrAuthRequired, true},
		{"ErrBranchExists wrapped", WrapErrorf(ErrBranchExists, "context %s", "arg"), ErrBranchExists, true},

		// Non-matching errors
		{"ErrAlreadyUpToDate vs ErrAuthRequired", ErrAlreadyUpToDate, ErrAuthRequired, false},
		{"ErrBranchExists vs ErrTagExists", ErrBranchExists, ErrTagExists, false},

		// Nil handling
		{"WrapError with nil", WrapError(nil, "context"), ErrAlreadyUpToDate, false},
		{"WrapErrorf with nil", WrapErrorf(nil, "context"), ErrAlreadyUpToDate, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.Is(tt.err, tt.target)
			assert.Equal(t, tt.expected, result,
				"errors.Is(%v, %v) should be %v", tt.err, tt.target, tt.expected)
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
			name:     "wrap ErrAlreadyUpToDate",
			err:      ErrAlreadyUpToDate,
			msg:      "operation failed",
			expected: "operation failed: already up to date",
		},
		{
			name:     "wrap ErrAuthRequired",
			err:      ErrAuthRequired,
			msg:      "authentication needed",
			expected: "authentication needed: authentication required",
		},
		{
			name:     "wrap nil error",
			err:      nil,
			msg:      "context",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := WrapError(tt.err, tt.msg)

			if tt.err == nil {
				assert.Nil(t, wrapped, "WrapError(nil) should return nil")
				return
			}

			require.NotNil(t, wrapped, "WrapError(%v) should not return nil", tt.err)
			assert.Equal(t, tt.expected, wrapped.Error())

			// Verify the original error is still detectable
			assert.True(t, errors.Is(wrapped, tt.err),
				"wrapped error should match original sentinel")
		})
	}
}

func TestWrapErrorf(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		format   string
		args     []any
		expected string
	}{
		{
			name:     "wrap with format",
			err:      ErrBranchExists,
			format:   "branch %s",
			args:     []any{"main"},
			expected: "branch main: branch already exists",
		},
		{
			name:     "wrap with multiple args",
			err:      ErrTagMissing,
			format:   "tag %s in %s",
			args:     []any{"v1.0", "repo"},
			expected: "tag v1.0 in repo: tag does not exist",
		},
		{
			name:     "wrap nil error",
			err:      nil,
			format:   "context %s",
			args:     []any{"arg"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := WrapErrorf(tt.err, tt.format, tt.args...)

			if tt.err == nil {
				assert.Nil(t, wrapped, "WrapErrorf(nil) should return nil")
				return
			}

			require.NotNil(t, wrapped, "WrapErrorf(%v) should not return nil", tt.err)
			assert.Equal(t, tt.expected, wrapped.Error())

			// Verify the original error is still detectable
			assert.True(t, errors.Is(wrapped, tt.err),
				"wrapped error should match original sentinel")
		})
	}
}
