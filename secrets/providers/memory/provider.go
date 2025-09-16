// Package memory provides an in-memory secret provider for testing and development.
// It implements the full WriteableProvider interface with thread-safe operations.
package memory

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/input-output-hk/catalyst-forge-libs/secrets"
)

const (
	// latestVersion is the default version identifier used when no specific version is requested
	latestVersion = "latest"
)

// Provider implements an in-memory secret store for testing and development.
// It provides thread-safe access to secrets stored in memory with no persistence.
type Provider struct {
	// store holds the secrets keyed by path and version
	store map[string]map[string]*secrets.Secret
	// mu protects concurrent access to the store
	mu sync.RWMutex
}

// New creates a new memory provider instance.
// It initializes an empty secret store ready for use.
func New() *Provider {
	return &Provider{
		store: make(map[string]map[string]*secrets.Secret),
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "memory"
}

// HealthCheck verifies the provider is operational.
// For the memory provider, this always returns nil as it's always healthy.
func (p *Provider) HealthCheck(ctx context.Context) error {
	// Memory provider is always healthy
	return nil
}

// Close gracefully shuts down the provider and clears all stored secrets.
// This implements secure cleanup for the memory provider.
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear all secrets securely
	for path, versions := range p.store {
		for version, secret := range versions {
			secret.Clear()
			delete(versions, version)
		}
		delete(p.store, path)
	}

	return nil
}

// Resolve retrieves a single secret by reference.
// It supports version-specific resolution and context cancellation.
func (p *Provider) Resolve(ctx context.Context, ref secrets.SecretRef) (*secrets.Secret, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("resolve operation cancelled: %w", ctx.Err())
	default:
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	versions, exists := p.store[ref.Path]
	if !exists {
		return nil, fmt.Errorf("secret not found: %s", ref.Path)
	}

	version := ref.Version
	if version == "" {
		version = latestVersion
	}

	secret, exists := versions[version]
	if !exists {
		return nil, fmt.Errorf("secret version not found: %s@%s", ref.Path, version)
	}

	// Return a copy to prevent external modification
	return &secrets.Secret{
		Value:     append([]byte(nil), secret.Value...), // Copy bytes
		Version:   secret.Version,
		CreatedAt: secret.CreatedAt,
		ExpiresAt: secret.ExpiresAt,
		AutoClear: secret.AutoClear,
	}, nil
}

// ResolveBatch retrieves multiple secrets in a single operation.
// It returns a map of successfully resolved secrets, with missing secrets omitted.
func (p *Provider) ResolveBatch(
	ctx context.Context,
	refs []secrets.SecretRef,
) (map[string]*secrets.Secret, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("resolve batch operation cancelled: %w", ctx.Err())
	default:
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	results := make(map[string]*secrets.Secret)

	for _, ref := range refs {
		versions, exists := p.store[ref.Path]
		if !exists {
			continue // Skip missing secrets
		}

		version := ref.Version
		if version == "" {
			version = "latest"
		}

		secret, exists := versions[version]
		if !exists {
			continue // Skip missing versions
		}

		// Return a copy to prevent external modification
		results[ref.Path] = &secrets.Secret{
			Value:     append([]byte(nil), secret.Value...), // Copy bytes
			Version:   secret.Version,
			CreatedAt: secret.CreatedAt,
			ExpiresAt: secret.ExpiresAt,
			AutoClear: secret.AutoClear,
		}
	}

	return results, nil
}

// Exists checks if a secret exists without retrieving its value.
// It supports version-specific existence checks.
func (p *Provider) Exists(ctx context.Context, ref secrets.SecretRef) (bool, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return false, fmt.Errorf("exists operation cancelled: %w", ctx.Err())
	default:
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	versions, exists := p.store[ref.Path]
	if !exists {
		return false, nil
	}

	version := ref.Version
	if version == "" {
		version = latestVersion
	}

	_, exists = versions[version]
	return exists, nil
}

// Store saves a secret value to the provider.
// It generates a version if none is specified and creates the secret metadata.
func (p *Provider) Store(ctx context.Context, ref secrets.SecretRef, value []byte) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("store operation cancelled: %w", ctx.Err())
	default:
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Initialize path map if it doesn't exist
	if p.store[ref.Path] == nil {
		p.store[ref.Path] = make(map[string]*secrets.Secret)
	}

	version := ref.Version
	if version == "" {
		version = latestVersion
	}

	// Create the secret with metadata
	secret := &secrets.Secret{
		Value:     append([]byte(nil), value...), // Copy bytes
		Version:   version,
		CreatedAt: time.Now(),
		ExpiresAt: nil, // No expiration for memory provider
		AutoClear: false,
	}

	p.store[ref.Path][version] = secret
	return nil
}

// Delete removes a secret from the provider.
// It clears the secret value securely before removal.
func (p *Provider) Delete(ctx context.Context, ref secrets.SecretRef) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("delete operation cancelled: %w", ctx.Err())
	default:
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	versions, exists := p.store[ref.Path]
	if !exists {
		return fmt.Errorf("secret not found: %s", ref.Path)
	}

	version := ref.Version
	if version == "" {
		version = latestVersion
	}

	secret, exists := versions[version]
	if !exists {
		return fmt.Errorf("secret version not found: %s@%s", ref.Path, version)
	}

	// Securely clear the secret before deletion
	secret.Clear()
	delete(versions, version)

	// Clean up empty path maps
	if len(versions) == 0 {
		delete(p.store, ref.Path)
	}

	return nil
}

// Rotate creates a new version of the secret with random content.
// It preserves the old version and returns the new secret.
func (p *Provider) Rotate(ctx context.Context, ref secrets.SecretRef) (*secrets.Secret, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("rotate operation cancelled: %w", ctx.Err())
	default:
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	versions, exists := p.store[ref.Path]
	if !exists {
		return nil, fmt.Errorf("secret not found: %s", ref.Path)
	}

	// Get the original secret to determine size for new content
	var originalSize int
	if originalSecret, exists := versions["latest"]; exists {
		originalSize = len(originalSecret.Value)
	} else {
		// If no latest version, use a default size
		originalSize = 32
	}

	// Generate new random content
	newValue := make([]byte, originalSize)
	if _, err := rand.Read(newValue); err != nil {
		return nil, fmt.Errorf("failed to generate random content: %w", err)
	}

	// Generate new version (timestamp-based for uniqueness)
	newVersion := fmt.Sprintf("v%d", time.Now().UnixNano())

	// Create new secret
	newSecret := &secrets.Secret{
		Value:     newValue,
		Version:   newVersion,
		CreatedAt: time.Now(),
		ExpiresAt: nil,
		AutoClear: false,
	}

	// Initialize path map if needed
	if p.store[ref.Path] == nil {
		p.store[ref.Path] = make(map[string]*secrets.Secret)
	}

	p.store[ref.Path][newVersion] = newSecret

	// Return a copy
	return &secrets.Secret{
		Value:     append([]byte(nil), newSecret.Value...),
		Version:   newSecret.Version,
		CreatedAt: newSecret.CreatedAt,
		ExpiresAt: newSecret.ExpiresAt,
		AutoClear: newSecret.AutoClear,
	}, nil
}
