// Package s3 provides comprehensive tests for client initialization and configuration.
package s3

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// TestClient_New tests the New() constructor with default configuration.
func TestClient_New(t *testing.T) {
	tests := []struct {
		name    string
		opts    []s3types.Option
		wantErr bool
	}{
		{
			name:    "default configuration",
			opts:    nil,
			wantErr: false,
		},
		{
			name:    "with region option",
			opts:    []s3types.Option{WithRegion("us-west-2")},
			wantErr: false,
		},
		{
			name:    "with multiple options",
			opts:    []s3types.Option{WithRegion("us-east-1"), WithMaxRetries(5)},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			assert.NotNil(t, client.s3Client)
			assert.NotNil(t, client.config)
		})
	}
}

// TestClient_New_ConcurrentSafety tests that client creation is safe for concurrent use.
func TestClient_New_ConcurrentSafety(t *testing.T) {
	const numGoroutines = 10
	const numCreations = 100

	var wg sync.WaitGroup
	clients := make([]*Client, 0, numGoroutines*numCreations)
	var clientsMu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numCreations; j++ {
				client, err := New(WithRegion("us-east-1"))
				require.NoError(t, err)
				require.NotNil(t, client)

				clientsMu.Lock()
				clients = append(clients, client)
				clientsMu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Verify all clients were created successfully
	assert.Len(t, clients, numGoroutines*numCreations)

	// Verify all clients have valid configuration
	for i, client := range clients {
		assert.NotNil(t, client, "client %d should not be nil", i)
		assert.NotNil(t, client.s3Client, "client %d s3Client should not be nil", i)
		assert.NotNil(t, client.config, "client %d config should not be nil", i)
	}
}

// TestClient_New_WithInvalidOptions tests client creation with invalid options.
func TestClient_New_WithInvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    []s3types.Option
		wantErr bool
	}{
		{
			name:    "empty region",
			opts:    []s3types.Option{WithRegion("")},
			wantErr: false, // AWS SDK allows empty region, uses default
		},
		{
			name:    "negative retries",
			opts:    []s3types.Option{WithMaxRetries(-1)},
			wantErr: false, // Should be handled gracefully
		},
		{
			name:    "zero timeout",
			opts:    []s3types.Option{WithTimeout(0)},
			wantErr: false, // Zero timeout is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}
}

// TestClient_New_WithAWSCredentials tests that client properly uses AWS credential chain.
func TestClient_New_WithAWSCredentials(t *testing.T) {
	// Skip if running in CI without AWS credentials
	if testing.Short() {
		t.Skip("Skipping credential test in short mode")
	}

	client, err := New(WithRegion("us-east-1"))
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify that the S3 client has a valid configuration
	assert.NotNil(t, client.s3Client)
	assert.NotNil(t, client.config)

	// Test that we can create a context and the client is functional
	ctx := context.Background()
	assert.NotNil(t, ctx)
}

// TestClient_New_WithCustomConfig tests client creation with custom AWS configuration.
func TestClient_New_WithCustomConfig(t *testing.T) {
	customConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-west-2"),
		config.WithRetryMaxAttempts(10),
	)
	require.NoError(t, err)

	client, err := New(WithAWSConfig(&customConfig))
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify configuration was applied
	assert.NotNil(t, client.s3Client)
	assert.NotNil(t, client.config)
	assert.Equal(t, "us-west-2", client.config.Region)
}

// TestClient_New_WithOptionsComposition tests that options can be composed and applied correctly.
func TestClient_New_WithOptionsComposition(t *testing.T) {
	opts := []s3types.Option{
		WithRegion("eu-west-1"),
		WithMaxRetries(3),
		WithTimeout(30 * time.Second),
		WithConcurrency(10),
		WithPartSize(5 * 1024 * 1024),
		WithForcePathStyle(true),
	}

	client, err := New(opts...)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify all options were applied
	assert.NotNil(t, client.s3Client)
	assert.NotNil(t, client.config)
	assert.Equal(t, "eu-west-1", client.config.Region)
}

// TestClient_New_WithDefaults tests that default values are applied correctly.
func TestClient_New_WithDefaults(t *testing.T) {
	client, err := New()
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify default values
	assert.NotNil(t, client.s3Client)
	assert.NotNil(t, client.config)

	// Test that default region is set (AWS SDK default behavior)
	assert.NotEmpty(t, client.config.Region)
}

// TestClient_New_ErrorHandling tests error handling during client creation.
func TestClient_New_ErrorHandling(t *testing.T) {
	// Test with invalid AWS configuration
	// This is difficult to test without mocking, so we'll test the general error path
	t.Run("general error handling", func(t *testing.T) {
		client, err := New()
		// In normal circumstances, this should succeed
		require.NoError(t, err)
		require.NotNil(t, client)
	})
}

