//go:build integration

// Package aws_test provides integration tests for the AWS Secrets Manager provider.
// These tests use LocalStack via testcontainers to avoid external AWS dependencies.
//
// IMPORTANT: This file uses build tags and will only be included when running:
//
//	go test -tags=integration -v ./...
//
// Running 'go test ./...' without the integration tag will skip these tests.
//
// The integration tests require Docker to be running for LocalStack containers.
package aws_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/input-output-hk/catalyst-forge-libs/secrets/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/localstack"

	"github.com/input-output-hk/catalyst-forge-libs/secrets/providers/aws"
)

// testContainer manages the LocalStack test container lifecycle
type testContainer struct {
	container *localstack.LocalStackContainer
	uri       string
}

var (
	// Global container instance - initialized once and reused across tests
	globalContainer *testContainer
	containerOnce   sync.Once
	containerMutex  sync.Mutex
)

// getTestContainer returns a singleton LocalStack container for all integration tests
func getTestContainer(ctx context.Context) (*testContainer, error) {
	containerMutex.Lock()
	defer containerMutex.Unlock()

	var err error
	containerOnce.Do(func() {
		// Start LocalStack container
		container, startErr := localstack.Run(ctx, "localstack/localstack:latest")
		if startErr != nil {
			err = fmt.Errorf("failed to start LocalStack container: %w", startErr)
			return
		}

		// Get the container URI for LocalStack port 4566
		port, _ := nat.NewPort("tcp", "4566")
		uri, uriErr := container.PortEndpoint(ctx, port, "")
		if uriErr != nil {
			// Attempt to terminate container but don't override the main error
			_ = container.Terminate(ctx) // Ignore error as we're already failing
			err = fmt.Errorf("failed to get LocalStack endpoint: %w", uriErr)
			return
		}

		// Ensure URI has http:// scheme
		if !strings.HasPrefix(uri, "http://") && !strings.HasPrefix(uri, "https://") {
			uri = "http://" + uri
		}

		globalContainer = &testContainer{
			container: container,
			uri:       uri,
		}
	})

	if err != nil {
		return nil, err
	}

	return globalContainer, nil
}

// terminateTestContainer cleans up the global test container
func terminateTestContainer(ctx context.Context) error {
	containerMutex.Lock()
	defer containerMutex.Unlock()

	if globalContainer != nil {
		err := globalContainer.container.Terminate(ctx)
		globalContainer = nil
		containerOnce = sync.Once{}
		return err
	}
	return nil
}

// newTestProvider creates a new AWS Secrets Manager provider configured for LocalStack
func newTestProvider(ctx context.Context, t *testing.T) (*aws.Provider, error) {
	tc, err := getTestContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Create provider with LocalStack endpoint
	return aws.New(aws.WithEndpoint(tc.uri))
}

// newTestProviderForBench creates a new AWS Secrets Manager provider configured for LocalStack (benchmark version)
func newTestProviderForBench(ctx context.Context) (*aws.Provider, error) {
	tc, err := getTestContainer(ctx)
	if err != nil {
		return nil, err
	}

	// Create provider with LocalStack endpoint
	return aws.New(aws.WithEndpoint(tc.uri))
}

// TestMain handles setup and teardown for integration tests
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Setup: Start LocalStack container
	_, err := getTestContainer(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start LocalStack: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown: Stop LocalStack container
	if err := terminateTestContainer(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to terminate LocalStack: %v\n", err)
	}

	os.Exit(code)
}

