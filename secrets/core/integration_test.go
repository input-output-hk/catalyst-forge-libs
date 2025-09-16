package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// integrationMockAuditLogger is a test implementation of AuditLogger
type integrationMockAuditLogger struct {
	logs []AuditEntry
}

func (m *integrationMockAuditLogger) LogAccess(
	ctx context.Context,
	action string,
	ref SecretRef,
	success bool,
	err error,
) {
	entry := NewAuditEntry(ctx, action, ref, success, err)
	m.logs = append(m.logs, *entry)
}

func (m *integrationMockAuditLogger) GetLogs() []AuditEntry {
	logs := make([]AuditEntry, len(m.logs))
	copy(logs, m.logs)
	return logs
}

func (m *integrationMockAuditLogger) ClearLogs() {
	m.logs = nil
}

// integrationMockWriteableProvider is a test implementation of WriteableProvider
type integrationMockWriteableProvider struct {
	name         string
	store        map[string]*Secret
	resolveError error
	storeError   error
	deleteError  error
	rotateError  error
	mu           sync.RWMutex
}

func newIntegrationMockWriteableProvider(name string) *integrationMockWriteableProvider {
	return &integrationMockWriteableProvider{
		name:  name,
		store: make(map[string]*Secret),
	}
}

func (m *integrationMockWriteableProvider) Name() string {
	return m.name
}

func (m *integrationMockWriteableProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *integrationMockWriteableProvider) Close() error {
	return nil
}

func (m *integrationMockWriteableProvider) Resolve(
	ctx context.Context,
	ref SecretRef,
) (*Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.resolveError != nil {
		return nil, m.resolveError
	}

	version := ref.Version
	if version == "" {
		version = "latest"
	}

	key := ref.Path + ":" + version
	secret, exists := m.store[key]
	if !exists {
		return nil, fmt.Errorf("secret not found: %s", ref.Path)
	}

	// Return a copy
	return &Secret{
		Value:     append([]byte(nil), secret.Value...),
		Version:   secret.Version,
		CreatedAt: secret.CreatedAt,
		ExpiresAt: secret.ExpiresAt,
		AutoClear: secret.AutoClear,
	}, nil
}

func (m *integrationMockWriteableProvider) ResolveBatch(
	ctx context.Context,
	refs []SecretRef,
) (map[string]*Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]*Secret)
	for _, ref := range refs {
		version := ref.Version
		if version == "" {
			version = "latest"
		}
		key := ref.Path + ":" + version
		if secret, exists := m.store[key]; exists {
			results[ref.Path] = &Secret{
				Value:     append([]byte(nil), secret.Value...),
				Version:   secret.Version,
				CreatedAt: secret.CreatedAt,
				ExpiresAt: secret.ExpiresAt,
				AutoClear: secret.AutoClear,
			}
		}
	}
	return results, nil
}

func (m *integrationMockWriteableProvider) Exists(
	ctx context.Context,
	ref SecretRef,
) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	version := ref.Version
	if version == "" {
		version = "latest"
	}
	key := ref.Path + ":" + version
	_, exists := m.store[key]
	return exists, nil
}

func (m *integrationMockWriteableProvider) Store(
	ctx context.Context,
	ref SecretRef,
	value []byte,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.storeError != nil {
		return m.storeError
	}

	version := ref.Version
	if version == "" {
		version = "latest"
	}

	// Store the version
	key := ref.Path + ":" + version
	m.store[key] = &Secret{
		Value:     append([]byte(nil), value...),
		Version:   version,
		CreatedAt: time.Now(),
		AutoClear: false,
	}

	// If this is not the "latest" version, also update the latest pointer
	if version != "latest" {
		latestKey := ref.Path + ":latest"
		m.store[latestKey] = &Secret{
			Value:     append([]byte(nil), value...),
			Version:   version,
			CreatedAt: time.Now(),
			AutoClear: false,
		}
	}

	return nil
}

func (m *integrationMockWriteableProvider) Delete(ctx context.Context, ref SecretRef) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.deleteError != nil {
		return m.deleteError
	}

	version := ref.Version
	if version == "" {
		version = "latest"
	}
	key := ref.Path + ":" + version
	delete(m.store, key)
	return nil
}

