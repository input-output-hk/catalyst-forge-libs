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

// managerMockProvider is a test implementation of the Provider interface
type managerMockProvider struct {
	name          string
	healthError   error
	closeError    error
	resolveError  error
	resolveResult *Secret
	batchResults  map[string]*Secret
	batchError    error
	existsResult  bool
	existsError   error
	mu            sync.RWMutex
	closed        bool
}

func newManagerMockProvider(name string) *managerMockProvider {
	return &managerMockProvider{
		name:         name,
		batchResults: make(map[string]*Secret),
	}
}

func (m *managerMockProvider) Name() string {
	return m.name
}

func (m *managerMockProvider) HealthCheck(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return errors.New("provider closed")
	}
	return m.healthError
}

func (m *managerMockProvider) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return m.closeError
}

func (m *managerMockProvider) Resolve(ctx context.Context, ref SecretRef) (*Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, errors.New("provider closed")
	}
	return m.resolveResult, m.resolveError
}

func (m *managerMockProvider) ResolveBatch(
	ctx context.Context,
	refs []SecretRef,
) (map[string]*Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, errors.New("provider closed")
	}
	return m.batchResults, m.batchError
}

func (m *managerMockProvider) Exists(ctx context.Context, ref SecretRef) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return false, errors.New("provider closed")
	}
	return m.existsResult, m.existsError
}

func TestNewManager(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		manager := NewManager(nil)
		assert.NotNil(t, manager)
		assert.Empty(t, manager.defaultProvider)
		assert.False(t, manager.autoClear)
		assert.False(t, manager.enableAudit)
		assert.Nil(t, manager.auditLogger)
		assert.NotNil(t, manager.providers)
	})

	t.Run("with config", func(t *testing.T) {
		config := &Config{
			DefaultProvider: "test-provider",
			AutoClear:       true,
			EnableAudit:     true,
			AuditLogger:     &managerMockAuditLogger{},
		}

		manager := NewManager(config)
		assert.NotNil(t, manager)
		assert.Equal(t, "test-provider", manager.defaultProvider)
		assert.True(t, manager.autoClear)
		assert.True(t, manager.enableAudit)
		assert.NotNil(t, manager.auditLogger)
	})
}

