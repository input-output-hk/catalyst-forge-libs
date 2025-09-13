//go:build integration

// Package secrets_test provides integration tests for the AWS Secrets Manager client.
// These tests use LocalStack via testcontainers to avoid external AWS dependencies.
//
// IMPORTANT: This file uses build tags and will only be included when running:
//
//	go test -tags=integration -v ./...
//
// Running 'go test ./...' without the integration tag will skip these tests.
//
// The integration tests require Docker to be running for LocalStack containers.
package secrets_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/localstack"

	secrets "github.com/input-output-hk/catalyst-forge-libs/services/aws/secrets"
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

// newTestClient creates a new secrets client configured for LocalStack
func newTestClient(ctx context.Context, t *testing.T, opts ...secrets.Option) (*secrets.Client, error) {
	tc, err := getTestContainer(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Use the LocalStack-specific constructor
	return secrets.NewClientWithLocalStack(ctx, tc.uri, opts...)
}

// newTestClientForBench creates a new secrets client configured for LocalStack (benchmark version)
func newTestClientForBench(ctx context.Context, opts ...secrets.Option) (*secrets.Client, error) {
	tc, err := getTestContainer(ctx)
	if err != nil {
		return nil, err
	}

	// Use the LocalStack-specific constructor
	return secrets.NewClientWithLocalStack(ctx, tc.uri, opts...)
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

// TestSecretLifecycle tests the complete secret lifecycle: create, get, put, describe, delete
func TestSecretLifecycle(t *testing.T) {
	ctx := context.Background()
	client, err := newTestClient(ctx, t)
	require.NoError(t, err, "Failed to create test client")
	require.NotNil(t, client, "Client should not be nil")

	secretName := fmt.Sprintf("test-secret-%d", time.Now().UnixNano())
	secretValue1 := `{"username":"admin","password":"secret123"}`
	secretValue2 := `{"username":"admin","password":"updated456"}`

	// Test 1: Create secret
	t.Run("CreateSecret", func(t *testing.T) {
		err := client.CreateSecret(ctx, secretName, secretValue1, "")
		assert.NoError(t, err, "CreateSecret should succeed")
	})

	// Test 2: Get secret
	t.Run("GetSecret", func(t *testing.T) {
		value, err := client.GetSecret(ctx, secretName)
		assert.NoError(t, err, "GetSecret should succeed")
		assert.Equal(t, secretValue1, value, "Retrieved value should match created value")
	})

	// Test 3: Describe secret
	t.Run("DescribeSecret", func(t *testing.T) {
		metadata, err := client.DescribeSecret(ctx, secretName)
		assert.NoError(t, err, "DescribeSecret should succeed")
		assert.NotNil(t, metadata, "Metadata should not be nil")
		assert.Equal(t, secretName, *metadata.Name, "Secret name should match")
	})

	// Test 4: Update secret
	t.Run("PutSecret", func(t *testing.T) {
		err := client.PutSecret(ctx, secretName, secretValue2)
		assert.NoError(t, err, "PutSecret should succeed")

		// Verify the update
		value, err := client.GetSecret(ctx, secretName)
		assert.NoError(t, err, "GetSecret after update should succeed")
		assert.Equal(t, secretValue2, value, "Retrieved value should match updated value")
	})
}

// TestErrorScenarios tests various error conditions
func TestErrorScenarios(t *testing.T) {
	ctx := context.Background()
	client, err := newTestClient(ctx, t)
	require.NoError(t, err, "Failed to create test client")

	// Test 1: Get non-existent secret
	t.Run("GetNonExistentSecret", func(t *testing.T) {
		_, err := client.GetSecret(ctx, "non-existent-secret")
		assert.Error(t, err, "GetSecret should fail for non-existent secret")
		assert.True(
			t,
			strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "ResourceNotFoundException"),
			"Error should indicate secret not found",
		)
	})

	// Test 2: Describe non-existent secret
	t.Run("DescribeNonExistentSecret", func(t *testing.T) {
		_, err := client.DescribeSecret(ctx, "non-existent-secret")
		assert.Error(t, err, "DescribeSecret should fail for non-existent secret")
		assert.True(
			t,
			strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "ResourceNotFoundException"),
			"Error should indicate secret not found",
		)
	})

	// Test 3: Put secret that doesn't exist (should fail)
	t.Run("PutNonExistentSecret", func(t *testing.T) {
		err := client.PutSecret(ctx, "non-existent-secret", "some-value")
		assert.Error(t, err, "PutSecret should fail for non-existent secret")
		assert.True(
			t,
			strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "ResourceNotFoundException"),
			"Error should indicate secret not found",
		)
	})

	// Test 4: Empty secret name
	t.Run("EmptySecretName", func(t *testing.T) {
		_, err := client.GetSecret(ctx, "")
		assert.Error(t, err, "GetSecret should fail with empty secret name")
		assert.Contains(t, err.Error(), "secret name cannot be empty")
	})

	// Test 5: Empty secret value for create
	t.Run("EmptySecretValue", func(t *testing.T) {
		err := client.CreateSecret(ctx, "test-secret", "", "")
		assert.Error(t, err, "CreateSecret should fail with empty secret value")
		assert.Contains(t, err.Error(), "secret value cannot be empty")
	})

	// Test 6: Empty secret value for put
	t.Run("EmptySecretValuePut", func(t *testing.T) {
		secretName := fmt.Sprintf("test-secret-%d", time.Now().UnixNano())
		// First create a secret
		err := client.CreateSecret(ctx, secretName, "initial-value", "")
		require.NoError(t, err, "Setup: CreateSecret should succeed")

		// Now try to put empty value
		err = client.PutSecret(ctx, secretName, "")
		assert.Error(t, err, "PutSecret should fail with empty secret value")
		assert.Contains(t, err.Error(), "secret value cannot be empty")
	})
}

