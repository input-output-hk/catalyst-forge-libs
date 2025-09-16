package memory

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/secrets/core"
)

func TestMemoryProvider_Name(t *testing.T) {
	p := New()
	assert.Equal(t, "memory", p.Name())
}

func TestMemoryProvider_HealthCheck(t *testing.T) {
	p := New()
	err := p.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestMemoryProvider_Close(t *testing.T) {
	p := New()

	// Store a secret first
	ref := core.SecretRef{Path: "test/secret"}
	value := []byte("test-value")
	err := p.Store(context.Background(), ref, value)
	require.NoError(t, err)

	// Verify it exists
	exists, err := p.Exists(context.Background(), ref)
	require.NoError(t, err)
	assert.True(t, exists)

	// Close the provider
	err = p.Close()
	assert.NoError(t, err)

	// Verify the store is cleared
	exists, err = p.Exists(context.Background(), ref)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryProvider_Store(t *testing.T) {
	p := New()
	ctx := context.Background()

	tests := []struct {
		name  string
		ref   core.SecretRef
		value []byte
	}{
		{
			name:  "basic store",
			ref:   core.SecretRef{Path: "db/password"},
			value: []byte("secret-password"),
		},
		{
			name:  "with version",
			ref:   core.SecretRef{Path: "api/key", Version: "v1"},
			value: []byte("api-key-v1"),
		},
		{
			name: "with metadata",
			ref: core.SecretRef{
				Path:     "cert/private",
				Version:  "latest",
				Metadata: map[string]string{"type": "rsa"},
			},
			value: []byte("private-key-data"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.Store(ctx, tt.ref, tt.value)
			assert.NoError(t, err)

			// Verify it can be resolved
			secret, err := p.Resolve(ctx, tt.ref)
			require.NoError(t, err)
			assert.Equal(t, tt.value, secret.Value)
			assert.NotEmpty(t, secret.Version)
			assert.False(t, secret.CreatedAt.IsZero())
		})
	}
}

func TestMemoryProvider_Resolve(t *testing.T) {
	p := New()
	ctx := context.Background()

	// Store a secret first
	ref := core.SecretRef{Path: "test/secret", Version: "v1"}
	value := []byte("test-value")
	err := p.Store(ctx, ref, value)
	require.NoError(t, err)

	// Resolve the secret
	secret, err := p.Resolve(ctx, ref)
	require.NoError(t, err)
	require.NotNil(t, secret)

	// Verify the secret contents
	assert.Equal(t, value, secret.Value)
	assert.Equal(t, "v1", secret.Version)
	assert.False(t, secret.CreatedAt.IsZero())
	assert.Nil(t, secret.ExpiresAt) // No expiration set
	assert.False(t, secret.AutoClear)
}

func TestMemoryProvider_Resolve_NotFound(t *testing.T) {
	p := New()
	ctx := context.Background()

	ref := core.SecretRef{Path: "nonexistent"}
	secret, err := p.Resolve(ctx, ref)
	assert.Error(t, err)
	assert.Nil(t, secret)
	assert.Contains(t, err.Error(), "secret not found")
}

func TestMemoryProvider_ResolveBatch(t *testing.T) {
	p := New()
	ctx := context.Background()

	// Store multiple secrets
	secretValues := map[string][]byte{
		"db/host":     []byte("localhost"),
		"db/password": []byte("secret123"),
		"api/key":     []byte("apikey456"),
	}

	refs := make([]core.SecretRef, 0, len(secretValues))
	for path, value := range secretValues {
		ref := core.SecretRef{Path: path}
		err := p.Store(ctx, ref, value)
		require.NoError(t, err)
		refs = append(refs, ref)
	}

	// Resolve batch
	results, err := p.ResolveBatch(ctx, refs)
	require.NoError(t, err)
	assert.Len(t, results, len(secretValues))

	// Verify all secrets were resolved
	for path, expectedValue := range secretValues {
		secret, exists := results[path]
		assert.True(t, exists, "secret %s should exist", path)
		assert.Equal(t, expectedValue, secret.Value)
	}
}

func TestMemoryProvider_ResolveBatch_PartialFailure(t *testing.T) {
	p := New()
	ctx := context.Background()

	// Store one secret
	existingRef := core.SecretRef{Path: "existing"}
	err := p.Store(ctx, existingRef, []byte("value"))
	require.NoError(t, err)

	// Try to resolve both existing and non-existing
	refs := []core.SecretRef{
		existingRef,
		{Path: "nonexistent"},
	}

	results, err := p.ResolveBatch(ctx, refs)
	// Should not error, even with partial failures
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// Only the existing secret should be in results
	secret, exists := results["existing"]
	assert.True(t, exists)
	assert.Equal(t, []byte("value"), secret.Value)
}

func TestMemoryProvider_Exists(t *testing.T) {
	p := New()
	ctx := context.Background()

	existingRef := core.SecretRef{Path: "exists"}
	nonExistingRef := core.SecretRef{Path: "does-not-exist"}

	// Store one secret
	err := p.Store(ctx, existingRef, []byte("value"))
	require.NoError(t, err)

	// Test existing secret
	exists, err := p.Exists(ctx, existingRef)
	require.NoError(t, err)
	assert.True(t, exists)

	// Test non-existing secret
	exists, err = p.Exists(ctx, nonExistingRef)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryProvider_Delete(t *testing.T) {
	p := New()
	ctx := context.Background()

	ref := core.SecretRef{Path: "to-delete"}

	// Store a secret
	err := p.Store(ctx, ref, []byte("value"))
	require.NoError(t, err)

	// Verify it exists
	exists, err := p.Exists(ctx, ref)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete it
	err = p.Delete(ctx, ref)
	assert.NoError(t, err)

	// Verify it's gone
	exists, err = p.Exists(ctx, ref)
	require.NoError(t, err)
	assert.False(t, exists)

	// Trying to resolve should fail
	secret, err := p.Resolve(ctx, ref)
	assert.Error(t, err)
	assert.Nil(t, secret)
}

func TestMemoryProvider_Rotate(t *testing.T) {
	p := New()
	ctx := context.Background()

	ref := core.SecretRef{Path: "rotate-me", Version: "v1"}

	// Store initial secret
	originalValue := []byte("original-value")
	err := p.Store(ctx, ref, originalValue)
	require.NoError(t, err)

	// Rotate the secret
	newSecret, err := p.Rotate(ctx, ref)
	require.NoError(t, err)
	require.NotNil(t, newSecret)

	// Verify the new secret has different content and version
	assert.NotEqual(t, originalValue, newSecret.Value)
	assert.NotEqual(t, "v1", newSecret.Version)
	assert.False(t, newSecret.CreatedAt.IsZero())

	// Original secret should still be accessible
	originalSecret, err := p.Resolve(ctx, core.SecretRef{Path: "rotate-me", Version: "v1"})
	require.NoError(t, err)
	assert.Equal(t, originalValue, originalSecret.Value)
	assert.Equal(t, "v1", originalSecret.Version)

	// New secret should be accessible with its version
	newSecretByVersion, err := p.Resolve(
		ctx,
		core.SecretRef{Path: "rotate-me", Version: newSecret.Version},
	)
	require.NoError(t, err)
	assert.Equal(t, newSecret.Value, newSecretByVersion.Value)
}

func TestMemoryProvider_Concurrency(t *testing.T) {
	p := New()
	ctx := context.Background()

	// Test concurrent access
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			ref := core.SecretRef{Path: fmt.Sprintf("concurrent/%d", id)}
			value := []byte(fmt.Sprintf("value-%d", id))

			// Store
			err := p.Store(ctx, ref, value)
			assert.NoError(t, err)

			// Resolve
			secret, err := p.Resolve(ctx, ref)
			assert.NoError(t, err)
			assert.Equal(t, value, secret.Value)

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMemoryProvider_ContextCancellation(t *testing.T) {
	p := New()

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ref := core.SecretRef{Path: "test"}
	_, err := p.Resolve(ctx, ref)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}
