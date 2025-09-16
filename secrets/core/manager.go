// Package core provides secure, provider-agnostic secrets management
// with automatic memory cleanup and just-in-time resolution.
package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Config holds the configuration for the Manager.
type Config struct {
	// DefaultProvider is the name of the default provider to use for resolution.
	DefaultProvider string

	// AutoClear controls whether resolved secrets should automatically clear
	// their memory after use (String(), Bytes(), UnmarshalJSON()).
	AutoClear bool

	// CacheTTL controls how long to cache resolved secrets.
	// A value of 0 disables caching. This will be implemented in Phase 7.
	CacheTTL time.Duration

	// EnableAudit controls whether audit logging is enabled.
	EnableAudit bool

	// AuditLogger is the audit logger to use when audit is enabled.
	// Can be nil if audit logging is disabled.
	AuditLogger AuditLogger
}

// Manager orchestrates secret resolution across multiple providers.
// It maintains a registry of providers and handles provider selection,
// configuration management, and graceful shutdown.
type Manager struct {
	// providers holds the registered providers indexed by name.
	providers map[string]Provider

	// defaultProvider is the name of the default provider to use.
	defaultProvider string

	// autoClear controls whether resolved secrets should auto-clear.
	autoClear bool

	// enableAudit controls whether audit logging is enabled.
	enableAudit bool

	// auditLogger is the audit logger to use (can be nil).
	auditLogger AuditLogger

	// mu protects concurrent access to the provider registry.
	mu sync.RWMutex
}

// NewManager creates a new Manager with the provided configuration.
// It initializes the provider registry and sets defaults from the config.
func NewManager(config *Config) *Manager {
	if config == nil {
		config = &Config{}
	}

	manager := &Manager{
		providers:       make(map[string]Provider),
		defaultProvider: config.DefaultProvider,
		autoClear:       config.AutoClear,
		enableAudit:     config.EnableAudit,
		auditLogger:     config.AuditLogger,
	}

	return manager
}

// RegisterProvider adds a provider to the manager's registry.
// It validates that no provider with the same name is already registered.
// Returns an error if a provider with the same name already exists.
func (m *Manager) RegisterProvider(name string, provider Provider) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.providers[name]; exists {
		return fmt.Errorf("provider with name %q already registered", name)
	}

	m.providers[name] = provider
	return nil
}

// Resolve resolves a secret using the default provider.
// It passes through to the provider's Resolve method and handles audit logging.
// Returns an error if no default provider is configured or if resolution fails.
func (m *Manager) Resolve(ctx context.Context, ref SecretRef) (*Secret, error) {
	if m.defaultProvider == "" {
		return nil, fmt.Errorf("no default provider configured")
	}

	return m.ResolveFrom(ctx, m.defaultProvider, ref)
}

// ResolveFrom resolves a secret using a specific provider.
// It validates the provider exists, passes through to the provider's Resolve method,
// and handles audit logging.
// Returns an error if the provider doesn't exist or if resolution fails.
func (m *Manager) ResolveFrom(
	ctx context.Context,
	providerName string,
	ref SecretRef,
) (*Secret, error) {
	if providerName == "" {
		return nil, fmt.Errorf("provider name cannot be empty")
	}

	m.mu.RLock()
	provider, exists := m.providers[providerName]
	m.mu.RUnlock()

	if !exists {
		if m.enableAudit && m.auditLogger != nil {
			m.auditLogger.LogAccess(
				ctx,
				"resolve",
				ref,
				false,
				fmt.Errorf("provider %q not found", providerName),
			)
		}
		return nil, fmt.Errorf("provider %q not found", providerName)
	}

	secret, err := provider.Resolve(ctx, ref)

	// Handle audit logging
	if m.enableAudit && m.auditLogger != nil {
		success := err == nil
		m.auditLogger.LogAccess(ctx, "resolve", ref, success, err)
	}

	// Apply AutoClear setting from manager config if secret was successfully resolved
	if err == nil && secret != nil {
		secret.AutoClear = m.autoClear
	}

	// Wrap provider errors with context
	if err != nil {
		return nil, WrapProviderError(providerName, ref, err, "failed to resolve secret")
	}

	return secret, nil
}

// ResolveBatch resolves multiple secrets using the default provider.
// It passes through to the provider's ResolveBatch method.
// Returns a map of successfully resolved secrets.
func (m *Manager) ResolveBatch(ctx context.Context, refs []SecretRef) (map[string]*Secret, error) {
	if m.defaultProvider == "" {
		return nil, fmt.Errorf("no default provider configured")
	}

	return m.ResolveBatchFrom(ctx, m.defaultProvider, refs)
}