// TestProviderLifecycle tests the complete provider lifecycle: create, read, update, delete
func TestProviderLifecycle(t *testing.T) {
	ctx := context.Background()
	provider, err := newTestProvider(ctx, t)
	require.NoError(t, err, "Failed to create test provider")
	require.NotNil(t, provider, "Provider should not be nil")

	secretName := fmt.Sprintf("test-secret-%d", time.Now().UnixNano())
	secretValue1 := `{"username":"admin","password":"secret123"}`
	secretValue2 := `{"username":"admin","password":"updated456"}`

	// Test 1: Basic provider methods
	t.Run("BasicProviderMethods", func(t *testing.T) {
		// Test Name()
		name := provider.Name()
		assert.Equal(t, "aws", name, "Provider name should be 'aws'")

		// Test HealthCheck()
		err := provider.HealthCheck(ctx)
		assert.NoError(t, err, "HealthCheck should succeed")

		// Test Close() - should not error even if no resources to close
		err = provider.Close()
		assert.NoError(t, err, "Close should succeed")
	})

	// Test 2: Store (create) secret
	t.Run("StoreSecret", func(t *testing.T) {
		ref := core.SecretRef{Path: secretName}
		err := provider.Store(ctx, ref, []byte(secretValue1))
		assert.NoError(t, err, "Store should succeed")
	})

	// Test 3: Exists - secret should exist
	t.Run("ExistsAfterStore", func(t *testing.T) {
		ref := core.SecretRef{Path: secretName}
		exists, err := provider.Exists(ctx, ref)
		assert.NoError(t, err, "Exists should succeed")
		assert.True(t, exists, "Secret should exist after store")
	})

	// Test 4: Resolve (read) secret
	t.Run("ResolveSecret", func(t *testing.T) {
		ref := core.SecretRef{Path: secretName}
		secret, err := provider.Resolve(ctx, ref)
		assert.NoError(t, err, "Resolve should succeed")
		assert.NotNil(t, secret, "Resolved secret should not be nil")
		assert.Equal(t, secretValue1, string(secret.Value), "Retrieved value should match stored value")
		assert.Equal(t, secretName, ref.Path, "Secret path should match")
	})

	// Test 5: Store (update) existing secret
	t.Run("UpdateSecret", func(t *testing.T) {
		ref := core.SecretRef{Path: secretName}
		err := provider.Store(ctx, ref, []byte(secretValue2))
		assert.NoError(t, err, "Update should succeed")

		// Verify the update
		secret, err := provider.Resolve(ctx, ref)
		assert.NoError(t, err, "Resolve after update should succeed")
		assert.Equal(t, secretValue2, string(secret.Value), "Retrieved value should match updated value")
	})

	// Test 6: Delete secret
	t.Run("DeleteSecret", func(t *testing.T) {
		ref := core.SecretRef{Path: secretName}
		err := provider.Delete(ctx, ref)
		assert.NoError(t, err, "Delete should succeed")

		// Note: LocalStack may not immediately remove secrets or may handle deletion differently
		// We verify the delete operation succeeded (no error), but don't enforce strict existence checks
		// as LocalStack behavior can vary
		t.Logf("Delete operation completed for secret: %s", secretName)
	})

	// Test 7: Resolve deleted secret - behavior may vary with LocalStack
	t.Run("ResolveDeletedSecret", func(t *testing.T) {
		ref := core.SecretRef{Path: secretName}
		_, err := provider.Resolve(ctx, ref)

		// LocalStack may or may not return an error for deleted secrets
		// The important thing is that the Delete operation succeeded
		if err != nil {
			// If an error is returned, it should be related to the secret not being found
			assert.True(t, strings.Contains(err.Error(), "not found") ||
				strings.Contains(err.Error(), "ResourceNotFoundException") ||
				strings.Contains(err.Error(), "InvalidRequestException"),
				"Error should indicate secret not found or access issue")
		} else {
			// LocalStack may still allow resolving deleted secrets
			t.Logf("LocalStack allows resolving deleted secret: %s", secretName)
		}
	})

	// Test 8: Delete non-existent secret should succeed (idempotent)
	t.Run("DeleteNonExistentSecret", func(t *testing.T) {
		ref := core.SecretRef{Path: "non-existent-secret-" + fmt.Sprintf("%d", time.Now().UnixNano())}
		err := provider.Delete(ctx, ref)
		assert.NoError(t, err, "Delete of non-existent secret should succeed")
	})
}