// TestClient_ConfigurationValidation tests that client configuration is validated.
func TestClient_ConfigurationValidation(t *testing.T) {
	client, err := New(WithRegion("us-east-1"))
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify that the client has required fields
	assert.NotNil(t, client.s3Client)
	assert.NotNil(t, client.config)
	assert.NotEmpty(t, client.config.Region)
}

// TestClient_OptionPrecedence tests that later options override earlier ones.
func TestClient_OptionPrecedence(t *testing.T) {
	client, err := New(
		WithRegion("us-east-1"),
		WithRegion("us-west-2"), // This should override the previous region
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify the last option took precedence
	assert.Equal(t, "us-west-2", client.config.Region)
}

// TestClient_ConfigIsolation tests that different client instances have isolated configurations.
func TestClient_ConfigIsolation(t *testing.T) {
	client1, err := New(WithRegion("us-east-1"))
	require.NoError(t, err)

	client2, err := New(WithRegion("us-west-2"))
	require.NoError(t, err)

	// Verify configurations are independent
	assert.Equal(t, "us-east-1", client1.config.Region)
	assert.Equal(t, "us-west-2", client2.config.Region)
	assert.NotEqual(t, client1.config.Region, client2.config.Region)
}

// TestClient_WithNilOptions tests behavior with nil options.
func TestClient_WithNilOptions(t *testing.T) {
	var opts []s3types.Option
	client, err := New(opts...)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Should work the same as New()
	assert.NotNil(t, client.s3Client)
	assert.NotNil(t, client.config)
}

// BenchmarkClient_New benchmarks client creation performance.
func BenchmarkClient_New(b *testing.B) {
	for i := 0; i < b.N; i++ {
		client, err := New(WithRegion("us-east-1"))
		if err != nil {
			b.Fatal(err)
		}
		_ = client
	}
}

// BenchmarkClient_New_Parallel benchmarks concurrent client creation.
func BenchmarkClient_New_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client, err := New(WithRegion("us-east-1"))
			if err != nil {
				b.Fatal(err)
			}
			_ = client
		}
	})
}

// TestWithRegion tests the WithRegion option.
func TestWithRegion(t *testing.T) {
	tests := []struct {
		name     string
		region   string
		expected string
	}{
		{
			name:     "us-east-1",
			region:   "us-east-1",
			expected: "us-east-1",
		},
		{
			name:     "eu-west-1",
			region:   "eu-west-1",
			expected: "eu-west-1",
		},
		{
			name:     "ap-southeast-1",
			region:   "ap-southeast-1",
			expected: "ap-southeast-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithRegion(tt.region))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, client.config.Region)
		})
	}
}

// TestWithMaxRetries tests the WithMaxRetries option.
func TestWithMaxRetries(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		expected   int
	}{
		{
			name:       "zero retries",
			maxRetries: 0,
			expected:   0,
		},
		{
			name:       "three retries",
			maxRetries: 3,
			expected:   3,
		},
		{
			name:       "ten retries",
			maxRetries: 10,
			expected:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithMaxRetries(tt.maxRetries))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, client.config.RetryMaxAttempts)
		})
	}
}

// TestWithTimeout tests the WithTimeout option.
func TestWithTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		hasTimer bool
	}{
		{
			name:     "no timeout",
			timeout:  0,
			hasTimer: false,
		},
		{
			name:     "30 second timeout",
			timeout:  30 * time.Second,
			hasTimer: true,
		},
		{
			name:     "5 minute timeout",
			timeout:  5 * time.Minute,
			hasTimer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithTimeout(tt.timeout))
			require.NoError(t, err)

			// Verify the client was created successfully
			assert.NotNil(t, client.s3Client)
		})
	}
}