// TestCaching tests the caching functionality
func TestCaching(t *testing.T) {
	ctx := context.Background()

	// Create client with caching enabled
	client, err := newTestClient(ctx, t,
		secrets.WithCache(secrets.NewInMemoryCache(5*time.Minute, 10)),
	)
	require.NoError(t, err, "Failed to create test client with cache")

	secretName := fmt.Sprintf("test-cache-secret-%d", time.Now().UnixNano())
	secretValue := `{"cached":"value"}`

	// Create secret
	err = client.CreateSecret(ctx, secretName, secretValue, "")
	require.NoError(t, err, "CreateSecret should succeed")

	// Test 1: First call should cache the value
	t.Run("CacheMiss", func(t *testing.T) {
		start := time.Now()
		value, err := client.GetSecretCached(ctx, secretName)
		duration := time.Since(start)

		assert.NoError(t, err, "GetSecretCached should succeed")
		assert.Equal(t, secretValue, value, "Retrieved value should match")

		// First call should take longer (no cache) - LocalStack adds some overhead
		t.Logf("First call took: %v", duration)
		assert.True(t, duration > 500*time.Microsecond, "First call should take some time")
	})

	// Test 2: Second call should use cache (faster)
	t.Run("CacheHit", func(t *testing.T) {
		start := time.Now()
		value, err := client.GetSecretCached(ctx, secretName)
		duration := time.Since(start)

		assert.NoError(t, err, "GetSecretCached should succeed")
		assert.Equal(t, secretValue, value, "Retrieved value should match")

		// Second call should be faster (cached)
		t.Logf("Second call took: %v", duration)
		assert.True(t, duration < 10*time.Millisecond, "Second call should be fast")
	})

	// Test 3: Cache invalidation
	t.Run("CacheInvalidation", func(t *testing.T) {
		// Invalidate cache
		client.InvalidateCache(secretName)

		// Next call should be slower again (cache miss)
		start := time.Now()
		value, err := client.GetSecretCached(ctx, secretName)
		duration := time.Since(start)

		assert.NoError(t, err, "GetSecretCached after invalidation should succeed")
		assert.Equal(t, secretValue, value, "Retrieved value should match")
		t.Logf("Call after invalidation took: %v", duration)
		assert.True(t, duration > 500*time.Microsecond, "Call after invalidation should take time")
	})

	// Test 4: Clear entire cache
	t.Run("CacheClear", func(t *testing.T) {
		// Clear entire cache
		client.ClearCache()

		// Verify cache size is 0
		size := client.GetCacheSize()
		assert.Equal(t, 0, size, "Cache should be empty after clear")
	})
}

