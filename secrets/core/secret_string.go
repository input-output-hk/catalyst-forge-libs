package core

import (
	"context"
	"fmt"
	"sync"
)

// SecretString encapsulates a secret reference along with the means to resolve it.
// It provides lazy resolution with one-time use semantics by default for enhanced security.
type SecretString struct {
	// ref holds the reference to the secret
	ref SecretRef

	// resolver is the interface used to resolve the secret
	resolver Resolver

	// oneTimeUse controls whether the secret is cleared after first use
	oneTimeUse bool

	// allowCache allows the resolved secret to be cached for multiple reads
	allowCache bool

	// resolved holds the cached resolved secret (if caching is enabled)
	resolved *Secret

	// consumed tracks whether the secret has been consumed (for one-time use)
	consumed bool

	// mu protects concurrent access to internal state
	mu sync.Mutex
}

// SecretStringOption is a functional option for configuring SecretString behavior.
type SecretStringOption func(*SecretString)

// WithOneTimeUse sets whether the secret should be consumed after first use.
// Default is true for security.
func WithOneTimeUse(oneTime bool) SecretStringOption {
	return func(s *SecretString) {
		s.oneTimeUse = oneTime
	}
}

// WithCaching enables caching of the resolved secret for multiple reads.
// This is only effective when one-time use is disabled.
// Default is false to minimize secret exposure.
func WithCaching(cache bool) SecretStringOption {
	return func(s *SecretString) {
		s.allowCache = cache
	}
}

// NewSecretString creates a new SecretString with the provided reference and resolver.
// By default, it enforces one-time use semantics for security.
func NewSecretString(ref SecretRef, resolver Resolver, opts ...SecretStringOption) *SecretString {
	if resolver == nil {
		// If no resolver provided, this SecretString will always fail to resolve
		resolver = &nilResolver{}
	}

	s := &SecretString{
		ref:        ref,
		resolver:   resolver,
		oneTimeUse: true,  // Default to one-time use for safety
		allowCache: false, // Default to no caching
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// NewSecretStringFromManager creates a SecretString that uses a Manager as its resolver.
// This is a convenience method for the common case of using the default provider.
func NewSecretStringFromManager(ref SecretRef, manager *Manager, opts ...SecretStringOption) *SecretString {
	return NewSecretString(ref, manager, opts...)
}

// Resolve retrieves the secret value, respecting one-time use semantics.
// For one-time use SecretStrings, this can only be called once successfully.
// Returns an error if the secret cannot be resolved or has already been consumed.
func (s *SecretString) Resolve(ctx context.Context) (*Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already consumed (for one-time use)
	if s.oneTimeUse && s.consumed {
		return nil, fmt.Errorf("secret has already been consumed (one-time use)")
	}

	// Check if we have a cached resolution
	if s.allowCache && s.resolved != nil {
		if s.oneTimeUse {
			s.consumed = true
			// Return the cached secret and clear our reference
			secret := s.resolved
			s.resolved = nil
			return secret, nil
		}
		// For non-one-time use with caching, return a copy to prevent external modification
		return s.copySecret(s.resolved), nil
	}

	// Resolve the secret
	secret, err := s.resolver.Resolve(ctx, s.ref)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve secret: %w", err)
	}

	// Handle caching if enabled and not one-time use
	if s.allowCache && !s.oneTimeUse {
		// Cache a copy of the resolved secret
		s.resolved = s.copySecret(secret)
	}

	// Mark as consumed if one-time use
	if s.oneTimeUse {
		s.consumed = true
	}

	return secret, nil
}

// String resolves and returns the secret as a string.
// For one-time use SecretStrings, this consumes the secret.
// The resolved secret's memory is automatically cleared if AutoClear is enabled.
func (s *SecretString) String(ctx context.Context) (string, error) {
	secret, err := s.Resolve(ctx)
	if err != nil {
		return "", err
	}

	// Use the secret's String() method which respects AutoClear
	return secret.String(), nil
}

// Bytes resolves and returns the secret as a byte slice.
// For one-time use SecretStrings, this consumes the secret.
// The resolved secret's memory is automatically cleared if AutoClear is enabled.
func (s *SecretString) Bytes(ctx context.Context) ([]byte, error) {
	secret, err := s.Resolve(ctx)
	if err != nil {
		return nil, err
	}

	// Use the secret's Bytes() method which respects AutoClear
	return secret.Bytes(), nil
}

// IsConsumed returns true if this is a one-time use SecretString that has been consumed.
func (s *SecretString) IsConsumed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.oneTimeUse && s.consumed
}

// Clear explicitly clears any cached secret value and marks the SecretString as consumed.
// This can be used to proactively clean up sensitive data.
func (s *SecretString) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.resolved != nil {
		s.resolved.Clear()
		s.resolved = nil
	}

	if s.oneTimeUse {
		s.consumed = true
	}
}

// Ref returns the SecretRef associated with this SecretString.
// This is safe to call multiple times and does not consume the secret.
func (s *SecretString) Ref() SecretRef {
	return s.ref
}

// copySecret creates a deep copy of a secret to prevent external modification.
func (s *SecretString) copySecret(original *Secret) *Secret {
	if original == nil {
		return nil
	}

	// Create a copy of the value
	value := make([]byte, len(original.Value))
	copy(value, original.Value)

	// Create a new secret with the copied value
	copied := &Secret{
		Value:     value,
		Version:   original.Version,
		CreatedAt: original.CreatedAt,
		ExpiresAt: original.ExpiresAt,
		AutoClear: original.AutoClear,
	}

	return copied
}

// nilResolver is a resolver that always returns an error.
// Used when no resolver is provided to SecretString.
type nilResolver struct{}

func (n *nilResolver) Resolve(ctx context.Context, ref SecretRef) (*Secret, error) {
	return nil, fmt.Errorf("no resolver configured")
}

func (n *nilResolver) ResolveBatch(ctx context.Context, refs []SecretRef) (map[string]*Secret, error) {
	return nil, fmt.Errorf("no resolver configured")
}

func (n *nilResolver) Exists(ctx context.Context, ref SecretRef) (bool, error) {
	return false, fmt.Errorf("no resolver configured")
}