// ResolveBatchFrom resolves multiple secrets using a specific provider.
// It validates the provider exists and passes through to the provider's ResolveBatch method.
// Returns a map of successfully resolved secrets.
func (m *Manager) ResolveBatchFrom(
	ctx context.Context,
	providerName string,
	refs []SecretRef,
) (map[string]*Secret, error) {
	if providerName == "" {
		return nil, fmt.Errorf("provider name cannot be empty")
	}

	m.mu.RLock()
	provider, exists := m.providers[providerName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("provider %q not found", providerName)
	}

	results, err := provider.ResolveBatch(ctx, refs)
	if err != nil {
		// For batch operations, we can't associate with a specific ref, so use a generic one
		genericRef := SecretRef{Path: "batch-operation"}
		return nil, WrapProviderError(providerName, genericRef, err, "failed to resolve batch")
	}

	// Apply AutoClear setting to all resolved secrets
	for _, secret := range results {
		if secret != nil {
			secret.AutoClear = m.autoClear
		}
	}

	return results, nil
}

// Exists checks if a secret exists using the default provider.
// It passes through to the provider's Exists method.
// Returns true if the secret exists, false otherwise.
func (m *Manager) Exists(ctx context.Context, ref SecretRef) (bool, error) {
	if m.defaultProvider == "" {
		return false, fmt.Errorf("no default provider configured")
	}

	return m.ExistsFrom(ctx, m.defaultProvider, ref)
}

// ExistsFrom checks if a secret exists using a specific provider.
// It validates the provider exists and passes through to the provider's Exists method.
// Returns true if the secret exists, false otherwise.
func (m *Manager) ExistsFrom(
	ctx context.Context,
	providerName string,
	ref SecretRef,
) (bool, error) {
	if providerName == "" {
		return false, fmt.Errorf("provider name cannot be empty")
	}

	m.mu.RLock()
	provider, exists := m.providers[providerName]
	m.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("provider %q not found", providerName)
	}

	exists, err := provider.Exists(ctx, ref)
	if err != nil {
		return false, WrapProviderError(providerName, ref, err, "failed to check existence")
	}

	return exists, nil
}

// Close gracefully shuts down all registered providers.
// It calls Close() on each provider and aggregates any errors.
// Returns nil if all providers closed successfully, or an aggregated error.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for name, provider := range m.providers {
		if err := provider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close provider %q: %w", name, err))
		}
	}

	// Clear the provider registry
	m.providers = make(map[string]Provider)

	if len(errs) == 0 {
		return nil
	}

	// Return aggregated error
	return fmt.Errorf("errors during shutdown: %v", errs)
}

// WriteableManager extends Manager with write operations for secrets.
// It provides Store and Delete operations for providers that support them.
type WriteableManager struct {
	*Manager
}

// NewWriteableManager creates a new WriteableManager with the provided configuration.
func NewWriteableManager(config *Config) *WriteableManager {
	return &WriteableManager{
		Manager: NewManager(config),
	}
}

// Store saves a secret value using the default provider.
// Returns an error if the provider doesn't support write operations.
func (m *WriteableManager) Store(ctx context.Context, ref SecretRef, value []byte) error {
	if m.defaultProvider == "" {
		return fmt.Errorf("no default provider configured")
	}

	return m.StoreIn(ctx, m.defaultProvider, ref, value)
}

// StoreIn saves a secret value using a specific provider.
// Returns an error if the provider doesn't exist or doesn't support write operations.
func (m *WriteableManager) StoreIn(
	ctx context.Context,
	providerName string,
	ref SecretRef,
	value []byte,
) error {
	if providerName == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	m.mu.RLock()
	provider, exists := m.providers[providerName]
	m.mu.RUnlock()

	if !exists {
		if m.enableAudit && m.auditLogger != nil {
			m.auditLogger.LogAccess(
				ctx,
				"store",
				ref,
				false,
				fmt.Errorf("provider %q not found", providerName),
			)
		}
		return fmt.Errorf("provider %q not found", providerName)
	}

	// Check if provider supports write operations
	writeableProvider, ok := provider.(WriteableProvider)
	if !ok {
		err := fmt.Errorf("provider %q does not support write operations", providerName)
		if m.enableAudit && m.auditLogger != nil {
			m.auditLogger.LogAccess(ctx, "store", ref, false, err)
		}
		return err
	}

	err := writeableProvider.Store(ctx, ref, value)

	// Handle audit logging
	if m.enableAudit && m.auditLogger != nil {
		success := err == nil
		m.auditLogger.LogAccess(ctx, "store", ref, success, err)
	}

	if err != nil {
		return WrapProviderError(providerName, ref, err, "failed to store secret")
	}

	return nil
}

