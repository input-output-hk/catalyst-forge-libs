package secrets

import "context"

// Resolver defines the core interface for secret resolution.
// Implementations provide the ability to fetch secrets from various backends.
type Resolver interface {
	// Resolve retrieves a single secret by reference.
	// Returns the resolved secret or an error if resolution fails.
	Resolve(ctx context.Context, ref SecretRef) (*Secret, error)

	// ResolveBatch retrieves multiple secrets in a single operation.
	// Returns a map of secret paths to resolved secrets.
	// Missing secrets should not cause the entire operation to fail.
	ResolveBatch(ctx context.Context, refs []SecretRef) (map[string]*Secret, error)

	// Exists checks if a secret exists without retrieving its value.
	// Returns true if the secret exists, false otherwise.
	Exists(ctx context.Context, ref SecretRef) (bool, error)
}

// Provider extends Resolver with provider management capabilities.
// All secret providers must implement this interface.
type Provider interface {
	Resolver

	// Name returns the provider's identifier (e.g., "aws", "vault", "memory").
	Name() string

	// HealthCheck verifies the provider's connectivity and functionality.
	// Returns nil if healthy, error otherwise.
	HealthCheck(ctx context.Context) error

	// Close gracefully shuts down the provider and releases resources.
	Close() error
}

// WriteableProvider extends Provider with write operations.
// Providers supporting mutation operations must implement this interface.
type WriteableProvider interface {
	Provider

	// Store saves a secret value to the provider.
	Store(ctx context.Context, ref SecretRef, value []byte) error

	// Delete removes a secret from the provider.
	Delete(ctx context.Context, ref SecretRef) error

	// Rotate creates a new version of the secret and returns it.
	// The old version should remain accessible until cleanup.
	Rotate(ctx context.Context, ref SecretRef) (*Secret, error)
}
