// Package secrets provides error types and handling for the secrets management module.
//
// Error handling follows Go best practices with wrapped errors for context preservation.
// All errors defined here can be unwrapped using errors.Is() and errors.As().
package secrets

import (
	"errors"
	"fmt"
)

// Standard error types for secrets management operations.
// These errors are defined as variables to enable error comparison using errors.Is().
var (
	// ErrSecretNotFound indicates that the requested secret was not found
	// in the provider's storage. This could mean the secret doesn't exist,
	// has expired, or the user doesn't have access to it.
	ErrSecretNotFound = errors.New("secret not found")

	// ErrProviderError indicates a general error occurred within the provider
	// implementation. This could include network issues, authentication failures,
	// or provider-specific errors that don't fit other categories.
	ErrProviderError = errors.New("provider error")

	// ErrInvalidRef indicates that the provided SecretRef is malformed or
	// contains invalid values (e.g., empty path, invalid version format).
	ErrInvalidRef = errors.New("invalid secret reference")

	// ErrAccessDenied indicates that the operation was denied due to
	// insufficient permissions or authentication failures.
	ErrAccessDenied = errors.New("access denied")
)

// ProviderError wraps provider-specific errors with additional context.
// It preserves the original error while adding provider information for debugging.
type ProviderError struct {
	Provider string    // Name of the provider where the error occurred
	Ref      SecretRef // The secret reference that caused the error
	Err      error     // The underlying error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider %q error for secret %q: %v", e.Provider, e.Ref.Path, e.Err)
}

// Unwrap returns the underlying error for error chain traversal.
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new ProviderError with context.
// It wraps the original error while preserving provider and secret information.
func NewProviderError(provider string, ref SecretRef, err error) *ProviderError {
	return &ProviderError{
		Provider: provider,
		Ref:      ref,
		Err:      err,
	}
}

// IsProviderError checks if an error is a ProviderError or contains one in its chain.
func IsProviderError(err error) bool {
	var pe *ProviderError
	return errors.As(err, &pe)
}

// WrapProviderError wraps a provider error with additional context using fmt.Errorf.
// This follows Go best practices for error wrapping and preserves the error chain.
func WrapProviderError(provider string, ref SecretRef, err error, msg string) error {
	if err == nil {
		return nil
	}
	pe := NewProviderError(provider, ref, err)
	return fmt.Errorf("%s: %w", msg, pe)
}

// ValidationError represents validation failures for secret references or values.
type ValidationError struct {
	Field   string // The field that failed validation
	Value   string // The problematic value (sanitized, never contains secrets)
	Message string // Human-readable validation message
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf(
		"validation failed for field %q: %s (value: %q)",
		e.Field,
		e.Message,
		e.Value,
	)
}

// NewValidationError creates a new ValidationError for field validation failures.
func NewValidationError(field, value, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// IsValidationError checks if an error is a ValidationError or contains one in its chain.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}
