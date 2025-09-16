// Package secrets provides secure, provider-agnostic secrets management
// with automatic memory cleanup and just-in-time resolution.
//
// # Basic Usage
//
// Set up a manager with a memory provider for testing:
//
//	import "github.com/input-output-hk/catalyst-forge-libs/secrets/providers/memory"
//
//	config := &secrets.Config{
//		DefaultProvider: "memory",
//		AutoClear:      true, // Enable automatic cleanup
//	}
//
//	manager := secrets.NewManager(config)
//	defer manager.Close()
//
//	provider := memory.New()
//	manager.RegisterProvider("memory", provider)
//
// # Storing and Retrieving Secrets
//
// Store a secret:
//
//	ref := secrets.SecretRef{
//		Path:    "database/password",
//		Version: "v1.0",
//	}
//	err := provider.Store(ctx, ref, []byte("my-secret-password"))
//
// Retrieve a secret:
//
//	secret, err := manager.Resolve(ctx, ref)
//	password := secret.String() // Auto-cleared if AutoClear is enabled
//
// # Provider-Agnostic Design
//
// The same API works with different providers:
//
//	// Memory provider for testing
//	memoryProvider := memory.New()
//	manager.RegisterProvider("memory", memoryProvider)
//
//	// AWS provider for production (conceptual)
//	// awsProvider := aws.New()
//	// manager.RegisterProvider("aws", awsProvider)
//
//	// Resolve works the same way regardless of provider
//	secret, _ := manager.Resolve(ctx, ref)
//
// # Security Features
//
// - Automatic memory zeroing after use
// - Copy-on-read prevents external modification
// - Just-in-time resolution minimizes exposure
// - Provider-agnostic interface for multiple backends
//
// # Error Handling
//
// The package provides structured error types:
//
//	if errors.Is(err, secrets.ErrSecretNotFound) {
//		// Handle missing secret
//	}
//	if secrets.IsProviderError(err) {
//		// Handle provider-specific error
//	}
//
// # Context Support
//
// All operations respect context cancellation:
//
//	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
//	defer cancel()
//	secret, err := manager.Resolve(ctx, ref) // Will cancel after 5 seconds
package secrets

import (
	"encoding/json"
	"fmt"
	"time"
)

// Secret represents a resolved secret value with metadata.
// It provides secure handling of sensitive data with automatic cleanup capabilities.
type Secret struct {
	// Value contains the secret data as bytes. This should never be logged or exposed.
	Value []byte
	// Version indicates the version of this secret (useful for rotation tracking).
	Version string
	// CreatedAt records when this secret was created.
	CreatedAt time.Time
	// ExpiresAt indicates when this secret expires (nil means no expiration).
	ExpiresAt *time.Time
	// AutoClear controls whether methods automatically clear memory after use.
	AutoClear bool
}

// SecretRef represents a reference to a secret without containing the actual value.
// This is used for secure configuration where secrets are resolved on-demand.
type SecretRef struct {
	// Path identifies the secret location (e.g., "db/password", "api/key").
	Path string
	// Version specifies which version of the secret to retrieve (empty for latest).
	Version string
	// Metadata contains additional provider-specific information.
	Metadata map[string]string
}

// String returns the secret value as a string.
// If AutoClear is enabled, the secret value is cleared after use.
// Returns a string copy to prevent external modification.
func (s *Secret) String() string {
	if s.Value == nil {
		return ""
	}

	value := string(s.Value)

	if s.AutoClear {
		s.Clear()
	}

	return value
}

// Bytes returns a copy of the secret value.
// If AutoClear is enabled, the secret value is cleared after use.
// Returns a byte slice copy to prevent external modification.
func (s *Secret) Bytes() []byte {
	if s.Value == nil {
		return nil
	}

	// Create a copy to prevent external modification
	value := make([]byte, len(s.Value))
	copy(value, s.Value)

	if s.AutoClear {
		s.Clear()
	}

	return value
}

// UnmarshalJSON implements json.Unmarshaler for Secret.
// It expects JSON bytes representing a Secret structure and unmarshals them.
// If AutoClear is enabled, the secret value is cleared after unmarshaling.
func (s *Secret) UnmarshalJSON(data []byte) error {
	// Create a temporary struct to unmarshal into
	var temp struct {
		Value     []byte     `json:"value"`
		Version   string     `json:"version"`
		CreatedAt time.Time  `json:"created_at"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
		AutoClear bool       `json:"auto_clear"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("failed to unmarshal secret: %w", err)
	}

	// Set the values on the secret
	s.Value = temp.Value
	s.Version = temp.Version
	s.CreatedAt = temp.CreatedAt
	s.ExpiresAt = temp.ExpiresAt
	s.AutoClear = temp.AutoClear

	// If AutoClear is enabled, clear the value after unmarshaling
	if s.AutoClear {
		s.Clear()
	}

	return nil
}

// Clear explicitly zeros out the secret value in memory to prevent
// sensitive data from remaining in memory after use.
// This implements secure memory cleanup.
func (s *Secret) Clear() {
	if s.Value != nil {
		// Zero out the memory to prevent sensitive data from remaining
		for i := range s.Value {
			s.Value[i] = 0
		}
		// Clear the slice reference
		s.Value = nil
	}
}
