package secrets

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAuditLogger is a test implementation of AuditLogger that captures log entries
type mockAuditLogger struct {
	entries []*AuditEntry
}

func (m *mockAuditLogger) LogAccess(
	ctx context.Context,
	action string,
	ref SecretRef,
	success bool,
	err error,
) {
	entry := NewAuditEntry(ctx, action, ref, success, err)
	m.entries = append(m.entries, entry)
}

func (m *mockAuditLogger) getEntries() []*AuditEntry {
	return m.entries
}

func (m *mockAuditLogger) clear() {
	m.entries = nil
}

func TestNewAuditEntry(t *testing.T) {
	ref := SecretRef{
		Path:     "test/secret",
		Version:  "v1",
		Metadata: map[string]string{"env": "test"},
	}

	ctx := context.WithValue(context.Background(), userIDKey, "test-user")
	ctx = context.WithValue(ctx, requestIDKey, "req-123")

	t.Run("successful operation", func(t *testing.T) {
		entry := NewAuditEntry(ctx, "resolve", ref, true, nil)

		assert.Equal(t, "resolve", entry.Action)
		assert.Equal(t, ref, entry.SecretRef)
		assert.True(t, entry.Success)
		assert.Empty(t, entry.Error)
		assert.NotZero(t, entry.Timestamp)
		assert.Equal(t, "test-user", entry.Context["user_id"])
		assert.Equal(t, "req-123", entry.Context["request_id"])
	})

	t.Run("failed operation", func(t *testing.T) {
		testErr := errors.New("secret not found")
		entry := NewAuditEntry(ctx, "resolve", ref, false, testErr)

		assert.Equal(t, "resolve", entry.Action)
		assert.Equal(t, ref, entry.SecretRef)
		assert.False(t, entry.Success)
		assert.Equal(t, "secret not found", entry.Error)
		assert.NotZero(t, entry.Timestamp)
		assert.Equal(t, "test-user", entry.Context["user_id"])
		assert.Equal(t, "req-123", entry.Context["request_id"])
	})

	t.Run("nil context", func(t *testing.T) {
		entry := NewAuditEntry(context.TODO(), "store", ref, true, nil)

		assert.Equal(t, "store", entry.Action)
		assert.Equal(t, ref, entry.SecretRef)
		assert.True(t, entry.Success)
		assert.Empty(t, entry.Error)
		assert.NotZero(t, entry.Timestamp)
		assert.Empty(t, entry.Context)
	})

	t.Run("context without values", func(t *testing.T) {
		entry := NewAuditEntry(context.Background(), "delete", ref, true, nil)

		assert.Equal(t, "delete", entry.Action)
		assert.Equal(t, ref, entry.SecretRef)
		assert.True(t, entry.Success)
		assert.Empty(t, entry.Context)
	})
}

func TestAuditEntryStructure(t *testing.T) {
	ref := SecretRef{
		Path:    "db/password",
		Version: "latest",
	}

	t.Run("timestamp is recent", func(t *testing.T) {
		before := time.Now()
		entry := NewAuditEntry(context.TODO(), "resolve", ref, true, nil)
		after := time.Now()

		assert.True(t, entry.Timestamp.After(before) || entry.Timestamp.Equal(before))
		assert.True(t, entry.Timestamp.Before(after) || entry.Timestamp.Equal(after))
	})

	t.Run("context values are properly extracted", func(t *testing.T) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, userIDKey, "user123")
		ctx = context.WithValue(ctx, requestIDKey, "req456")
		ctx = context.WithValue(ctx, sourceIPKey, "192.168.1.1")
		ctx = context.WithValue(ctx, otherKey, "should_be_ignored")

		entry := NewAuditEntry(ctx, "resolve", ref, true, nil)

		expectedContext := map[string]string{
			"user_id":    "user123",
			"request_id": "req456",
			"source_ip":  "192.168.1.1",
		}
		assert.Equal(t, expectedContext, entry.Context)
	})
}