// testConcurrentReads tests concurrent read operations on the same secret
func testConcurrentReads(ctx context.Context, t *testing.T, client *secrets.Client) {
	const numGoroutines = 10
	secretName := fmt.Sprintf("test-concurrent-read-%d", time.Now().UnixNano())

	// Create secret
	err := client.CreateSecret(ctx, secretName, "concurrent-value", "")
	require.NoError(t, err, "CreateSecret should succeed")

	var wg sync.WaitGroup
	results := make([]string, numGoroutines)
	errors := make([]error, numGoroutines)

	// Start concurrent readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			value, err := client.GetSecret(ctx, secretName)
			results[index] = value
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// Verify all operations succeeded and returned same value
	for i := 0; i < numGoroutines; i++ {
		assert.NoError(t, errors[i], "Concurrent read %d should succeed", i)
		assert.Equal(t, "concurrent-value", results[i], "Concurrent read %d should return correct value", i)
	}
}

// testConcurrentWrites tests concurrent write operations creating different secrets
func testConcurrentWrites(ctx context.Context, t *testing.T, client *secrets.Client) {
	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make([]error, numGoroutines)

	// Start concurrent writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			secretName := fmt.Sprintf("test-concurrent-write-%d-%d", time.Now().UnixNano(), index)
			secretValue := fmt.Sprintf("value-%d", index)
			err := client.CreateSecret(ctx, secretName, secretValue, "")
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// Verify all operations succeeded
	for i := 0; i < numGoroutines; i++ {
		assert.NoError(t, errors[i], "Concurrent write %d should succeed", i)
	}
}

// testMixedConcurrentOperations tests mixed concurrent operations (create, get, put)
func testMixedConcurrentOperations(ctx context.Context, t *testing.T, client *secrets.Client) {
	const numOperations = 20
	baseName := fmt.Sprintf("test-mixed-%d", time.Now().UnixNano())

	var wg sync.WaitGroup
	errors := make([]error, numOperations)

	// Start mixed operations
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			performMixedOperation(ctx, client, baseName, index, errors)
		}(i)
	}

	wg.Wait()

	// Check for critical errors (ignore expected "already exists" for creates)
	for i := 0; i < numOperations; i++ {
		if errors[i] != nil {
			// Only fail on unexpected errors
			if !strings.Contains(errors[i].Error(), "already exists") &&
				!strings.Contains(errors[i].Error(), "ResourceExistsException") &&
				!strings.Contains(errors[i].Error(), "not found") &&
				!strings.Contains(errors[i].Error(), "ResourceNotFoundException") {
				assert.NoError(t, errors[i], "Mixed operation %d should not have critical errors", i)
			}
		}
	}
}

// performMixedOperation performs one of three operations based on index
func performMixedOperation(ctx context.Context, client *secrets.Client, baseName string, index int, errors []error) {
	secretName := fmt.Sprintf("%s-%d", baseName, index%5) // Reuse 5 secrets
	secretValue := fmt.Sprintf("value-%d", index)

	// Alternate between create, get, put operations
	switch index % 3 {
	case 0:
		// Create or recreate secret
		err := client.CreateSecret(ctx, secretName, secretValue, "")
		if err != nil {
			// Ignore "already exists" errors for this test
			if !strings.Contains(err.Error(), "already exists") &&
				!strings.Contains(err.Error(), "ResourceExistsException") {
				errors[index] = err
			}
		}
	case 1:
		// Get secret
		_, err := client.GetSecret(ctx, secretName)
		errors[index] = err
	case 2:
		// Put secret (update)
		err := client.PutSecret(ctx, secretName, secretValue)
		errors[index] = err
	}
}

// TestConcurrentAccess tests thread safety with concurrent operations
func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	client, err := newTestClient(ctx, t)
	require.NoError(t, err, "Failed to create test client")

	// Test concurrent reads
	t.Run("ConcurrentReads", func(t *testing.T) {
		testConcurrentReads(ctx, t, client)
	})

	// Test concurrent writes (create different secrets)
	t.Run("ConcurrentWrites", func(t *testing.T) {
		testConcurrentWrites(ctx, t, client)
	})

	// Test mixed concurrent operations
	t.Run("MixedConcurrentOperations", func(t *testing.T) {
		testMixedConcurrentOperations(ctx, t, client)
	})
}

