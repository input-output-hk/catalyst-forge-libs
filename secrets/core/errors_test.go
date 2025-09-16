package core

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandardErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrSecretNotFound",
			err:      ErrSecretNotFound,
			expected: "secret not found",
		},
		{
			name:     "ErrProviderError",
			err:      ErrProviderError,
			expected: "provider error",
		},
		{
			name:     "ErrInvalidRef",
			err:      ErrInvalidRef,
			expected: "invalid secret reference",
		},
		{
			name:     "ErrAccessDenied",
			err:      ErrAccessDenied,
			expected: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestProviderError(t *testing.T) {
	t.Run("creation and formatting", func(t *testing.T) {
		ref := SecretRef{Path: "test/secret", Version: "latest"}
		originalErr := errors.New("connection failed")
		providerErr := NewProviderError("aws", ref, originalErr)

		expectedMsg := `provider "aws" error for secret "test/secret": connection failed`
		assert.Equal(t, expectedMsg, providerErr.Error())
		assert.Equal(t, "aws", providerErr.Provider)
		assert.Equal(t, ref, providerErr.Ref)
		assert.Equal(t, originalErr, providerErr.Err)
	})

	t.Run("error wrapping", func(t *testing.T) {
		ref := SecretRef{Path: "db/password"}
		originalErr := errors.New("timeout")
		wrappedErr := WrapProviderError("vault", ref, originalErr, "failed to fetch secret")

		assert.Error(t, wrappedErr)
		assert.Contains(t, wrappedErr.Error(), "failed to fetch secret")
		assert.Contains(t, wrappedErr.Error(), "provider \"vault\" error")
		assert.Contains(t, wrappedErr.Error(), "timeout")
	})

	t.Run("nil error handling", func(t *testing.T) {
		ref := SecretRef{Path: "test"}
		result := WrapProviderError("test", ref, nil, "message")
		assert.Nil(t, result)
	})

	t.Run("error chain traversal", func(t *testing.T) {
		ref := SecretRef{Path: "test"}
		originalErr := errors.New("network error")
		providerErr := NewProviderError("aws", ref, originalErr)

		// Test errors.Is with the original error
		assert.True(t, errors.Is(providerErr, originalErr))

		// Test errors.Is with ErrProviderError (should not match directly)
		assert.False(t, errors.Is(providerErr, ErrProviderError))

		// Test Unwrap
		unwrapped := providerErr.Unwrap()
		assert.Equal(t, originalErr, unwrapped)
	})

	t.Run("is provider error check", func(t *testing.T) {
		ref := SecretRef{Path: "test"}
		originalErr := errors.New("test error")

		t.Run("direct ProviderError", func(t *testing.T) {
			providerErr := NewProviderError("test", ref, originalErr)
			assert.True(t, IsProviderError(providerErr))
		})

		t.Run("wrapped ProviderError", func(t *testing.T) {
			providerErr := NewProviderError("test", ref, originalErr)
			wrappedErr := fmt.Errorf("operation failed: %w", providerErr)
			assert.True(t, IsProviderError(wrappedErr))
		})

		t.Run("non-provider error", func(t *testing.T) {
			regularErr := errors.New("regular error")
			assert.False(t, IsProviderError(regularErr))
		})
	})
}

func TestValidationError(t *testing.T) {
	t.Run("creation and formatting", func(t *testing.T) {
		validationErr := NewValidationError("path", "", "cannot be empty")

		expectedMsg := `validation failed for field "path": cannot be empty (value: "")`
		assert.Equal(t, expectedMsg, validationErr.Error())
		assert.Equal(t, "path", validationErr.Field)
		assert.Equal(t, "", validationErr.Value)
		assert.Equal(t, "cannot be empty", validationErr.Message)
	})

	t.Run("is validation error check", func(t *testing.T) {
		t.Run("direct ValidationError", func(t *testing.T) {
			validationErr := NewValidationError("version", "invalid", "invalid format")
			assert.True(t, IsValidationError(validationErr))
		})

		t.Run("wrapped ValidationError", func(t *testing.T) {
			validationErr := NewValidationError("path", "/invalid", "invalid characters")
			wrappedErr := fmt.Errorf("validation failed: %w", validationErr)
			assert.True(t, IsValidationError(wrappedErr))
		})

		t.Run("non-validation error", func(t *testing.T) {
			regularErr := errors.New("regular error")
			assert.False(t, IsValidationError(regularErr))
		})
	})
}

func TestErrorWrappingPatterns(t *testing.T) {
	t.Run("multiple levels of wrapping", func(t *testing.T) {
		// Simulate a provider error chain
		ref := SecretRef{Path: "api/key", Version: "v1"}

		// Original low-level error
		networkErr := errors.New("connection timeout")

		// Provider wraps it with context
		providerErr := NewProviderError("aws", ref, networkErr)

		// Manager wraps it with operation context
		managerErr := fmt.Errorf("failed to resolve secret from provider: %w", providerErr)

		// Final user-facing error
		userErr := fmt.Errorf("secret resolution failed: %w", managerErr)

		// Verify error chain preservation
		assert.True(t, errors.Is(userErr, networkErr))
		assert.True(t, IsProviderError(userErr))

		// Extract provider error details
		var extractedProviderErr *ProviderError
		require.True(t, errors.As(userErr, &extractedProviderErr))
		assert.Equal(t, "aws", extractedProviderErr.Provider)
		assert.Equal(t, ref.Path, extractedProviderErr.Ref.Path)
	})

	t.Run("context preservation in error chains", func(t *testing.T) {
		ref := SecretRef{Path: "db/creds"}

		// Start with a standard error
		baseErr := ErrSecretNotFound

		// Add provider context
		providerErr := NewProviderError("memory", ref, baseErr)

		// Add operation context
		opErr := fmt.Errorf("batch resolve failed for path %q: %w", ref.Path, providerErr)

		// Verify we can still identify the original error type
		assert.True(t, errors.Is(opErr, ErrSecretNotFound))

		// Verify we can extract provider information
		var extractedProviderErr *ProviderError
		require.True(t, errors.As(opErr, &extractedProviderErr))
		assert.Equal(t, "memory", extractedProviderErr.Provider)
		assert.Equal(t, ref.Path, extractedProviderErr.Ref.Path)
	})
}