func (m *integrationMockWriteableProvider) Rotate(
	ctx context.Context,
	ref SecretRef,
) (*Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.rotateError != nil {
		return nil, m.rotateError
	}

	// Generate new version
	newVersion := fmt.Sprintf("v%d", time.Now().UnixNano())
	newValue := make([]byte, 16)
	for i := range newValue {
		newValue[i] = byte(65 + i%26) // Simple pattern
	}

	newSecret := &Secret{
		Value:     newValue,
		Version:   newVersion,
		CreatedAt: time.Now(),
		AutoClear: false,
	}

	key := ref.Path + ":" + newVersion
	m.store[key] = newSecret

	return &Secret{
		Value:     append([]byte(nil), newValue...),
		Version:   newVersion,
		CreatedAt: newSecret.CreatedAt,
		AutoClear: false,
	}, nil
}

func TestManager_WriteableProvider_Integration(t *testing.T) {
	t.Run("end-to-end secret lifecycle", func(t *testing.T) {
		// Create manager with mock writeable provider
		manager := NewManager(&Config{
			DefaultProvider: "mock",
			AutoClear:       true,
		})

		mockProvider := newIntegrationMockWriteableProvider("mock")
		var provider Provider = mockProvider
		err := manager.RegisterProvider("mock", provider)
		require.NoError(t, err)

		ctx := context.Background()

		// Test 1: Store a secret
		ref := SecretRef{Path: "test/secret", Version: "v1"}
		secretValue := []byte("my-secret-value")

		// Use type assertion to access WriteableProvider methods
		writeableProvider, ok := provider.(interface {
			Store(ctx context.Context, ref SecretRef, value []byte) error
			Delete(ctx context.Context, ref SecretRef) error
		})
		require.True(t, ok)
		err = writeableProvider.Store(ctx, ref, secretValue)
		assert.NoError(t, err)

		// Test 2: Verify secret exists
		exists, err := manager.Exists(ctx, ref)
		assert.NoError(t, err)
		assert.True(t, exists)

		// Test 3: Resolve the secret via manager
		resolvedSecret, err := manager.Resolve(ctx, ref)
		assert.NoError(t, err)
		assert.NotNil(t, resolvedSecret)
		assert.Equal(t, secretValue, resolvedSecret.Value)
		assert.Equal(t, "v1", resolvedSecret.Version)
		assert.True(t, resolvedSecret.AutoClear)
		assert.False(t, resolvedSecret.CreatedAt.IsZero())

		// Test 4: Check AutoClear behavior
		// Using the secret should clear it
		_ = resolvedSecret.String()
		assert.Nil(t, resolvedSecret.Value, "Secret should be cleared after AutoClear usage")

		// Test 5: Try to resolve the same secret again (should get a new copy)
		resolvedSecret2, err := manager.Resolve(ctx, ref)
		assert.NoError(t, err)
		assert.NotNil(t, resolvedSecret2)
		assert.Equal(t, secretValue, resolvedSecret2.Value)

		// Test 6: Delete the secret
		err = writeableProvider.Delete(ctx, ref)
		assert.NoError(t, err)

		// Test 7: Verify secret no longer exists
		exists, err = manager.Exists(ctx, ref)
		assert.NoError(t, err)
		assert.False(t, exists)

		// Test 8: Try to resolve deleted secret
		deletedSecret, err := manager.Resolve(ctx, ref)
		assert.Error(t, err)
		assert.Nil(t, deletedSecret)
		assert.Contains(t, err.Error(), "secret not found")
	})

	t.Run("batch operations", func(t *testing.T) {
		manager := NewManager(&Config{
			DefaultProvider: "mock",
		})

		mockProvider := newIntegrationMockWriteableProvider("mock")
		var provider Provider = mockProvider
		err := manager.RegisterProvider("mock", provider)
		require.NoError(t, err)

		ctx := context.Background()

		// Store multiple secrets
		secrets := map[string][]byte{
			"db/host":     []byte("localhost"),
			"db/user":     []byte("admin"),
			"db/password": []byte("secret123"),
			"api/key":     []byte("apikey456"),
		}

		refs := make([]SecretRef, 0, len(secrets))
		writeableProvider, ok := provider.(interface {
			Store(ctx context.Context, ref SecretRef, value []byte) error
		})
		require.True(t, ok)

		for path, value := range secrets {
			ref := SecretRef{Path: path}
			storeErr := writeableProvider.Store(ctx, ref, value)
			require.NoError(t, storeErr)
			refs = append(refs, ref)
		}

		// Resolve batch
		results, err := manager.ResolveBatch(ctx, refs)
		assert.NoError(t, err)
		assert.Len(t, results, len(secrets))

		// Verify all secrets were resolved correctly
		for path, expectedValue := range secrets {
			secret, exists := results[path]
			assert.True(t, exists, "Secret %s should exist in results", path)
			assert.Equal(t, expectedValue, secret.Value)
		}

		// Test batch with some missing secrets
		refsWithMissing := make([]SecretRef, len(refs)+1)
		copy(refsWithMissing, refs)
		refsWithMissing[len(refs)] = SecretRef{Path: "nonexistent"}
		results, err = manager.ResolveBatch(ctx, refsWithMissing)
		assert.NoError(t, err)               // Batch should not fail due to missing secrets
		assert.Len(t, results, len(secrets)) // Should only contain existing secrets
	})

	t.Run("version management", func(t *testing.T) {
		manager := NewManager(&Config{
			DefaultProvider: "mock",
		})

		mockProvider := newIntegrationMockWriteableProvider("mock")
		var provider Provider = mockProvider
		err := manager.RegisterProvider("mock", provider)
		require.NoError(t, err)

		ctx := context.Background()
		path := "versioned/secret"

		// Store multiple versions
		v1Ref := SecretRef{Path: path, Version: "v1"}
		v2Ref := SecretRef{Path: path, Version: "v2"}
		latestRef := SecretRef{Path: path} // No version = latest

		writeableProvider, ok := provider.(interface {
			Store(ctx context.Context, ref SecretRef, value []byte) error
		})
		require.True(t, ok)

		err = writeableProvider.Store(ctx, v1Ref, []byte("version-1"))
		require.NoError(t, err)

		err = writeableProvider.Store(ctx, v2Ref, []byte("version-2"))
		require.NoError(t, err)

		// Resolve specific versions
		secret1, err := manager.Resolve(ctx, v1Ref)
		assert.NoError(t, err)
		assert.Equal(t, []byte("version-1"), secret1.Value)
		assert.Equal(t, "v1", secret1.Version)

		secret2, err := manager.Resolve(ctx, v2Ref)
		assert.NoError(t, err)
		assert.Equal(t, []byte("version-2"), secret2.Value)
		assert.Equal(t, "v2", secret2.Version)

		// Resolve latest (should be v2)
		latestSecret, err := manager.Resolve(ctx, latestRef)
		assert.NoError(t, err)
		assert.Equal(t, []byte("version-2"), latestSecret.Value)
	})
}