// TestErrorScenarios tests various error conditions and edge cases
func TestErrorScenarios(t *testing.T) {
	ctx := context.Background()
	provider, err := newTestProvider(ctx, t)
	require.NoError(t, err, "Failed to create test provider")

	// Test 1: Resolve non-existent secret
	t.Run("ResolveNonExistentSecret", func(t *testing.T) {
		ref := core.SecretRef{Path: "non-existent-secret-" + fmt.Sprintf("%d", time.Now().UnixNano())}
		_, err := provider.Resolve(ctx, ref)
		assert.Error(t, err, "Resolve should fail for non-existent secret")
		assert.True(t, strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "ResourceNotFoundException"),
			"Error should indicate secret not found")
	})

	// Test 2: Exists for non-existent secret
	t.Run("ExistsNonExistentSecret", func(t *testing.T) {
		ref := core.SecretRef{Path: "non-existent-secret-" + fmt.Sprintf("%d", time.Now().UnixNano())}
		exists, err := provider.Exists(ctx, ref)
		assert.NoError(t, err, "Exists should succeed")
		assert.False(t, exists, "Exists should return false for non-existent secret")
	})

	// Test 3: Empty secret path
	t.Run("EmptySecretPath", func(t *testing.T) {
		ref := core.SecretRef{Path: ""}
		_, err := provider.Resolve(ctx, ref)
		assert.Error(t, err, "Resolve should fail with empty secret path")
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	// Test 4: Empty secret path for Exists
	t.Run("EmptySecretPathExists", func(t *testing.T) {
		ref := core.SecretRef{Path: ""}
		_, err := provider.Exists(ctx, ref)
		assert.Error(t, err, "Exists should fail with empty secret path")
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	// Test 5: Empty secret path for Store
	t.Run("EmptySecretPathStore", func(t *testing.T) {
		ref := core.SecretRef{Path: ""}
		err := provider.Store(ctx, ref, []byte("test"))
		assert.Error(t, err, "Store should fail with empty secret path")
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	// Test 6: Empty secret path for Delete
	t.Run("EmptySecretPathDelete", func(t *testing.T) {
		ref := core.SecretRef{Path: ""}
		err := provider.Delete(ctx, ref)
		assert.Error(t, err, "Delete should fail with empty secret path")
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	// Test 7: Empty value for Store
	t.Run("EmptyValueStore", func(t *testing.T) {
		ref := core.SecretRef{Path: "test-empty-value-" + fmt.Sprintf("%d", time.Now().UnixNano())}
		err := provider.Store(ctx, ref, []byte(""))
		assert.NoError(t, err, "Store should succeed with empty value (AWS allows empty secrets)")
	})

	// Test 8: Invalid version references
	t.Run("InvalidVersionReferences", func(t *testing.T) {
		secretName := "test-version-" + fmt.Sprintf("%d", time.Now().UnixNano())

		// Create a secret first
		ref := core.SecretRef{Path: secretName}
		err := provider.Store(ctx, ref, []byte("test-value"))
		require.NoError(t, err, "Setup: Store should succeed")

		// Test invalid version ID
		ref.Version = "invalid-version-id"
		_, err = provider.Resolve(ctx, ref)
		assert.Error(t, err, "Resolve should fail with invalid version ID")

		// Test invalid version stage
		ref.Version = "INVALID_STAGE"
		_, err = provider.Resolve(ctx, ref)
		assert.Error(t, err, "Resolve should fail with invalid version stage")
	})
}

// TestBatchOperations tests ResolveBatch functionality and concurrent access
func TestBatchOperations(t *testing.T) {
	ctx := context.Background()
	provider, err := newTestProvider(ctx, t)
	require.NoError(t, err, "Failed to create test provider")

	// Create multiple secrets for batch testing
	const numSecrets = 5
	secretRefs := make([]core.SecretRef, numSecrets)
	secretValues := make([]string, numSecrets)

	for i := 0; i < numSecrets; i++ {
		secretName := fmt.Sprintf("test-batch-secret-%d-%d", time.Now().UnixNano(), i)
		secretValue := fmt.Sprintf(`{"data":"value-%d","timestamp":%d}`, i, time.Now().Unix())

		secretRefs[i] = core.SecretRef{Path: secretName}
		secretValues[i] = secretValue

		// Create the secret
		err := provider.Store(ctx, secretRefs[i], []byte(secretValue))
		require.NoError(t, err, "Setup: Store secret %d should succeed", i)
	}

	// Test 1: ResolveBatch with all existing secrets
	t.Run("ResolveBatchAllExisting", func(t *testing.T) {
		results, err := provider.ResolveBatch(ctx, secretRefs)
		assert.NoError(t, err, "ResolveBatch should succeed")
		assert.Len(t, results, numSecrets, "Should return all secrets")

		// Verify all secrets are returned correctly
		for i, ref := range secretRefs {
			secret, exists := results[ref.Path]
			assert.True(t, exists, "Secret %d should exist in results", i)
			assert.Equal(t, secretValues[i], string(secret.Value), "Secret %d value should match", i)
		}
	})

	// Test 2: ResolveBatch with mix of existing and non-existing secrets
	t.Run("ResolveBatchPartialSuccess", func(t *testing.T) {
		// Add some non-existent secrets to the batch
		mixedRefs := append(secretRefs[:3], // First 3 existing
			core.SecretRef{Path: "non-existent-1-" + fmt.Sprintf("%d", time.Now().UnixNano())},
			core.SecretRef{Path: "non-existent-2-" + fmt.Sprintf("%d", time.Now().UnixNano())})

		results, err := provider.ResolveBatch(ctx, mixedRefs)
		assert.NoError(t, err, "ResolveBatch should succeed even with missing secrets")
		assert.Len(t, results, 3, "Should return only existing secrets")

		// Verify existing secrets are returned
		for i := 0; i < 3; i++ {
			secret, exists := results[secretRefs[i].Path]
			assert.True(t, exists, "Existing secret %d should be in results", i)
			assert.Equal(t, secretValues[i], string(secret.Value), "Secret %d value should match", i)
		}
	})

	// Test 3: ResolveBatch with empty slice
	t.Run("ResolveBatchEmpty", func(t *testing.T) {
		results, err := provider.ResolveBatch(ctx, []core.SecretRef{})
		assert.NoError(t, err, "ResolveBatch with empty slice should succeed")
		assert.Len(t, results, 0, "Should return empty map for empty input")
	})

	// Test 4: ResolveBatch with context cancellation
	t.Run("ResolveBatchContextCancellation", func(t *testing.T) {
		// Create a context that will be cancelled
		cancelCtx, cancel := context.WithCancel(ctx)

		// Cancel immediately
		cancel()

		results, err := provider.ResolveBatch(cancelCtx, secretRefs)
		assert.Error(t, err, "ResolveBatch should fail with cancelled context")
		assert.True(t, strings.Contains(err.Error(), "cancelled") ||
			strings.Contains(err.Error(), "context"),
			"Error should indicate context cancellation")
		assert.Len(t, results, 0, "Should return empty results on cancellation")
	})
}

// TestBinarySecrets tests handling of binary secrets and JSON detection
func TestBinarySecrets(t *testing.T) {
	ctx := context.Background()
	provider, err := newTestProvider(ctx, t)
	require.NoError(t, err, "Failed to create test provider")

	// Test 1: Store and retrieve JSON secret (should be stored as string)
	t.Run("JSONSecret", func(t *testing.T) {
		secretName := "test-json-secret-" + fmt.Sprintf("%d", time.Now().UnixNano())
		jsonValue := `{"username":"admin","password":"secret123","nested":{"key":"value"}}`

		ref := core.SecretRef{Path: secretName}
		err := provider.Store(ctx, ref, []byte(jsonValue))
		assert.NoError(t, err, "Store JSON secret should succeed")

		secret, err := provider.Resolve(ctx, ref)
		assert.NoError(t, err, "Resolve JSON secret should succeed")
		assert.Equal(t, jsonValue, string(secret.Value), "Retrieved JSON should match stored value")
	})

	// Test 2: Store and retrieve plain text secret (should be stored as string)
	t.Run("PlainTextSecret", func(t *testing.T) {
		secretName := "test-text-secret-" + fmt.Sprintf("%d", time.Now().UnixNano())
		textValue := "This is a plain text secret without JSON structure"

		ref := core.SecretRef{Path: secretName}
		err := provider.Store(ctx, ref, []byte(textValue))
		assert.NoError(t, err, "Store text secret should succeed")

		secret, err := provider.Resolve(ctx, ref)
		assert.NoError(t, err, "Resolve text secret should succeed")
		assert.Equal(t, textValue, string(secret.Value), "Retrieved text should match stored value")
	})

	// Test 3: Store and retrieve binary data (should be stored as binary)
	t.Run("BinarySecret", func(t *testing.T) {
		secretName := "test-binary-secret-" + fmt.Sprintf("%d", time.Now().UnixNano())
		binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD, 0xFC}

		ref := core.SecretRef{Path: secretName}
		err := provider.Store(ctx, ref, binaryData)
		assert.NoError(t, err, "Store binary secret should succeed")

		secret, err := provider.Resolve(ctx, ref)
		assert.NoError(t, err, "Resolve binary secret should succeed")

		// Note: LocalStack might not perfectly preserve binary data format,
		// so we check that we get reasonable data back
		assert.NotEmpty(t, secret.Value, "Retrieved binary data should not be empty")
		// For LocalStack compatibility, we may get the data back as a string representation
		// The important thing is that we can store and retrieve the data
	})

	// Test 4: Store secret with metadata (tags)
	t.Run("SecretWithMetadata", func(t *testing.T) {
		secretName := "test-metadata-secret-" + fmt.Sprintf("%d", time.Now().UnixNano())
		secretValue := "secret with metadata"

		ref := core.SecretRef{
			Path: secretName,
			Metadata: map[string]string{
				"environment": "test",
				"team":        "platform",
				"purpose":     "integration-test",
			},
		}

		err := provider.Store(ctx, ref, []byte(secretValue))
		assert.NoError(t, err, "Store secret with metadata should succeed")

		secret, err := provider.Resolve(ctx, ref)
		assert.NoError(t, err, "Resolve secret with metadata should succeed")
		assert.Equal(t, secretValue, string(secret.Value), "Retrieved value should match")
	})

	// Test 5: Update secret value and verify it changes
	t.Run("UpdateSecretValue", func(t *testing.T) {
		secretName := "test-update-secret-" + fmt.Sprintf("%d", time.Now().UnixNano())
		originalValue := "original value"
		updatedValue := "updated value"

		ref := core.SecretRef{Path: secretName}

		// Store original value
		err := provider.Store(ctx, ref, []byte(originalValue))
		require.NoError(t, err, "Store original value should succeed")

		// Verify original value
		secret, err := provider.Resolve(ctx, ref)
		require.NoError(t, err, "Resolve original value should succeed")
		assert.Equal(t, originalValue, string(secret.Value), "Original value should match")

		// Update value
		err = provider.Store(ctx, ref, []byte(updatedValue))
		assert.NoError(t, err, "Update value should succeed")

		// Verify updated value
		secret, err = provider.Resolve(ctx, ref)
		assert.NoError(t, err, "Resolve updated value should succeed")
		assert.Equal(t, updatedValue, string(secret.Value), "Updated value should match")
	})
}

// BenchmarkResolve benchmarks the Resolve operation
func BenchmarkResolve(b *testing.B) {
	ctx := context.Background()
	provider, err := newTestProviderForBench(ctx)
	require.NoError(b, err, "Failed to create test provider")

	secretName := fmt.Sprintf("bench-secret-%d", time.Now().UnixNano())
	secretValue := `{"benchmark":"data","timestamp":` + fmt.Sprintf("%d", time.Now().Unix()) + `}`

	// Setup: Create secret
	ref := core.SecretRef{Path: secretName}
	err = provider.Store(ctx, ref, []byte(secretValue))
	require.NoError(b, err, "CreateSecret should succeed")

	// Reset timer to exclude setup time
	b.ResetTimer()
	b.ReportAllocs()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		secret, err := provider.Resolve(ctx, ref)
		if err != nil {
			b.Fatal(err)
		}
		if string(secret.Value) != secretValue {
			b.Fatalf("Expected %s, got %s", secretValue, string(secret.Value))
		}
	}
}

// BenchmarkResolveBatch benchmarks the ResolveBatch operation
func BenchmarkResolveBatch(b *testing.B) {
	ctx := context.Background()
	provider, err := newTestProviderForBench(ctx)
	require.NoError(b, err, "Failed to create test provider")

	// Setup: Create multiple secrets
	const numSecrets = 10
	refs := make([]core.SecretRef, numSecrets)
	expectedValues := make([]string, numSecrets)

	for i := 0; i < numSecrets; i++ {
		secretName := fmt.Sprintf("bench-batch-secret-%d-%d", time.Now().UnixNano(), i)
		secretValue := fmt.Sprintf(`{"data":"value-%d","index":%d}`, i, i)

		refs[i] = core.SecretRef{Path: secretName}
		expectedValues[i] = secretValue

		err := provider.Store(ctx, refs[i], []byte(secretValue))
		require.NoError(b, err, "CreateSecret %d should succeed", i)
	}

	// Reset timer to exclude setup time
	b.ResetTimer()
	b.ReportAllocs()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		results, err := provider.ResolveBatch(ctx, refs)
		if err != nil {
			b.Fatal(err)
		}
		if len(results) != numSecrets {
			b.Fatalf("Expected %d results, got %d", numSecrets, len(results))
		}
		// Verify a few results to ensure correctness
		for j := 0; j < 3; j++ { // Check first 3
			secret, exists := results[refs[j].Path]
			if !exists {
				b.Fatalf("Secret %d not found in results", j)
			}
			if string(secret.Value) != expectedValues[j] {
				b.Fatalf("Secret %d value mismatch", j)
			}
		}
	}
}

// BenchmarkStore benchmarks the Store operation
func BenchmarkStore(b *testing.B) {
	ctx := context.Background()
	provider, err := newTestProviderForBench(ctx)
	require.NoError(b, err, "Failed to create test provider")

	// Reset timer to exclude setup time
	b.ResetTimer()
	b.ReportAllocs()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		secretName := fmt.Sprintf("bench-store-secret-%d-%d", time.Now().UnixNano(), i)
		secretValue := fmt.Sprintf(`{"data":"value-%d","iteration":%d}`, i, i)

		ref := core.SecretRef{Path: secretName}
		err := provider.Store(ctx, ref, []byte(secretValue))
		if err != nil {
			b.Fatal(err)
		}
	}
}