func TestManager_RegisterProvider(t *testing.T) {
	manager := NewManager(nil)

	t.Run("successful registration", func(t *testing.T) {
		provider := newManagerMockProvider("test-provider")
		err := manager.RegisterProvider("test", provider)
		assert.NoError(t, err)
	})

	t.Run("empty name", func(t *testing.T) {
		provider := newManagerMockProvider("test-provider")
		err := manager.RegisterProvider("", provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider name cannot be empty")
	})

	t.Run("nil provider", func(t *testing.T) {
		err := manager.RegisterProvider("test", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider cannot be nil")
	})

	t.Run("duplicate registration", func(t *testing.T) {
		provider1 := newManagerMockProvider("test-provider-1")
		provider2 := newManagerMockProvider("test-provider-2")

		err := manager.RegisterProvider("duplicate", provider1)
		assert.NoError(t, err)

		err = manager.RegisterProvider("duplicate", provider2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider with name")
	})

	t.Run("concurrent registration", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				provider := newManagerMockProvider(fmt.Sprintf("provider-%d", id))
				err := manager.RegisterProvider(fmt.Sprintf("test-%d", id), provider)
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestManager_Resolve(t *testing.T) {
	ctx := context.Background()

	t.Run("no default provider configured", func(t *testing.T) {
		manager := NewManager(nil)
		ref := SecretRef{Path: "test/path"}
		secret, err := manager.Resolve(ctx, ref)
		assert.Error(t, err)
		assert.Nil(t, secret)
		assert.Contains(t, err.Error(), "no default provider configured")
	})

	t.Run("successful resolution", func(t *testing.T) {
		manager := NewManager(&Config{
			DefaultProvider: "test-provider",
			AutoClear:       true,
		})
		provider := newManagerMockProvider("test-provider")
		expectedSecret := &Secret{
			Value:     []byte("test-value"),
			Version:   "v1",
			CreatedAt: time.Now(),
			AutoClear: false,
		}
		provider.resolveResult = expectedSecret

		err := manager.RegisterProvider("test-provider", provider)
		require.NoError(t, err)

		ref := SecretRef{Path: "test/path"}
		secret, err := manager.Resolve(ctx, ref)
		assert.NoError(t, err)
		assert.NotNil(t, secret)
		assert.Equal(t, expectedSecret.Value, secret.Value)
		assert.Equal(t, expectedSecret.Version, secret.Version)
		// AutoClear should be applied from manager config
		assert.True(t, secret.AutoClear)
	})

	t.Run("provider resolution error", func(t *testing.T) {
		manager := NewManager(&Config{
			DefaultProvider: "test-provider",
		})
		provider := newManagerMockProvider("test-provider")
		provider.resolveError = errors.New("provider error")

		err := manager.RegisterProvider("test-provider", provider)
		require.NoError(t, err)

		ref := SecretRef{Path: "test/path"}
		secret, err := manager.Resolve(ctx, ref)
		assert.Error(t, err)
		assert.Nil(t, secret)
		assert.Contains(t, err.Error(), "failed to resolve secret")
		assert.Contains(t, err.Error(), "provider error")
	})

	t.Run("provider not found", func(t *testing.T) {
		manager := NewManager(&Config{
			DefaultProvider: "nonexistent-provider",
		})
		ref := SecretRef{Path: "test/path"}
		secret, err := manager.Resolve(ctx, ref)
		assert.Error(t, err)
		assert.Nil(t, secret)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestManager_ResolveFrom(t *testing.T) {
	ctx := context.Background()

	t.Run("empty provider name", func(t *testing.T) {
		manager := NewManager(&Config{
			AutoClear: true,
		})
		ref := SecretRef{Path: "test/path"}
		secret, err := manager.ResolveFrom(ctx, "", ref)
		assert.Error(t, err)
		assert.Nil(t, secret)
		assert.Contains(t, err.Error(), "provider name cannot be empty")
	})

	t.Run("successful resolution", func(t *testing.T) {
		manager := NewManager(&Config{
			AutoClear: true,
		})
		provider := newManagerMockProvider("specific-provider")
		expectedSecret := &Secret{
			Value:     []byte("specific-value"),
			Version:   "v2",
			CreatedAt: time.Now(),
			AutoClear: false,
		}
		provider.resolveResult = expectedSecret

		err := manager.RegisterProvider("specific-provider", provider)
		require.NoError(t, err)

		ref := SecretRef{Path: "test/path"}
		secret, err := manager.ResolveFrom(ctx, "specific-provider", ref)
		assert.NoError(t, err)
		assert.NotNil(t, secret)
		assert.True(t, secret.AutoClear) // Should be applied from manager
	})

	t.Run("provider not found", func(t *testing.T) {
		manager := NewManager(&Config{
			AutoClear: true,
		})
		ref := SecretRef{Path: "test/path"}
		secret, err := manager.ResolveFrom(ctx, "nonexistent", ref)
		assert.Error(t, err)
		assert.Nil(t, secret)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestManager_Close(t *testing.T) {
	t.Run("successful close", func(t *testing.T) {
		manager := NewManager(nil)
		ctx := context.Background()

		provider1 := newManagerMockProvider("provider1")
		provider2 := newManagerMockProvider("provider2")

		err := manager.RegisterProvider("p1", provider1)
		require.NoError(t, err)
		err = manager.RegisterProvider("p2", provider2)
		require.NoError(t, err)

		// Verify providers are not closed
		err = provider1.HealthCheck(ctx)
		assert.NoError(t, err)
		err = provider2.HealthCheck(ctx)
		assert.NoError(t, err)

		// Close manager
		err = manager.Close()
		assert.NoError(t, err)

		// Verify providers are closed
		err = provider1.HealthCheck(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider closed")
		err = provider2.HealthCheck(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider closed")

		// Verify registry is cleared
		assert.Len(t, manager.providers, 0)
	})

	t.Run("close with provider errors", func(t *testing.T) {
		manager := NewManager(nil)

		provider1 := newManagerMockProvider("provider1")
		provider2 := newManagerMockProvider("provider2")
		provider2.closeError = errors.New("close failed")

		err := manager.RegisterProvider("p1", provider1)
		require.NoError(t, err)
		err = manager.RegisterProvider("p2", provider2)
		require.NoError(t, err)

		err = manager.Close()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "errors during shutdown")
		assert.Contains(t, err.Error(), "close failed")
	})

	t.Run("close empty manager", func(t *testing.T) {
		manager := NewManager(nil)
		err := manager.Close()
		assert.NoError(t, err)
	})

	t.Run("concurrent close", func(t *testing.T) {
		manager := NewManager(nil)

		// Register multiple providers
		for i := 0; i < 5; i++ {
			provider := newManagerMockProvider(fmt.Sprintf("provider-%d", i))
			err := manager.RegisterProvider(fmt.Sprintf("p%d", i), provider)
			require.NoError(t, err)
		}

		done := make(chan bool, 10)

		// Close from multiple goroutines
		for i := 0; i < 10; i++ {
			go func() {
				err := manager.Close()
				// Close should be idempotent, some calls may error due to concurrent access
				_ = err // We don't assert here as concurrent closes may behave differently
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestManager_ResolveBatch(t *testing.T) {
	manager := NewManager(&Config{
		DefaultProvider: "test-provider",
		AutoClear:       true,
	})
	ctx := context.Background()

	provider := newManagerMockProvider("test-provider")
	expectedResults := map[string]*Secret{
		"secret1": {Value: []byte("value1"), Version: "v1", CreatedAt: time.Now()},
		"secret2": {Value: []byte("value2"), Version: "v2", CreatedAt: time.Now()},
	}
	provider.batchResults = expectedResults

	err := manager.RegisterProvider("test-provider", provider)
	require.NoError(t, err)

	refs := []SecretRef{
		{Path: "secret1"},
		{Path: "secret2"},
	}

	results, err := manager.ResolveBatch(ctx, refs)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	for path, secret := range results {
		expected, exists := expectedResults[path]
		assert.True(t, exists)
		assert.Equal(t, expected.Value, secret.Value)
		assert.True(t, secret.AutoClear) // Should be applied from manager
	}
}

func TestManager_Exists(t *testing.T) {
	manager := NewManager(&Config{
		DefaultProvider: "test-provider",
	})
	ctx := context.Background()

	provider := newManagerMockProvider("test-provider")
	provider.existsResult = true

	err := manager.RegisterProvider("test-provider", provider)
	require.NoError(t, err)

	ref := SecretRef{Path: "test/path"}
	exists, err := manager.Exists(ctx, ref)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestManager_AutoClear(t *testing.T) {
	manager := NewManager(&Config{
		DefaultProvider: "test-provider",
		AutoClear:       true,
	})
	ctx := context.Background()

	provider := newManagerMockProvider("test-provider")
	provider.resolveResult = &Secret{
		Value:     []byte("test-value"),
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false, // Provider returns false
	}

	err := manager.RegisterProvider("test-provider", provider)
	require.NoError(t, err)

	ref := SecretRef{Path: "test/path"}
	secret, err := manager.Resolve(ctx, ref)
	assert.NoError(t, err)
	assert.True(t, secret.AutoClear) // Manager config should override
}

func TestManager_ContextCancellation(t *testing.T) {
	manager := NewManager(&Config{
		DefaultProvider: "test-provider",
	})

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	provider := newManagerMockProvider("test-provider")
	// Mock provider doesn't actually check context, so it should work normally
	provider.resolveResult = &Secret{
		Value:     []byte("test-value"),
		Version:   "v1",
		CreatedAt: time.Now(),
	}

	err := manager.RegisterProvider("test-provider", provider)
	require.NoError(t, err)

	ref := SecretRef{Path: "test/path"}
	secret, err := manager.Resolve(ctx, ref)
	// Since the mock provider doesn't respect context cancellation,
	// it should succeed (this tests that the manager doesn't add extra context checks)
	assert.NoError(t, err)
	assert.NotNil(t, secret)
}

// managerMockAuditLogger is a test implementation of AuditLogger
type managerMockAuditLogger struct {
	logs []AuditEntry
	mu   sync.Mutex
}

func (m *managerMockAuditLogger) LogAccess(
	ctx context.Context,
	action string,
	ref SecretRef,
	success bool,
	err error,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := NewAuditEntry(ctx, action, ref, success, err)
	m.logs = append(m.logs, *entry)
}

func (m *managerMockAuditLogger) GetLogs() []AuditEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	logs := make([]AuditEntry, len(m.logs))
	copy(logs, m.logs)
	return logs
}

func TestManager_AuditLogging(t *testing.T) {
	auditLogger := &managerMockAuditLogger{}
	manager := NewManager(&Config{
		DefaultProvider: "test-provider",
		EnableAudit:     true,
		AuditLogger:     auditLogger,
	})
	ctx := context.Background()

	provider := newManagerMockProvider("test-provider")
	provider.resolveResult = &Secret{
		Value:     []byte("test-value"),
		Version:   "v1",
		CreatedAt: time.Now(),
	}

	err := manager.RegisterProvider("test-provider", provider)
	require.NoError(t, err)

	// Perform a successful resolution
	ref := SecretRef{Path: "test/path"}
	secret, err := manager.Resolve(ctx, ref)
	assert.NoError(t, err)
	assert.NotNil(t, secret)

	// Check audit log
	logs := auditLogger.GetLogs()
	assert.Len(t, logs, 1)
	assert.Equal(t, "resolve", logs[0].Action)
	assert.Equal(t, ref.Path, logs[0].SecretRef.Path)
	assert.True(t, logs[0].Success)
}

func TestManager_AuditLogging_Error(t *testing.T) {
	auditLogger := &managerMockAuditLogger{}
	manager := NewManager(&Config{
		DefaultProvider: "test-provider",
		EnableAudit:     true,
		AuditLogger:     auditLogger,
	})
	ctx := context.Background()

	provider := newManagerMockProvider("test-provider")
	provider.resolveError = errors.New("test error")

	err := manager.RegisterProvider("test-provider", provider)
	require.NoError(t, err)

	// Perform a failed resolution
	ref := SecretRef{Path: "test/path"}
	secret, err := manager.Resolve(ctx, ref)
	assert.Error(t, err)
	assert.Nil(t, secret)

	// Check audit log
	logs := auditLogger.GetLogs()
	assert.Len(t, logs, 1)
	assert.Equal(t, "resolve", logs[0].Action)
	assert.False(t, logs[0].Success)
	assert.Equal(t, "test error", logs[0].Error)
}

func TestManager_NoAuditLogging(t *testing.T) {
	auditLogger := &managerMockAuditLogger{}
	manager := NewManager(&Config{
		DefaultProvider: "test-provider",
		EnableAudit:     false, // Audit disabled
		AuditLogger:     auditLogger,
	})
	ctx := context.Background()

	provider := newManagerMockProvider("test-provider")
	provider.resolveResult = &Secret{
		Value:     []byte("test-value"),
		Version:   "v1",
		CreatedAt: time.Now(),
	}

	err := manager.RegisterProvider("test-provider", provider)
	require.NoError(t, err)

	// Perform resolution
	ref := SecretRef{Path: "test/path"}
	secret, err := manager.Resolve(ctx, ref)
	assert.NoError(t, err)
	assert.NotNil(t, secret)

	// Check no logs were created
	logs := auditLogger.GetLogs()
	assert.Len(t, logs, 0)
}

// Benchmark tests
func BenchmarkManager_Resolve(b *testing.B) {
	manager := NewManager(&Config{
		DefaultProvider: "test-provider",
	})
	ctx := context.Background()

	provider := newManagerMockProvider("test-provider")
	provider.resolveResult = &Secret{
		Value:     []byte("benchmark-value"),
		Version:   "v1",
		CreatedAt: time.Now(),
	}

	err := manager.RegisterProvider("test-provider", provider)
	require.NoError(b, err)

	ref := SecretRef{Path: "benchmark/path"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.Resolve(ctx, ref)
	}
}

func BenchmarkManager_ResolveFrom(b *testing.B) {
	manager := NewManager(nil)
	ctx := context.Background()

	provider := newManagerMockProvider("test-provider")
	provider.resolveResult = &Secret{
		Value:     []byte("benchmark-value"),
		Version:   "v1",
		CreatedAt: time.Now(),
	}

	err := manager.RegisterProvider("test-provider", provider)
	require.NoError(b, err)

	ref := SecretRef{Path: "benchmark/path"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ResolveFrom(ctx, "test-provider", ref)
	}
}