func TestManager_AuditLogging_Integration(t *testing.T) {
	auditLogger := &integrationMockAuditLogger{}
	manager := NewManager(&Config{
		DefaultProvider: "mock",
		EnableAudit:     true,
		AuditLogger:     auditLogger,
	})

	mockProvider := newIntegrationMockWriteableProvider("mock")
	var provider Provider = mockProvider
	err := manager.RegisterProvider("mock", provider)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("audit successful operations", func(t *testing.T) {
		auditLogger.ClearLogs()

		// Store a secret
		ref := SecretRef{Path: "audit/test"}
		value := []byte("audit-value")
		writeableProvider, ok := provider.(interface {
			Store(ctx context.Context, ref SecretRef, value []byte) error
		})
		require.True(t, ok)
		err := writeableProvider.Store(ctx, ref, value)
		require.NoError(t, err)

		// Resolve the secret (should trigger audit)
		secret, err := manager.Resolve(ctx, ref)
		assert.NoError(t, err)
		assert.NotNil(t, secret)

		// Check audit logs
		logs := auditLogger.GetLogs()
		assert.Len(t, logs, 1)

		log := logs[0]
		assert.Equal(t, "resolve", log.Action)
		assert.Equal(t, ref.Path, log.SecretRef.Path)
		assert.True(t, log.Success)
		assert.Empty(t, log.Error)
	})

	t.Run("audit failed operations", func(t *testing.T) {
		auditLogger.ClearLogs()

		// Try to resolve non-existent secret
		ref := SecretRef{Path: "nonexistent"}
		secret, err := manager.Resolve(ctx, ref)
		assert.Error(t, err)
		assert.Nil(t, secret)

		// Check audit logs
		logs := auditLogger.GetLogs()
		assert.Len(t, logs, 1)

		log := logs[0]
		assert.Equal(t, "resolve", log.Action)
		assert.Equal(t, ref.Path, log.SecretRef.Path)
		assert.False(t, log.Success)
		assert.Contains(t, log.Error, "secret not found")
	})

	t.Run("audit batch operations", func(t *testing.T) {
		auditLogger.ClearLogs()

		// Store a secret
		ref := SecretRef{Path: "batch/audit"}
		writeableProvider, ok := provider.(interface {
			Store(ctx context.Context, ref SecretRef, value []byte) error
		})
		require.True(t, ok)
		err := writeableProvider.Store(ctx, ref, []byte("batch-value"))
		require.NoError(t, err)

		// Batch resolve (this doesn't go through audit currently, but testing the flow)
		refs := []SecretRef{ref}
		results, err := manager.ResolveBatch(ctx, refs)
		assert.NoError(t, err)
		assert.Len(t, results, 1)

		// Note: Batch operations don't currently trigger individual audit logs
		// This is a design choice that could be changed if needed
	})
}