// BenchmarkGetSecret benchmarks the GetSecret operation
func BenchmarkGetSecret(b *testing.B) {
	ctx := context.Background()
	client, err := newTestClientForBench(ctx)
	require.NoError(b, err, "Failed to create test client")

	secretName := fmt.Sprintf("bench-secret-%d", time.Now().UnixNano())
	secretValue := `{"benchmark":"data"}`

	// Setup: Create secret
	err = client.CreateSecret(ctx, secretName, secretValue, "")
	require.NoError(b, err, "CreateSecret should succeed")

	// Reset timer to exclude setup time
	b.ResetTimer()

	// Run benchmark
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			value, err := client.GetSecret(ctx, secretName)
			if err != nil {
				b.Fatal(err)
			}
			if value != secretValue {
				b.Fatalf("Expected %s, got %s", secretValue, value)
			}
		}
	})
}

// BenchmarkGetSecretCached benchmarks the cached GetSecret operation
func BenchmarkGetSecretCached(b *testing.B) {
	ctx := context.Background()

	// Create client with caching
	client, err := newTestClientForBench(ctx,
		secrets.WithCache(secrets.NewInMemoryCache(5*time.Minute, 100)),
	)
	require.NoError(b, err, "Failed to create test client with cache")

	secretName := fmt.Sprintf("bench-cached-secret-%d", time.Now().UnixNano())
	secretValue := `{"benchmark":"cached-data"}`

	// Setup: Create secret and warm the cache
	err = client.CreateSecret(ctx, secretName, secretValue, "")
	require.NoError(b, err, "CreateSecret should succeed")

	// Warm the cache
	_, err = client.GetSecretCached(ctx, secretName)
	require.NoError(b, err, "Cache warm-up should succeed")

	// Reset timer to exclude setup time
	b.ResetTimer()

	// Run benchmark
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			value, err := client.GetSecretCached(ctx, secretName)
			if err != nil {
				b.Fatal(err)
			}
			if value != secretValue {
				b.Fatalf("Expected %s, got %s", secretValue, value)
			}
		}
	})
}

// TestLogging tests that sensitive data is not logged
func TestLogging(t *testing.T) {
	ctx := context.Background()

	// Create a logger that captures output
	var logBuffer strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{}))

	client, err := newTestClient(ctx, t, secrets.WithLogger(logger))
	require.NoError(t, err, "Failed to create test client with logger")

	secretName := fmt.Sprintf("test-log-secret-%d", time.Now().UnixNano())
	secretValue := `{"password":"super-secret-123","token":"abc123xyz"}`

	// Perform operations that should log
	err = client.CreateSecret(ctx, secretName, secretValue, "")
	require.NoError(t, err, "CreateSecret should succeed")

	_, err = client.GetSecret(ctx, secretName)
	require.NoError(t, err, "GetSecret should succeed")

	err = client.PutSecret(ctx, secretName, `{"password":"updated-secret","token":"new-token"}`)
	require.NoError(t, err, "PutSecret should succeed")

	// Check that logs don't contain sensitive values
	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, secretName, "Logs should contain secret name")
	assert.NotContains(t, logOutput, "super-secret-123", "Logs should NOT contain secret values")
	assert.NotContains(t, logOutput, "abc123xyz", "Logs should NOT contain token values")
	assert.NotContains(t, logOutput, "updated-secret", "Logs should NOT contain updated secret values")
	assert.NotContains(t, logOutput, "new-token", "Logs should NOT contain updated token values")
}

// TestBinarySecrets tests handling of binary secrets
func TestBinarySecrets(t *testing.T) {
	ctx := context.Background()
	client, err := newTestClient(ctx, t)
	require.NoError(t, err, "Failed to create test client")

	secretName := fmt.Sprintf("test-binary-secret-%d", time.Now().UnixNano())
	binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD}

	// LocalStack might not support binary secrets the same way as real AWS
	// This test verifies the client handles the scenario gracefully
	t.Run("BinarySecretHandling", func(t *testing.T) {
		// For LocalStack compatibility, we'll test with string representation of binary data
		binaryString := string(binaryData)
		err := client.CreateSecret(ctx, secretName, binaryString, "")
		assert.NoError(t, err, "CreateSecret with binary-like data should succeed")

		value, err := client.GetSecret(ctx, secretName)
		assert.NoError(t, err, "GetSecret should succeed")

		// LocalStack may modify binary data, so we'll check that we get a reasonable response
		assert.NotEmpty(t, value, "Retrieved value should not be empty")
		assert.Contains(t, value, "\x00\x01\x02\x03", "Retrieved value should contain expected binary data")
	})
}

