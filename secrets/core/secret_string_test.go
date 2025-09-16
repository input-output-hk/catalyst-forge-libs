package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockResolver is a test resolver that can be configured with responses
type mockResolver struct {
	resolveFunc      func(ctx context.Context, ref SecretRef) (*Secret, error)
	resolveBatchFunc func(ctx context.Context, refs []SecretRef) (map[string]*Secret, error)
	existsFunc       func(ctx context.Context, ref SecretRef) (bool, error)
	resolveCalls     int
	mu               sync.Mutex
}

func (m *mockResolver) Resolve(ctx context.Context, ref SecretRef) (*Secret, error) {
	m.mu.Lock()
	m.resolveCalls++
	m.mu.Unlock()

	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, ref)
	}
	return &Secret{
		Value:     []byte("mock-secret"),
		Version:   "v1",
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockResolver) ResolveBatch(ctx context.Context, refs []SecretRef) (map[string]*Secret, error) {
	if m.resolveBatchFunc != nil {
		return m.resolveBatchFunc(ctx, refs)
	}
	return nil, fmt.Errorf("batch not implemented")
}

func (m *mockResolver) Exists(ctx context.Context, ref SecretRef) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, ref)
	}
	return true, nil
}

func (m *mockResolver) getResolveCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resolveCalls
}