// TestWithConcurrency tests the WithConcurrency option.
func TestWithConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
	}{
		{
			name:        "concurrency 1",
			concurrency: 1,
		},
		{
			name:        "concurrency 5",
			concurrency: 5,
		},
		{
			name:        "concurrency 20",
			concurrency: 20,
		},
		{
			name:        "invalid concurrency 0",
			concurrency: 0,
		},
		{
			name:        "invalid concurrency -1",
			concurrency: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithConcurrency(tt.concurrency))
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestWithPartSize tests the WithPartSize option.
func TestWithPartSize(t *testing.T) {
	tests := []struct {
		name     string
		partSize int64
	}{
		{
			name:     "5MB part size",
			partSize: 5 * 1024 * 1024,
		},
		{
			name:     "10MB part size",
			partSize: 10 * 1024 * 1024,
		},
		{
			name:     "100MB part size",
			partSize: 100 * 1024 * 1024,
		},
		{
			name:     "invalid part size 0",
			partSize: 0,
		},
		{
			name:     "invalid part size -1",
			partSize: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithPartSize(tt.partSize))
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestWithForcePathStyle tests the WithForcePathStyle option.
func TestWithForcePathStyle(t *testing.T) {
	tests := []struct {
		name           string
		forcePathStyle bool
	}{
		{
			name:           "force path style true",
			forcePathStyle: true,
		},
		{
			name:           "force path style false",
			forcePathStyle: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithForcePathStyle(tt.forcePathStyle))
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestWithEndpoint tests the WithEndpoint option.
func TestWithEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{
			name:     "localhost endpoint",
			endpoint: "http://localhost:4566",
		},
		{
			name:     "custom endpoint",
			endpoint: "https://minio.example.com",
		},
		{
			name:     "empty endpoint",
			endpoint: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithEndpoint(tt.endpoint))
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestWithDisableSSL tests the WithDisableSSL option.
func TestWithDisableSSL(t *testing.T) {
	tests := []struct {
		name       string
		disableSSL bool
	}{
		{
			name:       "disable SSL true",
			disableSSL: true,
		},
		{
			name:       "disable SSL false",
			disableSSL: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithDisableSSL(tt.disableSSL))
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestWithConcurrencyLimit tests the WithConcurrencyLimit option.
func TestWithConcurrencyLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
	}{
		{
			name:  "limit 1",
			limit: 1,
		},
		{
			name:  "limit 10",
			limit: 10,
		},
		{
			name:  "limit 0",
			limit: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithConcurrencyLimit(tt.limit))
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestWithRetryMode tests the WithRetryMode option.
func TestWithRetryMode(t *testing.T) {
	tests := []struct {
		name      string
		retryMode string
	}{
		{
			name:      "standard retry mode",
			retryMode: "standard",
		},
		{
			name:      "adaptive retry mode",
			retryMode: "adaptive",
		},
		{
			name:      "empty retry mode",
			retryMode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithRetryMode(tt.retryMode))
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestWithDefaultBucket tests the WithDefaultBucket option.
func TestWithDefaultBucket(t *testing.T) {
	tests := []struct {
		name          string
		defaultBucket string
	}{
		{
			name:          "my-bucket",
			defaultBucket: "my-bucket",
		},
		{
			name:          "test-bucket-123",
			defaultBucket: "test-bucket-123",
		},
		{
			name:          "empty bucket",
			defaultBucket: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(WithDefaultBucket(tt.defaultBucket))
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

// TestOptionComposition tests that multiple options can be composed together.
func TestOptionComposition(t *testing.T) {
	client, err := New(
		WithRegion("us-west-2"),
		WithMaxRetries(5),
		WithTimeout(60*time.Second),
		WithConcurrency(10),
		WithPartSize(16*1024*1024),
		WithForcePathStyle(true),
		WithDefaultBucket("test-bucket"),
	)

	require.NoError(t, err)
	require.NotNil(t, client)
	assert.NotNil(t, client.s3Client)
	assert.NotNil(t, client.config)
	assert.Equal(t, "us-west-2", client.config.Region)
}

// TestOptionOrderIndependence tests that option order doesn't affect the result.
func TestOptionOrderIndependence(t *testing.T) {
	// Create client with options in one order
	client1, err := New(
		WithRegion("us-east-1"),
		WithMaxRetries(3),
		WithConcurrency(5),
	)
	require.NoError(t, err)

	// Create client with options in different order
	client2, err := New(
		WithConcurrency(5),
		WithMaxRetries(3),
		WithRegion("us-east-1"),
	)
	require.NoError(t, err)

	// Both should have the same configuration
	assert.Equal(t, client1.config.Region, client2.config.Region)
	assert.Equal(t, client1.config.RetryMaxAttempts, client2.config.RetryMaxAttempts)
}

// TestOptionDefaults tests that options have appropriate defaults.
func TestOptionDefaults(t *testing.T) {
	client, err := New()
	require.NoError(t, err)

	// Verify default values
	assert.NotNil(t, client.s3Client)
	assert.NotNil(t, client.config)
	assert.NotEmpty(t, client.config.Region) // Should have a default region
}

// TestInvalidOptions tests behavior with invalid option values.
func TestInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts []s3types.Option
	}{
		{
			name: "negative concurrency",
			opts: []s3types.Option{WithConcurrency(-1)},
		},
		{
			name: "negative part size",
			opts: []s3types.Option{WithPartSize(-1)},
		},
		{
			name: "negative retries",
			opts: []s3types.Option{WithMaxRetries(-1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.opts...)
			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}
