package secrets

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		targetError error
		expectIs    bool
	}{
		{
			name:        "ErrSecretNotFound matches itself",
			err:         ErrSecretNotFound,
			targetError: ErrSecretNotFound,
			expectIs:    true,
		},
		{
			name:        "ErrSecretEmpty matches itself",
			err:         ErrSecretEmpty,
			targetError: ErrSecretEmpty,
			expectIs:    true,
		},
		{
			name:        "ErrAccessDenied matches itself",
			err:         ErrAccessDenied,
			targetError: ErrAccessDenied,
			expectIs:    true,
		},
		{
			name:        "wrapped ErrSecretNotFound matches",
			err:         fmt.Errorf("operation failed: %w", ErrSecretNotFound),
			targetError: ErrSecretNotFound,
			expectIs:    true,
		},
		{
			name:        "different error does not match",
			err:         errors.New("some other error"),
			targetError: ErrSecretNotFound,
			expectIs:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.Is(tt.err, tt.targetError)
			assert.Equal(t, tt.expectIs, result)
		})
	}
}

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrSecretNotFound message",
			err:      ErrSecretNotFound,
			expected: "secret not found",
		},
		{
			name:     "ErrSecretEmpty message",
			err:      ErrSecretEmpty,
			expected: "secret value is empty",
		},
		{
			name:     "ErrAccessDenied message",
			err:      ErrAccessDenied,
			expected: "access denied to secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}