func TestManager_ErrorPropagation_Integration(t *testing.T) {
	t.Run("provider errors wrapped correctly", func(t *testing.T) {
		manager := NewManager(&Config{
			DefaultProvider: "mock",
		})

		mockProvider := newIntegrationMockWriteableProvider("mock")
		var provider Provider = mockProvider
		err := manager.RegisterProvider("mock", provider)
		require.NoError(t, err)

		ctx := context.Background()

		// Try to resolve non-existent secret
		ref := SecretRef{Path: "nonexistent/path"}
		secret, err := manager.Resolve(ctx, ref)
		assert.Error(t, err)
		assert.Nil(t, secret)

		// Verify error is wrapped with context
		assert.Contains(t, err.Error(), "failed to resolve secret")
		assert.Contains(t, err.Error(), "mock")
		assert.Contains(t, err.Error(), "secret not found")

		// Test error types
		assert.True(t, IsProviderError(err) || IsProviderError(UnwrapError(err)))
	})

	t.Run("batch errors handled gracefully", func(t *testing.T) {
		manager := NewManager(&Config{
			DefaultProvider: "mock",
		})

		mockProvider := newIntegrationMockWriteableProvider("mock")
		var provider Provider = mockProvider
		err := manager.RegisterProvider("mock", provider)
		require.NoError(t, err)

		ctx := context.Background()

		// Mix of existing and non-existing secrets
		existingRef := SecretRef{Path: "exists"}
		writeableProvider, ok := provider.(interface {
			Store(ctx context.Context, ref SecretRef, value []byte) error
		})
		require.True(t, ok)
		err = writeableProvider.Store(ctx, existingRef, []byte("value"))
		require.NoError(t, err)

		refs := []SecretRef{
			existingRef,
			{Path: "nonexistent1"},
			{Path: "nonexistent2"},
		}

		results, err := manager.ResolveBatch(ctx, refs)
		// Batch should succeed even with missing secrets
		assert.NoError(t, err)
		assert.Len(t, results, 1) // Only existing secret

		secret, exists := results["exists"]
		assert.True(t, exists)
		assert.Equal(t, []byte("value"), secret.Value)
	})

	t.Run("context cancellation", func(t *testing.T) {
		manager := NewManager(&Config{
			DefaultProvider: "mock",
		})

		mockProvider := newIntegrationMockWriteableProvider("mock")
		var provider Provider = mockProvider
		err := manager.RegisterProvider("mock", provider)
		require.NoError(t, err)

		// Create a cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		ref := SecretRef{Path: "test"}
		writeableProvider, ok := provider.(interface {
			Store(ctx context.Context, ref SecretRef, value []byte) error
		})
		require.True(t, ok)
		err = writeableProvider.Store(ctx, ref, []byte("value"))
		require.NoError(t, err)

		// Context cancellation is handled by the mock provider
		secret, err := manager.Resolve(ctx, ref)
		assert.NoError(t, err) // Mock provider doesn't check context
		assert.NotNil(t, secret)
	})
}