func TestNewSecretString(t *testing.T) {
	ref := SecretRef{Path: "test/secret"}
	resolver := &mockResolver{}

	t.Run("default configuration", func(t *testing.T) {
		ss := NewSecretString(ref, resolver)
		require.NotNil(t, ss)
		assert.Equal(t, ref, ss.ref)
		assert.True(t, ss.oneTimeUse)
		assert.False(t, ss.allowCache)
		assert.False(t, ss.consumed)
	})

	t.Run("with nil resolver", func(t *testing.T) {
		ss := NewSecretString(ref, nil)
		require.NotNil(t, ss)

		ctx := context.Background()
		_, err := ss.Resolve(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no resolver configured")
	})

	t.Run("with functional options", func(t *testing.T) {
		ss := NewSecretString(ref, resolver,
			WithOneTimeUse(false),
			WithCaching(true),
		)
		require.NotNil(t, ss)
		assert.False(t, ss.oneTimeUse)
		assert.True(t, ss.allowCache)
	})

	t.Run("from manager", func(t *testing.T) {
		manager := NewManager(&Config{})
		ss := NewSecretStringFromManager(ref, manager, WithOneTimeUse(false))
		require.NotNil(t, ss)
		assert.False(t, ss.oneTimeUse)
	})
}

func TestSecretString_OneTimeUse(t *testing.T) {
	ctx := context.Background()
	ref := SecretRef{Path: "test/secret"}

	t.Run("enforces one-time use by default", func(t *testing.T) {
		resolver := &mockResolver{}
		ss := NewSecretString(ref, resolver)

		// First resolve should succeed
		secret, err := ss.Resolve(ctx)
		require.NoError(t, err)
		require.NotNil(t, secret)
		assert.Equal(t, "mock-secret", string(secret.Value))
		assert.True(t, ss.IsConsumed())

		// Second resolve should fail
		_, err = ss.Resolve(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already been consumed")

		// Resolver should only be called once
		assert.Equal(t, 1, resolver.getResolveCalls())
	})

	t.Run("String() consumes the secret", func(t *testing.T) {
		resolver := &mockResolver{}
		ss := NewSecretString(ref, resolver)

		// First String() should succeed
		str, err := ss.String(ctx)
		require.NoError(t, err)
		assert.Equal(t, "mock-secret", str)

		// Second String() should fail
		_, err = ss.String(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already been consumed")
	})

	t.Run("Bytes() consumes the secret", func(t *testing.T) {
		resolver := &mockResolver{}
		ss := NewSecretString(ref, resolver)

		// First Bytes() should succeed
		bytes, err := ss.Bytes(ctx)
		require.NoError(t, err)
		assert.Equal(t, []byte("mock-secret"), bytes)

		// Second Bytes() should fail
		_, err = ss.Bytes(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already been consumed")
	})

	t.Run("can disable one-time use", func(t *testing.T) {
		resolver := &mockResolver{}
		ss := NewSecretString(ref, resolver, WithOneTimeUse(false))

		// Multiple resolves should succeed
		for i := 0; i < 3; i++ {
			secret, err := ss.Resolve(ctx)
			require.NoError(t, err)
			require.NotNil(t, secret)
			assert.Equal(t, "mock-secret", string(secret.Value))
			assert.False(t, ss.IsConsumed())
		}
	})
}

func TestSecretString_Caching(t *testing.T) {
	ctx := context.Background()
	ref := SecretRef{Path: "test/secret"}

	t.Run("caching with multiple use", func(t *testing.T) {
		resolver := &mockResolver{}
		ss := NewSecretString(ref, resolver,
			WithOneTimeUse(false),
			WithCaching(true),
		)

		// Multiple resolves should succeed
		for i := 0; i < 3; i++ {
			secret, err := ss.Resolve(ctx)
			require.NoError(t, err)
			require.NotNil(t, secret)
			assert.Equal(t, "mock-secret", string(secret.Value))
		}

		// Resolver should only be called once due to caching
		assert.Equal(t, 1, resolver.getResolveCalls())
	})

	t.Run("no caching by default", func(t *testing.T) {
		callCount := 0
		resolver := &mockResolver{
			resolveFunc: func(ctx context.Context, ref SecretRef) (*Secret, error) {
				callCount++
				return &Secret{
					Value:     []byte(fmt.Sprintf("secret-%d", callCount)),
					Version:   "v1",
					CreatedAt: time.Now(),
				}, nil
			},
		}

		ss := NewSecretString(ref, resolver, WithOneTimeUse(false))

		// Each resolve should get a fresh secret
		str1, err := ss.String(ctx)
		require.NoError(t, err)
		assert.Equal(t, "secret-1", str1)

		str2, err := ss.String(ctx)
		require.NoError(t, err)
		assert.Equal(t, "secret-2", str2)

		assert.Equal(t, 2, callCount)
	})

	t.Run("caching ineffective with one-time use", func(t *testing.T) {
		resolver := &mockResolver{}
		ss := NewSecretString(ref, resolver,
			WithOneTimeUse(true),
			WithCaching(true),
		)

		// First resolve should succeed
		_, err := ss.Resolve(ctx)
		require.NoError(t, err)

		// Second resolve should fail despite caching
		_, err = ss.Resolve(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already been consumed")
	})
}

func TestSecretString_Clear(t *testing.T) {
	ctx := context.Background()
	ref := SecretRef{Path: "test/secret"}

	t.Run("clear marks as consumed for one-time use", func(t *testing.T) {
		resolver := &mockResolver{}
		ss := NewSecretString(ref, resolver)

		assert.False(t, ss.IsConsumed())
		ss.Clear()
		assert.True(t, ss.IsConsumed())

		// Should not be able to resolve after clear
		_, err := ss.Resolve(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already been consumed")
	})

	t.Run("clear removes cached value", func(t *testing.T) {
		resolver := &mockResolver{}
		ss := NewSecretString(ref, resolver,
			WithOneTimeUse(false),
			WithCaching(true),
		)

		// Resolve to cache the value
		_, err := ss.Resolve(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, resolver.getResolveCalls())

		// Clear should remove the cached value
		ss.Clear()

		// Next resolve should call resolver again
		_, err = ss.Resolve(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, resolver.getResolveCalls())
	})
}

func TestSecretString_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	ref := SecretRef{Path: "test/secret"}

	t.Run("propagates resolver errors", func(t *testing.T) {
		expectedErr := fmt.Errorf("resolver error")
		resolver := &mockResolver{
			resolveFunc: func(ctx context.Context, ref SecretRef) (*Secret, error) {
				return nil, expectedErr
			},
		}
		ss := NewSecretString(ref, resolver)

		_, err := ss.Resolve(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve secret")
		assert.Contains(t, err.Error(), expectedErr.Error())

		// Should not be marked as consumed on error
		assert.False(t, ss.IsConsumed())
	})

	t.Run("context cancellation", func(t *testing.T) {
		resolver := &mockResolver{
			resolveFunc: func(ctx context.Context, ref SecretRef) (*Secret, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(100 * time.Millisecond):
					return &Secret{Value: []byte("secret")}, nil
				}
			},
		}
		ss := NewSecretString(ref, resolver)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := ss.Resolve(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), context.Canceled.Error())
	})
}

func TestSecretString_Concurrency(t *testing.T) {
	ctx := context.Background()
	ref := SecretRef{Path: "test/secret"}

	t.Run("one-time use with concurrent access", func(t *testing.T) {
		resolver := &mockResolver{}
		ss := NewSecretString(ref, resolver)

		var wg sync.WaitGroup
		successCount := 0
		errorCount := 0
		var mu sync.Mutex

		// Try to resolve concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := ss.Resolve(ctx)
				mu.Lock()
				if err == nil {
					successCount++
				} else {
					errorCount++
				}
				mu.Unlock()
			}()
		}

		wg.Wait()

		// Only one should succeed due to one-time use
		assert.Equal(t, 1, successCount)
		assert.Equal(t, 9, errorCount)
		assert.Equal(t, 1, resolver.getResolveCalls())
	})

	t.Run("multiple use with concurrent access", func(t *testing.T) {
		resolver := &mockResolver{}
		ss := NewSecretString(ref, resolver,
			WithOneTimeUse(false),
			WithCaching(true),
		)

		var wg sync.WaitGroup
		successCount := 0
		var mu sync.Mutex

		// Try to resolve concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := ss.Resolve(ctx)
				if err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		// All should succeed with multiple use
		assert.Equal(t, 10, successCount)
		// Should only resolve once due to caching
		assert.Equal(t, 1, resolver.getResolveCalls())
	})
}

func TestSecretString_WithManager(t *testing.T) {
	ctx := context.Background()

	t.Run("uses manager as resolver", func(t *testing.T) {
		// Set up manager with memory provider
		config := &Config{
			DefaultProvider: "memory",
			AutoClear:       true,
		}
		manager := NewManager(config)
		defer manager.Close()

		// Use the existing mockProvider from audit_test.go pattern
		provider := &mockProvider{
			resolveResult: &Secret{
				Value:     []byte("manager-secret"),
				Version:   "v1",
				CreatedAt: time.Now(),
			},
		}
		err := manager.RegisterProvider("memory", provider)
		require.NoError(t, err)

		// Create SecretString using manager
		ref := SecretRef{Path: "test/secret"}
		ss := NewSecretStringFromManager(ref, manager)

		// Resolve should use the manager
		str, err := ss.String(ctx)
		require.NoError(t, err)
		assert.Equal(t, "manager-secret", str)
	})
}

func TestSecretString_Ref(t *testing.T) {
	ref := SecretRef{
		Path:    "test/secret",
		Version: "v1",
		Metadata: map[string]string{
			"key": "value",
		},
	}
	resolver := &mockResolver{}
	ss := NewSecretString(ref, resolver)

	t.Run("returns reference without consuming", func(t *testing.T) {
		// Get ref multiple times
		for i := 0; i < 3; i++ {
			gotRef := ss.Ref()
			assert.Equal(t, ref, gotRef)
			assert.False(t, ss.IsConsumed())
		}
	})
}

func TestSecretString_AutoClear(t *testing.T) {
	ctx := context.Background()
	ref := SecretRef{Path: "test/secret"}

	t.Run("respects secret's AutoClear setting", func(t *testing.T) {
		resolver := &mockResolver{
			resolveFunc: func(ctx context.Context, ref SecretRef) (*Secret, error) {
				return &Secret{
					Value:     []byte("auto-clear-secret"),
					AutoClear: true,
				}, nil
			},
		}

		ss := NewSecretString(ref, resolver, WithOneTimeUse(false))

		// Resolve the secret
		secret, err := ss.Resolve(ctx)
		require.NoError(t, err)

		// Call String() which should trigger AutoClear
		str := secret.String()
		assert.Equal(t, "auto-clear-secret", str)

		// Secret value should be cleared
		assert.Nil(t, secret.Value)
		assert.Equal(t, "", secret.String())
	})
}

func TestSecretString_CopyPreventsModification(t *testing.T) {
	ctx := context.Background()
	ref := SecretRef{Path: "test/secret"}

	t.Run("external modification doesn't affect cached value", func(t *testing.T) {
		resolver := &mockResolver{
			resolveFunc: func(ctx context.Context, ref SecretRef) (*Secret, error) {
				return &Secret{
					Value:     []byte("original-secret"),
					Version:   "v1",
					CreatedAt: time.Now(),
				}, nil
			},
		}

		ss := NewSecretString(ref, resolver,
			WithOneTimeUse(false),
			WithCaching(true),
		)

		// First resolve
		secret1, err := ss.Resolve(ctx)
		require.NoError(t, err)

		// Modify the returned secret
		secret1.Value[0] = 'X'

		// Second resolve should return unmodified cached value
		secret2, err := ss.Resolve(ctx)
		require.NoError(t, err)
		assert.Equal(t, "original-secret", string(secret2.Value))
	})
}