// Delete removes a secret using the default provider.
// Returns an error if the provider doesn't support write operations.
func (m *WriteableManager) Delete(ctx context.Context, ref SecretRef) error {
	if m.defaultProvider == "" {
		return fmt.Errorf("no default provider configured")
	}

	return m.DeleteFrom(ctx, m.defaultProvider, ref)
}

// DeleteFrom removes a secret using a specific provider.
// Returns an error if the provider doesn't exist or doesn't support write operations.
func (m *WriteableManager) DeleteFrom(
	ctx context.Context,
	providerName string,
	ref SecretRef,
) error {
	if providerName == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	m.mu.RLock()
	provider, exists := m.providers[providerName]
	m.mu.RUnlock()

	if !exists {
		if m.enableAudit && m.auditLogger != nil {
			m.auditLogger.LogAccess(
				ctx,
				"delete",
				ref,
				false,
				fmt.Errorf("provider %q not found", providerName),
			)
		}
		return fmt.Errorf("provider %q not found", providerName)
	}

	// Check if provider supports write operations
	writeableProvider, ok := provider.(WriteableProvider)
	if !ok {
		err := fmt.Errorf("provider %q does not support write operations", providerName)
		if m.enableAudit && m.auditLogger != nil {
			m.auditLogger.LogAccess(ctx, "delete", ref, false, err)
		}
		return err
	}

	err := writeableProvider.Delete(ctx, ref)

	// Handle audit logging
	if m.enableAudit && m.auditLogger != nil {
		success := err == nil
		m.auditLogger.LogAccess(ctx, "delete", ref, success, err)
	}

	if err != nil {
		return WrapProviderError(providerName, ref, err, "failed to delete secret")
	}

	return nil
}

// RotatableManager extends WriteableManager with rotation capabilities for secrets.
// It provides Rotate operations for providers that support them.
type RotatableManager struct {
	*WriteableManager
}

// NewRotatableManager creates a new RotatableManager with the provided configuration.
func NewRotatableManager(config *Config) *RotatableManager {
	return &RotatableManager{
		WriteableManager: NewWriteableManager(config),
	}
}

// Rotate creates a new version of a secret using the default provider.
// Returns the new secret or an error if the provider doesn't support rotation.
func (m *RotatableManager) Rotate(ctx context.Context, ref SecretRef) (*Secret, error) {
	if m.defaultProvider == "" {
		return nil, fmt.Errorf("no default provider configured")
	}

	return m.RotateIn(ctx, m.defaultProvider, ref)
}

// RotateIn creates a new version of a secret using a specific provider.
// Returns the new secret or an error if the provider doesn't exist or doesn't support rotation.
func (m *RotatableManager) RotateIn(
	ctx context.Context,
	providerName string,
	ref SecretRef,
) (*Secret, error) {
	if providerName == "" {
		return nil, fmt.Errorf("provider name cannot be empty")
	}

	m.mu.RLock()
	provider, exists := m.providers[providerName]
	m.mu.RUnlock()

	if !exists {
		if m.enableAudit && m.auditLogger != nil {
			m.auditLogger.LogAccess(
				ctx,
				"rotate",
				ref,
				false,
				fmt.Errorf("provider %q not found", providerName),
			)
		}
		return nil, fmt.Errorf("provider %q not found", providerName)
	}

	// Check if provider supports rotation operations
	rotatableProvider, ok := provider.(RotatableProvider)
	if !ok {
		err := fmt.Errorf("provider %q does not support rotation operations", providerName)
		if m.enableAudit && m.auditLogger != nil {
			m.auditLogger.LogAccess(ctx, "rotate", ref, false, err)
		}
		return nil, err
	}

	secret, err := rotatableProvider.Rotate(ctx, ref)

	// Handle audit logging
	if m.enableAudit && m.auditLogger != nil {
		success := err == nil
		m.auditLogger.LogAccess(ctx, "rotate", ref, success, err)
	}

	// Apply AutoClear setting from manager config if secret was successfully rotated
	if err == nil && secret != nil {
		secret.AutoClear = m.autoClear
	}

	if err != nil {
		return nil, WrapProviderError(providerName, ref, err, "failed to rotate secret")
	}

	return secret, nil
}