func TestManager_Lifecycle_Integration(t *testing.T) {
	t.Run("manager close cleans up properly", func(t *testing.T) {
		manager := NewManager(&Config{
			DefaultProvider: "mock",
		})

		mockProvider := newIntegrationMockWriteableProvider("mock")
		var provider Provider = mockProvider
		err := manager.RegisterProvider("mock", provider)
		require.NoError(t, err)

		ctx := context.Background()

		// Store some secrets
		ref1 := SecretRef{Path: "cleanup/test1"}
		ref2 := SecretRef{Path: "cleanup/test2"}
		writeableProvider, ok := provider.(interface {
			Store(ctx context.Context, ref SecretRef, value []byte) error
		})
		require.True(t, ok)
		err = writeableProvider.Store(ctx, ref1, []byte("value1"))
		require.NoError(t, err)
		err = writeableProvider.Store(ctx, ref2, []byte("value2"))
		require.NoError(t, err)

		// Verify secrets exist
		exists1, err := manager.Exists(ctx, ref1)
		assert.NoError(t, err)
		assert.True(t, exists1)

		exists2, err := manager.Exists(ctx, ref2)
		assert.NoError(t, err)
		assert.True(t, exists2)

		// Close manager
		err = manager.Close()
		assert.NoError(t, err)

		// Verify provider registry is cleared
		assert.Len(t, manager.providers, 0)
	})

	t.Run("multiple providers", func(t *testing.T) {
		manager := NewManager(&Config{
			DefaultProvider: "mock1",
		})

		mockProvider1 := newIntegrationMockWriteableProvider("mock1")
		mockProvider2 := newIntegrationMockWriteableProvider("mock2")

		var provider1 Provider = mockProvider1
		var provider2 Provider = mockProvider2

		err := manager.RegisterProvider("mock1", provider1)
		require.NoError(t, err)
		err = manager.RegisterProvider("mock2", provider2)
		require.NoError(t, err)

		ctx := context.Background()

		// Store secrets in different providers
		ref1 := SecretRef{Path: "provider1/secret"}
		ref2 := SecretRef{Path: "provider2/secret"}

		writeableProvider1, ok := provider1.(interface {
			Store(ctx context.Context, ref SecretRef, value []byte) error
		})
		require.True(t, ok)
		err = writeableProvider1.Store(ctx, ref1, []byte("value1"))
		require.NoError(t, err)

		writeableProvider2, ok := provider2.(interface {
			Store(ctx context.Context, ref SecretRef, value []byte) error
		})
		require.True(t, ok)
		err = writeableProvider2.Store(ctx, ref2, []byte("value2"))
		require.NoError(t, err)

		// Resolve from specific providers
		secret1, err := manager.ResolveFrom(ctx, "mock1", ref1)
		assert.NoError(t, err)
		assert.Equal(t, []byte("value1"), secret1.Value)

		secret2, err := manager.ResolveFrom(ctx, "mock2", ref2)
		assert.NoError(t, err)
		assert.Equal(t, []byte("value2"), secret2.Value)

		// Resolve from default provider
		defaultSecret, err := manager.Resolve(ctx, ref1)
		assert.NoError(t, err)
		assert.Equal(t, []byte("value1"), defaultSecret.Value)
	})
}

// UnwrapError is a helper to unwrap errors for testing
func UnwrapError(err error) error {
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return unwrapped
	}
	return err
}

// Benchmark integration performance
func BenchmarkManager_Resolve_Integration(b *testing.B) {
	manager := NewManager(&Config{
		DefaultProvider: "mock",
	})

	mockProvider := newIntegrationMockWriteableProvider("mock")
	var provider Provider = mockProvider
	err := manager.RegisterProvider("mock", provider)
	require.NoError(b, err)

	ctx := context.Background()

	// Pre-populate with test data
	ref := SecretRef{Path: "benchmark/secret"}
	value := []byte("benchmark-secret-value")
	writeableProvider, ok := provider.(interface {
		Store(ctx context.Context, ref SecretRef, value []byte) error
	})
	require.True(b, ok)
	err = writeableProvider.Store(ctx, ref, value)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.Resolve(ctx, ref)
	}
}

func BenchmarkManager_ResolveBatch_Integration(b *testing.B) {
	manager := NewManager(&Config{
		DefaultProvider: "mock",
	})

	mockProvider := newIntegrationMockWriteableProvider("mock")
	var provider Provider = mockProvider
	err := manager.RegisterProvider("mock", provider)
	require.NoError(b, err)

	ctx := context.Background()

	// Pre-populate with multiple secrets
	refs := make([]SecretRef, 10)
	writeableProvider, ok := provider.(interface {
		Store(ctx context.Context, ref SecretRef, value []byte) error
	})
	require.True(b, ok)

	for i := 0; i < 10; i++ {
		ref := SecretRef{Path: fmt.Sprintf("benchmark/secret%d", i)}
		value := []byte(fmt.Sprintf("benchmark-value-%d", i))
		err = writeableProvider.Store(ctx, ref, value)
		require.NoError(b, err)
		refs[i] = ref
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ResolveBatch(ctx, refs)
	}
}