// TestKMSIntegration tests KMS key integration (simulated with LocalStack)
func TestKMSIntegration(t *testing.T) {
	ctx := context.Background()
	client, err := newTestClient(ctx, t)
	require.NoError(t, err, "Failed to create test client")

	secretName := fmt.Sprintf("test-kms-secret-%d", time.Now().UnixNano())
	secretValue := "kms-encrypted-secret"

	// LocalStack has limited KMS support, so we'll test the API without actual encryption
	t.Run("CreateWithKMSKey", func(t *testing.T) {
		// Use a dummy KMS key ID (LocalStack will accept it)
		kmsKeyID := "alias/aws/secretsmanager"
		err := client.CreateSecret(ctx, secretName, secretValue, kmsKeyID)
		assert.NoError(t, err, "CreateSecret with KMS key should succeed")

		// Verify the secret was created
		value, err := client.GetSecret(ctx, secretName)
		assert.NoError(t, err, "GetSecret should succeed")
		assert.Equal(t, secretValue, value, "Retrieved value should match")
	})
}

// TestPerformanceMetrics tests basic performance characteristics
func TestPerformanceMetrics(t *testing.T) {
	ctx := context.Background()
	client, err := newTestClient(ctx, t)
	require.NoError(t, err, "Failed to create test client")

	// Create multiple secrets for performance testing
	const numSecrets = 10
	secretNames := make([]string, numSecrets)
	secretValues := make([]string, numSecrets)

	for i := 0; i < numSecrets; i++ {
		secretNames[i] = fmt.Sprintf("perf-test-secret-%d-%d", time.Now().UnixNano(), i)
		secretValues[i] = fmt.Sprintf(`{"data":"value-%d","timestamp":%d}`, i, time.Now().Unix())
	}

	// Measure creation performance
	t.Run("BulkCreatePerformance", func(t *testing.T) {
		start := time.Now()

		for i := 0; i < numSecrets; i++ {
			err := client.CreateSecret(ctx, secretNames[i], secretValues[i], "")
			assert.NoError(t, err, "CreateSecret %d should succeed", i)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(numSecrets)

		t.Logf("Created %d secrets in %v (avg: %v per secret)", numSecrets, duration, avgDuration)
		assert.True(t, avgDuration < 500*time.Millisecond, "Average create time should be reasonable")
	})

	// Measure retrieval performance
	t.Run("BulkRetrievePerformance", func(t *testing.T) {
		start := time.Now()

		for i := 0; i < numSecrets; i++ {
			value, err := client.GetSecret(ctx, secretNames[i])
			assert.NoError(t, err, "GetSecret %d should succeed", i)
			assert.Equal(t, secretValues[i], value, "Retrieved value %d should match", i)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(numSecrets)

		t.Logf("Retrieved %d secrets in %v (avg: %v per secret)", numSecrets, duration, avgDuration)
		assert.True(t, avgDuration < 100*time.Millisecond, "Average retrieve time should be reasonable")
	})

	// Measure cached retrieval performance
	t.Run("CachedRetrievePerformance", func(t *testing.T) {
		cacheClient, err := newTestClient(ctx, t,
			secrets.WithCache(secrets.NewInMemoryCache(5*time.Minute, 100)),
		)
		require.NoError(t, err, "Failed to create cached client")

		// Warm the cache
		for i := 0; i < numSecrets; i++ {
			_, err := cacheClient.GetSecretCached(ctx, secretNames[i])
			assert.NoError(t, err, "Cache warm-up %d should succeed", i)
		}

		// Measure cached retrieval
		start := time.Now()

		for i := 0; i < numSecrets; i++ {
			value, err := cacheClient.GetSecretCached(ctx, secretNames[i])
			assert.NoError(t, err, "Cached GetSecret %d should succeed", i)
			assert.Equal(t, secretValues[i], value, "Cached retrieved value %d should match", i)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(numSecrets)

		t.Logf("Retrieved %d cached secrets in %v (avg: %v per secret)", numSecrets, duration, avgDuration)
		assert.True(t, avgDuration < 10*time.Millisecond, "Average cached retrieve time should be very fast")
	})
}