func TestManagerAuditLogging(t *testing.T) {
	mockLogger := &mockAuditLogger{}
	mockProvider := &mockProvider{}

	config := &Config{
		DefaultProvider: "mock",
		EnableAudit:     true,
		AuditLogger:     mockLogger,
	}

	manager := NewManager(config)
	err := manager.RegisterProvider("mock", mockProvider)
	require.NoError(t, err)

	ref := SecretRef{Path: "test/secret", Version: "v1"}

	t.Run("successful resolution with audit logging", func(t *testing.T) {
		mockLogger.clear()
		mockProvider.reset()

		// Mock successful resolution
		expectedSecret := &Secret{
			Value:   []byte("test-value"),
			Version: "v1",
		}
		mockProvider.resolveResult = expectedSecret
		mockProvider.resolveError = nil

		ctx := context.WithValue(context.Background(), userIDKey, "test-user")
		secret, err := manager.ResolveFrom(ctx, "mock", ref)

		assert.NoError(t, err)
		assert.NotNil(t, secret)

		// Verify audit logging
		entries := mockLogger.getEntries()
		require.Len(t, entries, 1)

		entry := entries[0]
		assert.Equal(t, "resolve", entry.Action)
		assert.Equal(t, ref, entry.SecretRef)
		assert.True(t, entry.Success)
		assert.Empty(t, entry.Error)
		assert.Equal(t, "test-user", entry.Context["user_id"])
	})

	t.Run("failed resolution with audit logging", func(t *testing.T) {
		mockLogger.clear()
		mockProvider.reset()

		// Mock failed resolution
		mockProvider.resolveResult = nil
		mockProvider.resolveError = ErrSecretNotFound

		ctx := context.Background()
		secret, err := manager.ResolveFrom(ctx, "mock", ref)

		assert.Error(t, err)
		assert.Nil(t, secret)
		assert.True(t, errors.Is(err, ErrSecretNotFound))

		// Verify audit logging
		entries := mockLogger.getEntries()
		require.Len(t, entries, 1)

		entry := entries[0]
		assert.Equal(t, "resolve", entry.Action)
		assert.Equal(t, ref, entry.SecretRef)
		assert.False(t, entry.Success)
		assert.Equal(t, "secret not found", entry.Error)
		assert.Empty(t, entry.Context)
	})

	t.Run("provider not found with audit logging", func(t *testing.T) {
		mockLogger.clear()

		ctx := context.Background()
		secret, err := manager.ResolveFrom(ctx, "nonexistent", ref)

		assert.Error(t, err)
		assert.Nil(t, secret)
		assert.Contains(t, err.Error(), "provider \"nonexistent\" not found")

		// Verify audit logging for provider not found
		entries := mockLogger.getEntries()
		require.Len(t, entries, 1)

		entry := entries[0]
		assert.Equal(t, "resolve", entry.Action)
		assert.Equal(t, ref, entry.SecretRef)
		assert.False(t, entry.Success)
		assert.Contains(t, entry.Error, "provider \"nonexistent\" not found")
	})
}

func TestManagerAuditLoggingDisabled(t *testing.T) {
	mockLogger := &mockAuditLogger{}
	mockProvider := &mockProvider{}

	// Disable audit logging
	config := &Config{
		DefaultProvider: "mock",
		EnableAudit:     false, // Disabled
		AuditLogger:     mockLogger,
	}

	manager := NewManager(config)
	err := manager.RegisterProvider("mock", mockProvider)
	require.NoError(t, err)

	ref := SecretRef{Path: "test/secret", Version: "v1"}

	t.Run("no audit logging when disabled", func(t *testing.T) {
		mockLogger.clear()
		mockProvider.reset()

		// Mock successful resolution
		expectedSecret := &Secret{
			Value:   []byte("test-value"),
			Version: "v1",
		}
		mockProvider.resolveResult = expectedSecret
		mockProvider.resolveError = nil

		ctx := context.Background()
		secret, err := manager.ResolveFrom(ctx, "mock", ref)

		assert.NoError(t, err)
		assert.NotNil(t, secret)

		// Verify no audit logging occurred
		entries := mockLogger.getEntries()
		assert.Len(t, entries, 0)
	})
}

func TestManagerNilAuditLogger(t *testing.T) {
	mockProvider := &mockProvider{}

	// Nil audit logger but audit enabled
	config := &Config{
		DefaultProvider: "mock",
		EnableAudit:     true,
		AuditLogger:     nil, // Nil logger
	}

	manager := NewManager(config)
	err := manager.RegisterProvider("mock", mockProvider)
	require.NoError(t, err)

	ref := SecretRef{Path: "test/secret", Version: "v1"}

	t.Run("no panic with nil audit logger", func(t *testing.T) {
		mockProvider.reset()

		// Mock successful resolution
		expectedSecret := &Secret{
			Value:   []byte("test-value"),
			Version: "v1",
		}
		mockProvider.resolveResult = expectedSecret
		mockProvider.resolveError = nil

		ctx := context.Background()
		secret, err := manager.ResolveFrom(ctx, "mock", ref)

		assert.NoError(t, err)
		assert.NotNil(t, secret)
		// Should not panic with nil audit logger
	})

	t.Run("no panic with nil audit logger on error", func(t *testing.T) {
		mockProvider.reset()

		// Mock failed resolution
		mockProvider.resolveResult = nil
		mockProvider.resolveError = ErrSecretNotFound

		ctx := context.Background()
		secret, err := manager.ResolveFrom(ctx, "mock", ref)

		assert.Error(t, err)
		assert.Nil(t, secret)
		// Should not panic with nil audit logger
	})
}

// mockProvider is a test implementation of Provider for testing
type mockProvider struct {
	resolveResult *Secret
	resolveError  error
}

func (m *mockProvider) reset() {
	m.resolveResult = nil
	m.resolveError = nil
}

func (m *mockProvider) Resolve(ctx context.Context, ref SecretRef) (*Secret, error) {
	return m.resolveResult, m.resolveError
}

func (m *mockProvider) ResolveBatch(
	ctx context.Context,
	refs []SecretRef,
) (map[string]*Secret, error) {
	// Not implemented for this test
	return nil, errors.New("not implemented")
}

func (m *mockProvider) Exists(ctx context.Context, ref SecretRef) (bool, error) {
	// Not implemented for this test
	return false, errors.New("not implemented")
}

func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockProvider) Close() error {
	return nil
}
