// Package secrets provides tests for the AWS Secrets Manager client.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockManagerAPI implements ManagerAPI for testing
type mockManagerAPI struct {
	getSecretValueFunc func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	putSecretValueFunc func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	createSecretFunc   func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	describeSecretFunc func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
}

func (m *mockManagerAPI) GetSecretValue(
	ctx context.Context,
	params *secretsmanager.GetSecretValueInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.GetSecretValueOutput, error) {
	if m.getSecretValueFunc != nil {
		return m.getSecretValueFunc(ctx, params, optFns...)
	}
	return nil, fmt.Errorf("GetSecretValue not implemented")
}

func (m *mockManagerAPI) PutSecretValue(
	ctx context.Context,
	params *secretsmanager.PutSecretValueInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.PutSecretValueOutput, error) {
	if m.putSecretValueFunc != nil {
		return m.putSecretValueFunc(ctx, params, optFns...)
	}
	return nil, fmt.Errorf("PutSecretValue not implemented")
}

func (m *mockManagerAPI) CreateSecret(
	ctx context.Context,
	params *secretsmanager.CreateSecretInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.CreateSecretOutput, error) {
	if m.createSecretFunc != nil {
		return m.createSecretFunc(ctx, params, optFns...)
	}
	return nil, fmt.Errorf("CreateSecret not implemented")
}

func (m *mockManagerAPI) DescribeSecret(
	ctx context.Context,
	params *secretsmanager.DescribeSecretInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.DescribeSecretOutput, error) {
	if m.describeSecretFunc != nil {
		return m.describeSecretFunc(ctx, params, optFns...)
	}
	return nil, fmt.Errorf("DescribeSecret not implemented")
}

// mockCache implements a simple cache interface for testing
type mockCache struct {
	ttl time.Duration
}

func (m *mockCache) Get(key string) (any, bool)                   { return nil, false }
func (m *mockCache) Set(key string, value any, ttl time.Duration) {}

// Helper types and functions for testing
type logEntry struct {
	level      string
	msg        string
	secretName string
}

type testLogHandler struct {
	logs *[]logEntry
}

func (h *testLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

//nolint:gocritic // slog.Handler interface requires slog.Record by value
func (h *testLogHandler) Handle(ctx context.Context, r slog.Record) error {
	entry := logEntry{
		level: r.Level.String(),
		msg:   r.Message,
	}

	// Extract secret name from log attributes
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "secret_name" {
			entry.secretName = a.Value.String()
		}
		return true
	})

	*h.logs = append(*h.logs, entry)
	return nil
}

func (h *testLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *testLogHandler) WithGroup(name string) slog.Handler {
	return h
}

func stringPtr(s string) *string {
	return &s
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		options     []Option
		validate    func(t *testing.T, client *Client, err error)
		expectError bool
	}{
		{
			name:    "successful creation with defaults",
			ctx:     context.Background(),
			options: []Option{},
			validate: func(t *testing.T, client *Client, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.api)
				assert.Nil(t, client.logger)
				assert.Nil(t, client.cache)
				assert.Nil(t, client.retryer)
			},
			expectError: false,
		},
		{
			name: "successful creation with custom logger",
			ctx:  context.Background(),
			options: []Option{
				WithLogger(slog.New(slog.NewTextHandler(nil, nil))),
			},
			validate: func(t *testing.T, client *Client, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.logger)
				assert.Nil(t, client.cache)
				assert.Nil(t, client.retryer)
			},
			expectError: false,
		},
		{
			name: "successful creation with cache",
			ctx:  context.Background(),
			options: []Option{
				WithCache(&mockCache{ttl: time.Minute}),
			},
			validate: func(t *testing.T, client *Client, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.cache)
				assert.Nil(t, client.logger)
				assert.Nil(t, client.retryer)
			},
			expectError: false,
		},
		{
			name: "successful creation with custom retryer",
			ctx:  context.Background(),
			options: []Option{
				WithCustomRetryer(&CustomRetryer{maxAttempts: 5}),
			},
			validate: func(t *testing.T, client *Client, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.retryer)
				assert.Equal(t, 5, client.retryer.MaxAttempts())
				assert.Nil(t, client.logger)
				assert.Nil(t, client.cache)
			},
			expectError: false,
		},
		{
			name: "successful creation with all options",
			ctx:  context.Background(),
			options: []Option{
				WithLogger(slog.New(slog.NewTextHandler(nil, nil))),
				WithCache(&mockCache{ttl: time.Hour}),
				WithCustomRetryer(&CustomRetryer{maxAttempts: 3}),
			},
			validate: func(t *testing.T, client *Client, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.logger)
				assert.NotNil(t, client.cache)
				assert.NotNil(t, client.retryer)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.ctx, tt.options...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.validate != nil {
				tt.validate(t, client, err)
			}
		})
	}
}

func TestNewClientWithNilContext(t *testing.T) {
	// Test that NewClient properly handles nil context
	client, err := NewClient(nil) //nolint:staticcheck // intentionally testing nil context error handling

	// Should return an error for nil context
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "context")
}

func TestClientImplementsInterface(t *testing.T) {
	ctx := context.Background()
	client, err := NewClient(ctx)

	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify the client has the expected AWS API client
	assert.NotNil(t, client.api)
	assert.IsType(t, &secretsmanager.Client{}, client.api)
}

func TestClientWithContextTimeout(t *testing.T) {
	// Test with a context that has a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	client, err := NewClient(ctx)

	// Should succeed within the timeout
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestClient_handleError(t *testing.T) {
	client := &Client{} // No logger, cache, or retryer needed for error handling tests

	tests := []struct {
		name         string
		inputErr     error
		operation    string
		expectCustom bool
		expectMsg    string
	}{
		{
			name:         "nil error returns nil",
			inputErr:     nil,
			operation:    "GetSecret",
			expectCustom: false,
		},
		{
			name:         "standard error wrapped with operation context",
			inputErr:     fmt.Errorf("network timeout"),
			operation:    "GetSecret",
			expectCustom: false,
			expectMsg:    "GetSecret operation failed: network timeout",
		},
		{
			name:         "ErrSecretNotFound preserved",
			inputErr:     ErrSecretNotFound,
			operation:    "GetSecret",
			expectCustom: true,
		},
		{
			name:         "ErrSecretEmpty preserved",
			inputErr:     ErrSecretEmpty,
			operation:    "GetSecret",
			expectCustom: true,
		},
		{
			name:         "ErrAccessDenied preserved",
			inputErr:     ErrAccessDenied,
			operation:    "GetSecret",
			expectCustom: true,
		},
		{
			name:         "smithy API error wrapped",
			inputErr:     &smithy.GenericAPIError{Code: "InvalidRequest", Message: "invalid parameter"},
			operation:    "PutSecret",
			expectCustom: false,
			expectMsg:    "PutSecret operation failed: InvalidRequest: invalid parameter",
		},
		{
			name:         "wrapped custom error preserved",
			inputErr:     fmt.Errorf("some context: %w", ErrSecretNotFound),
			operation:    "DescribeSecret",
			expectCustom: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.handleError(tt.inputErr, tt.operation)

			if tt.inputErr == nil {
				assert.NoError(t, result)
				return
			}

			assert.Error(t, result)

			if tt.expectCustom {
				// For custom errors, check if they're preserved
				assert.True(t, errors.Is(result, tt.inputErr) || errors.Is(result, errors.Unwrap(tt.inputErr)))
			} else if tt.expectMsg != "" {
				assert.Contains(t, result.Error(), tt.expectMsg)
			}
		})
	}
}

func TestClient_handleErrorWithLogger(t *testing.T) {
	// Test that error handling works with a logger configured
	logger := slog.New(slog.NewTextHandler(nil, nil))
	client := &Client{logger: logger}

	err := fmt.Errorf("test error")
	result := client.handleError(err, "TestOperation")

	assert.Error(t, result)
	assert.Contains(t, result.Error(), "TestOperation operation failed")
}

func TestClient_GetSecret(t *testing.T) {
	tests := []struct {
		name           string
		secretName     string
		setupMock      func(*mockManagerAPI)
		expectError    bool
		expectedError  error
		expectedResult string
		validateLogs   func(*testing.T, []logEntry)
	}{
		{
			name:       "successful secret retrieval with string value",
			secretName: "test-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					assert.Equal(t, "test-secret", *params.SecretId)
					return &secretsmanager.GetSecretValueOutput{
						SecretString: stringPtr("my-secret-value"),
						Name:         stringPtr("test-secret"),
					}, nil
				}
			},
			expectError:    false,
			expectedResult: "my-secret-value",
			validateLogs: func(t *testing.T, logs []logEntry) {
				assert.Contains(t, logs, logEntry{level: "INFO", msg: "retrieving secret", secretName: "test-secret"})
				// Ensure no secret values are logged
				for _, log := range logs {
					assert.NotContains(t, log.msg, "my-secret-value")
				}
			},
		},
		{
			name:       "successful secret retrieval with binary value",
			secretName: "binary-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return &secretsmanager.GetSecretValueOutput{
						SecretBinary: []byte("binary-secret-data"),
						Name:         stringPtr("binary-secret"),
					}, nil
				}
			},
			expectError:    false,
			expectedResult: "binary-secret-data",
		},
		{
			name:       "secret not found error",
			secretName: "nonexistent-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return nil, &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "Secret not found"}
				}
			},
			expectError:   true,
			expectedError: ErrSecretNotFound,
		},
		{
			name:       "access denied error",
			secretName: "restricted-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "Access denied"}
				}
			},
			expectError:   true,
			expectedError: ErrAccessDenied,
		},
		{
			name:       "empty secret response",
			secretName: "empty-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return &secretsmanager.GetSecretValueOutput{
						Name: stringPtr("empty-secret"),
					}, nil
				}
			},
			expectError:   true,
			expectedError: ErrSecretEmpty,
		},
		{
			name:       "network error",
			secretName: "network-error-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return nil, fmt.Errorf("network timeout")
				}
			},
			expectError: true,
			validateLogs: func(t *testing.T, logs []logEntry) {
				assert.Contains(
					t,
					logs,
					logEntry{level: "ERROR", msg: "failed to retrieve secret", secretName: "network-error-secret"},
				)
			},
		},
		{
			name:       "context cancellation",
			secretName: "cancelled-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					// Simulate context cancellation by returning a context.Canceled error
					return nil, context.Canceled
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup logger to capture logs
			var logs []logEntry
			logger := slog.New(&testLogHandler{logs: &logs})

			// Setup mock API
			mockAPI := &mockManagerAPI{}
			if tt.setupMock != nil {
				tt.setupMock(mockAPI)
			}

			// Create client
			client := &Client{
				api:    mockAPI,
				logger: logger,
			}

			// Call GetSecret
			ctx := context.Background()
			result, err := client.GetSecret(ctx, tt.secretName)

			// Validate results
			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.ErrorIs(t, err, tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			// Validate logs if specified
			if tt.validateLogs != nil {
				tt.validateLogs(t, logs)
			}
		})
	}
}

func TestClient_GetSecretWithNilContext(t *testing.T) {
	client := &Client{}
	//nolint:staticcheck // intentionally testing nil context error handling
	result, err := client.GetSecret(nil, "test-secret")

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "context")
}

func TestClient_GetSecretWithEmptySecretName(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	result, err := client.GetSecret(ctx, "")

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "secret name")
}

func TestClient_PutSecret(t *testing.T) {
	tests := []struct {
		name          string
		secretName    string
		secretValue   string
		setupMock     func(*mockManagerAPI)
		expectError   bool
		expectedError error
		validateLogs  func(*testing.T, []logEntry)
	}{
		{
			name:        "successful secret update with string value",
			secretName:  "test-secret",
			secretValue: "new-secret-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.putSecretValueFunc = func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
					assert.Equal(t, "test-secret", *params.SecretId)
					assert.Equal(t, "new-secret-value", *params.SecretString)
					return &secretsmanager.PutSecretValueOutput{
						Name: stringPtr("test-secret"),
					}, nil
				}
			},
			expectError: false,
			validateLogs: func(t *testing.T, logs []logEntry) {
				assert.Contains(t, logs, logEntry{level: "INFO", msg: "updating secret", secretName: "test-secret"})
				// Ensure no secret values are logged
				for _, log := range logs {
					assert.NotContains(t, log.msg, "new-secret-value")
				}
			},
		},
		{
			name:        "successful secret update with different value",
			secretName:  "existing-secret",
			secretValue: "updated-secret-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.putSecretValueFunc = func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
					assert.Equal(t, "existing-secret", *params.SecretId)
					assert.Equal(t, "updated-secret-value", *params.SecretString)
					return &secretsmanager.PutSecretValueOutput{
						Name: stringPtr("existing-secret"),
					}, nil
				}
			},
			expectError: false,
		},
		{
			name:        "secret not found error",
			secretName:  "nonexistent-secret",
			secretValue: "some-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.putSecretValueFunc = func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
					return nil, &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "Secret not found"}
				}
			},
			expectError:   true,
			expectedError: ErrSecretNotFound,
		},
		{
			name:        "access denied error",
			secretName:  "restricted-secret",
			secretValue: "some-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.putSecretValueFunc = func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
					return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "Access denied"}
				}
			},
			expectError:   true,
			expectedError: ErrAccessDenied,
		},
		{
			name:        "network error",
			secretName:  "network-error-secret",
			secretValue: "some-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.putSecretValueFunc = func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
					return nil, fmt.Errorf("network timeout")
				}
			},
			expectError: true,
			validateLogs: func(t *testing.T, logs []logEntry) {
				assert.Contains(
					t,
					logs,
					logEntry{level: "ERROR", msg: "failed to update secret", secretName: "network-error-secret"},
				)
			},
		},
		{
			name:        "context cancellation",
			secretName:  "cancelled-secret",
			secretValue: "some-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.putSecretValueFunc = func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
					return nil, context.Canceled
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup logger to capture logs
			var logs []logEntry
			logger := slog.New(&testLogHandler{logs: &logs})

			// Setup mock API
			mockAPI := &mockManagerAPI{}
			if tt.setupMock != nil {
				tt.setupMock(mockAPI)
			}

			// Create client
			client := &Client{
				api:    mockAPI,
				logger: logger,
			}

			// Call PutSecret
			ctx := context.Background()
			err := client.PutSecret(ctx, tt.secretName, tt.secretValue)

			// Validate results
			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.ErrorIs(t, err, tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			// Validate logs if specified
			if tt.validateLogs != nil {
				tt.validateLogs(t, logs)
			}
		})
	}
}

func TestClient_PutSecretWithNilContext(t *testing.T) {
	client := &Client{}
	//nolint:staticcheck // intentionally testing nil context error handling
	err := client.PutSecret(nil, "test-secret", "test-value")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestClient_PutSecretWithEmptySecretName(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	err := client.PutSecret(ctx, "", "test-value")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret name")
}

func TestClient_PutSecretWithEmptyValue(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	err := client.PutSecret(ctx, "test-secret", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret value")
}

func TestClient_CreateSecret(t *testing.T) {
	tests := []struct {
		name           string
		secretName     string
		secretValue    string
		kmsKeyID       string
		setupMock      func(*mockManagerAPI)
		expectError    bool
		expectedError  error
		validateResult func(*testing.T, error)
		validateLogs   func(*testing.T, []logEntry)
	}{
		{
			name:        "successful secret creation with string value",
			secretName:  "new-secret",
			secretValue: "secret-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.createSecretFunc = func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					assert.Equal(t, "new-secret", *params.Name)
					assert.Equal(t, "secret-value", *params.SecretString)
					assert.Nil(t, params.KmsKeyId)
					return &secretsmanager.CreateSecretOutput{
						Name: stringPtr("new-secret"),
					}, nil
				}
			},
			expectError: false,
			validateLogs: func(t *testing.T, logs []logEntry) {
				assert.Contains(t, logs, logEntry{level: "INFO", msg: "creating secret", secretName: "new-secret"})
				// Ensure no secret values are logged
				for _, log := range logs {
					assert.NotContains(t, log.msg, "secret-value")
				}
			},
		},
		{
			name:        "successful secret creation with KMS key",
			secretName:  "encrypted-secret",
			secretValue: "encrypted-value",
			kmsKeyID:    "alias/aws/secretsmanager",
			setupMock: func(mock *mockManagerAPI) {
				mock.createSecretFunc = func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					assert.Equal(t, "encrypted-secret", *params.Name)
					assert.Equal(t, "encrypted-value", *params.SecretString)
					assert.Equal(t, "alias/aws/secretsmanager", *params.KmsKeyId)
					return &secretsmanager.CreateSecretOutput{
						Name: stringPtr("encrypted-secret"),
					}, nil
				}
			},
			expectError: false,
		},
		{
			name:        "secret already exists error",
			secretName:  "existing-secret",
			secretValue: "some-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.createSecretFunc = func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					return nil, &smithy.GenericAPIError{
						Code:    "ResourceExistsException",
						Message: "Secret already exists",
					}
				}
			},
			expectError: true,
			validateResult: func(t *testing.T, result error) {
				assert.Contains(
					t,
					result.Error(),
					"CreateSecret operation failed: ResourceExistsException: Secret already exists",
				)
			},
		},
		{
			name:        "access denied error",
			secretName:  "restricted-secret",
			secretValue: "some-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.createSecretFunc = func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "Access denied"}
				}
			},
			expectError:   true,
			expectedError: ErrAccessDenied,
		},
		{
			name:        "network error",
			secretName:  "network-error-secret",
			secretValue: "some-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.createSecretFunc = func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					return nil, fmt.Errorf("network timeout")
				}
			},
			expectError: true,
			validateLogs: func(t *testing.T, logs []logEntry) {
				assert.Contains(
					t,
					logs,
					logEntry{level: "ERROR", msg: "failed to create secret", secretName: "network-error-secret"},
				)
			},
		},
		{
			name:        "context cancellation",
			secretName:  "cancelled-secret",
			secretValue: "some-value",
			setupMock: func(mock *mockManagerAPI) {
				mock.createSecretFunc = func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					return nil, context.Canceled
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup logger to capture logs
			var logs []logEntry
			logger := slog.New(&testLogHandler{logs: &logs})

			// Setup mock API
			mockAPI := &mockManagerAPI{}
			if tt.setupMock != nil {
				tt.setupMock(mockAPI)
			}

			// Create client
			client := &Client{
				api:    mockAPI,
				logger: logger,
			}

			// Call CreateSecret
			ctx := context.Background()
			err := client.CreateSecret(ctx, tt.secretName, tt.secretValue, tt.kmsKeyID)

			// Validate results
			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.ErrorIs(t, err, tt.expectedError)
				}
				if tt.validateResult != nil {
					tt.validateResult(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			// Validate logs if specified
			if tt.validateLogs != nil {
				tt.validateLogs(t, logs)
			}
		})
	}
}

func TestClient_CreateSecretWithNilContext(t *testing.T) {
	client := &Client{}
	//nolint:staticcheck // intentionally testing nil context error handling
	err := client.CreateSecret(nil, "test-secret", "test-value", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestClient_CreateSecretWithEmptySecretName(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	err := client.CreateSecret(ctx, "", "test-value", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret name")
}

func TestClient_CreateSecretWithEmptyValue(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	err := client.CreateSecret(ctx, "test-secret", "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret value")
}

// DescribeSecretResult represents the metadata returned by DescribeSecret
type DescribeSecretResult struct {
	Name            string
	Description     string
	CreatedDate     time.Time
	LastChangedDate time.Time
	Tags            map[string]string
}

// mockDescribeSecretResult creates a mock DescribeSecret result for testing
func mockDescribeSecretResult(name string) *secretsmanager.DescribeSecretOutput {
	return &secretsmanager.DescribeSecretOutput{
		Name:            &name,
		Description:     stringPtr("Test secret for unit tests"),
		CreatedDate:     &time.Time{}, // Zero time for simplicity
		LastChangedDate: &time.Time{}, // Zero time for simplicity
	}
}

func TestClient_DescribeSecret(t *testing.T) {
	tests := []struct {
		name           string
		secretName     string
		setupMock      func(*mockManagerAPI)
		expectError    bool
		expectedError  error
		validateResult func(*testing.T, *secretsmanager.DescribeSecretOutput)
		validateLogs   func(*testing.T, []logEntry)
	}{
		{
			name:       "successful secret description",
			secretName: "test-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					assert.Equal(t, "test-secret", *params.SecretId)
					return mockDescribeSecretResult("test-secret"), nil
				}
			},
			expectError: false,
			validateResult: func(t *testing.T, result *secretsmanager.DescribeSecretOutput) {
				assert.Equal(t, "test-secret", *result.Name)
				assert.Equal(t, "Test secret for unit tests", *result.Description)
			},
			validateLogs: func(t *testing.T, logs []logEntry) {
				assert.Contains(t, logs, logEntry{level: "INFO", msg: "describing secret", secretName: "test-secret"})
			},
		},
		{
			name:       "secret not found error",
			secretName: "nonexistent-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "Secret not found"}
				}
			},
			expectError:   true,
			expectedError: ErrSecretNotFound,
		},
		{
			name:       "access denied error",
			secretName: "restricted-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "Access denied"}
				}
			},
			expectError:   true,
			expectedError: ErrAccessDenied,
		},
		{
			name:       "network error",
			secretName: "network-error-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, fmt.Errorf("network timeout")
				}
			},
			expectError: true,
			validateLogs: func(t *testing.T, logs []logEntry) {
				assert.Contains(
					t,
					logs,
					logEntry{level: "ERROR", msg: "failed to describe secret", secretName: "network-error-secret"},
				)
			},
		},
		{
			name:       "context cancellation",
			secretName: "cancelled-secret",
			setupMock: func(mock *mockManagerAPI) {
				mock.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, context.Canceled
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup logger to capture logs
			var logs []logEntry
			logger := slog.New(&testLogHandler{logs: &logs})

			// Setup mock API
			mockAPI := &mockManagerAPI{}
			if tt.setupMock != nil {
				tt.setupMock(mockAPI)
			}

			// Create client
			client := &Client{
				api:    mockAPI,
				logger: logger,
			}

			// Call DescribeSecret
			ctx := context.Background()
			result, err := client.DescribeSecret(ctx, tt.secretName)

			// Validate results
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.expectedError != nil {
					assert.ErrorIs(t, err, tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validateResult != nil {
					tt.validateResult(t, result)
				}
			}

			// Validate logs if specified
			if tt.validateLogs != nil {
				tt.validateLogs(t, logs)
			}
		})
	}
}

func TestClient_DescribeSecretWithNilContext(t *testing.T) {
	client := &Client{}
	//nolint:staticcheck // intentionally testing nil context error handling
	result, err := client.DescribeSecret(nil, "test-secret")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context")
}

func TestClient_DescribeSecretWithEmptySecretName(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	result, err := client.DescribeSecret(ctx, "")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "secret name")
}

// Test Cache Integration Tests

func TestNewClientWithCache(t *testing.T) {
	t.Run("successful cache client creation", func(t *testing.T) {
		ctx := context.Background()
		cacheTTL := 5 * time.Minute
		cacheSize := 100

		client, err := NewClientWithCache(ctx, cacheTTL, cacheSize)

		require.NoError(t, err)
		require.NotNil(t, client)
		assert.NotNil(t, client.cache)

		// Verify cache is properly configured
		if inMemoryCache, ok := client.cache.(*InMemoryCache); ok {
			assert.Equal(t, cacheSize, inMemoryCache.maxSize)
			assert.Equal(t, cacheTTL, inMemoryCache.defaultTTL)
		}
	})

	t.Run("cache client with options", func(t *testing.T) {
		ctx := context.Background()
		logger := slog.New(slog.NewTextHandler(nil, nil))

		client, err := NewClientWithCache(ctx, 5*time.Minute, 50,
			WithLogger(logger),
		)

		require.NoError(t, err)
		require.NotNil(t, client)
		assert.NotNil(t, client.cache)
		assert.Equal(t, logger, client.logger)
	})

	t.Run("cache client with zero TTL", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewClientWithCache(ctx, 0, 100)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TTL must be positive")
	})

	t.Run("cache client with negative TTL", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewClientWithCache(ctx, -time.Minute, 100)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TTL must be positive")
	})

	t.Run("cache client with zero TTL", func(t *testing.T) {
		ctx := context.Background()
		_, err := NewClientWithCache(ctx, 0, 100)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TTL must be positive")
	})
}

func TestClient_GetSecretCached(t *testing.T) {
	setupTestClient := func() (*Client, *mockManagerAPI) {
		mockAPI := &mockManagerAPI{}
		client := &Client{
			api:    mockAPI,
			cache:  NewInMemoryCache(5*time.Minute, 10),
			logger: nil,
		}
		return client, mockAPI
	}

	t.Run("cache hit returns cached value", func(t *testing.T) {
		client, mockAPI := setupTestClient()
		ctx := context.Background()
		secretName := "test-secret"
		cachedValue := "cached-secret-value"

		// Pre-populate cache
		client.cache.Set(secretName, cachedValue, time.Minute)

		result, err := client.GetSecretCached(ctx, secretName)

		assert.NoError(t, err)
		assert.Equal(t, cachedValue, result)

		// Verify AWS API was not called
		assert.Nil(t, mockAPI.getSecretValueFunc)
	})

	t.Run("cache miss fetches from AWS and caches", func(t *testing.T) {
		client, mockAPI := setupTestClient()
		ctx := context.Background()
		secretName := "test-secret"
		awsValue := "aws-secret-value"

		// Mock AWS API response
		mockAPI.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			assert.Equal(t, secretName, *params.SecretId)
			return &secretsmanager.GetSecretValueOutput{
				SecretString: &awsValue,
			}, nil
		}

		result, err := client.GetSecretCached(ctx, secretName)

		assert.NoError(t, err)
		assert.Equal(t, awsValue, result)

		// Verify value was cached
		cachedValue, found := client.cache.Get(secretName)
		assert.True(t, found)
		assert.Equal(t, awsValue, cachedValue)
	})

	t.Run("cache miss with binary secret", func(t *testing.T) {
		client, mockAPI := setupTestClient()
		ctx := context.Background()
		secretName := "binary-secret"
		binaryData := []byte("binary-secret-value")

		// Mock AWS API response
		mockAPI.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			assert.Equal(t, secretName, *params.SecretId)
			return &secretsmanager.GetSecretValueOutput{
				SecretBinary: binaryData,
			}, nil
		}

		result, err := client.GetSecretCached(ctx, secretName)

		assert.NoError(t, err)
		assert.Equal(t, string(binaryData), result)

		// Verify value was cached
		cachedValue, found := client.cache.Get(secretName)
		assert.True(t, found)
		assert.Equal(t, string(binaryData), cachedValue)
	})

	t.Run("cache miss with AWS error", func(t *testing.T) {
		client, mockAPI := setupTestClient()
		ctx := context.Background()
		secretName := "error-secret"

		// Mock AWS API error
		mockAPI.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return nil, &smithy.OperationError{
				ServiceID:     "SecretsManager",
				OperationName: "GetSecretValue",
				Err:           fmt.Errorf("test error"),
			}
		}

		result, err := client.GetSecretCached(ctx, secretName)

		assert.Error(t, err)
		assert.Empty(t, result)

		// Verify nothing was cached
		cachedValue, found := client.cache.Get(secretName)
		assert.False(t, found)
		assert.Nil(t, cachedValue)
	})

	t.Run("no cache configured falls back to GetSecret", func(t *testing.T) {
		mockAPI := &mockManagerAPI{}
		client := &Client{
			api:    mockAPI,
			cache:  nil, // No cache configured
			logger: nil,
		}
		ctx := context.Background()
		secretName := "test-secret"
		awsValue := "aws-secret-value"

		// Mock AWS API response
		mockAPI.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return &secretsmanager.GetSecretValueOutput{
				SecretString: &awsValue,
			}, nil
		}

		result, err := client.GetSecretCached(ctx, secretName)

		assert.NoError(t, err)
		assert.Equal(t, awsValue, result)
	})

	t.Run("successful cache operation", func(t *testing.T) {
		client, mockAPI := setupTestClient()
		ctx := context.Background()
		secretName := "test-secret"
		secretValue := "test-value"

		// Mock AWS API response
		mockAPI.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			return &secretsmanager.GetSecretValueOutput{
				SecretString: &secretValue,
			}, nil
		}

		result, err := client.GetSecretCached(ctx, secretName)

		// This should succeed since we're passing a valid context and proper mock
		assert.NoError(t, err)
		assert.Equal(t, secretValue, result)
	})

	t.Run("empty secret name error", func(t *testing.T) {
		client, _ := setupTestClient()
		ctx := context.Background()

		result, err := client.GetSecretCached(ctx, "")

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "secret name cannot be empty")
	})
}

func TestClient_InvalidateCache(t *testing.T) {
	setupTestClient := func() (*Client, *mockManagerAPI) {
		mockAPI := &mockManagerAPI{}
		client := &Client{
			api:    mockAPI,
			cache:  NewInMemoryCache(5*time.Minute, 10),
			logger: nil,
		}
		return client, mockAPI
	}

	t.Run("invalidate existing cache entry", func(t *testing.T) {
		client, _ := setupTestClient()
		secretName := "test-secret"
		secretValue := "test-value"

		// Add to cache
		client.cache.Set(secretName, secretValue, time.Minute)
		assert.Equal(t, 1, client.GetCacheSize())

		// Invalidate cache
		client.InvalidateCache(secretName)

		// Verify entry was removed
		cachedValue, found := client.cache.Get(secretName)
		assert.False(t, found)
		assert.Nil(t, cachedValue)
		assert.Equal(t, 0, client.GetCacheSize())
	})

	t.Run("invalidate non-existent cache entry", func(t *testing.T) {
		client, _ := setupTestClient()

		// Should not panic
		client.InvalidateCache("non-existent")
		assert.Equal(t, 0, client.GetCacheSize())
	})

	t.Run("invalidate with empty secret name", func(t *testing.T) {
		client, _ := setupTestClient()

		// Should not panic
		client.InvalidateCache("")
	})

	t.Run("invalidate with no cache configured", func(t *testing.T) {
		mockAPI := &mockManagerAPI{}
		client := &Client{
			api:    mockAPI,
			cache:  nil, // No cache configured
			logger: nil,
		}

		// Should not panic
		client.InvalidateCache("test-secret")
	})
}

func TestClient_ClearCache(t *testing.T) {
	setupTestClient := func() (*Client, *mockManagerAPI) {
		mockAPI := &mockManagerAPI{}
		client := &Client{
			api:    mockAPI,
			cache:  NewInMemoryCache(5*time.Minute, 10),
			logger: nil,
		}
		return client, mockAPI
	}

	t.Run("clear cache with multiple entries", func(t *testing.T) {
		client, _ := setupTestClient()

		// Add multiple entries
		client.cache.Set("secret1", "value1", time.Minute)
		client.cache.Set("secret2", "value2", time.Minute)
		client.cache.Set("secret3", "value3", time.Minute)

		assert.Equal(t, 3, client.GetCacheSize())

		// Clear cache
		client.ClearCache()

		// Verify all entries are gone
		assert.Equal(t, 0, client.GetCacheSize())

		cachedValue, found := client.cache.Get("secret1")
		assert.False(t, found)
		assert.Nil(t, cachedValue)
	})

	t.Run("clear empty cache", func(t *testing.T) {
		client, _ := setupTestClient()

		// Should not panic
		client.ClearCache()
		assert.Equal(t, 0, client.GetCacheSize())
	})

	t.Run("clear cache with no cache configured", func(t *testing.T) {
		mockAPI := &mockManagerAPI{}
		client := &Client{
			api:    mockAPI,
			cache:  nil, // No cache configured
			logger: nil,
		}

		// Should not panic
		client.ClearCache()
	})
}

func TestClient_GetCacheSize(t *testing.T) {
	t.Run("cache size with entries", func(t *testing.T) {
		mockAPI := &mockManagerAPI{}
		client := &Client{
			api:    mockAPI,
			cache:  NewInMemoryCache(5*time.Minute, 10),
			logger: nil,
		}

		// Add entries
		client.cache.Set("secret1", "value1", time.Minute)
		client.cache.Set("secret2", "value2", time.Minute)

		assert.Equal(t, 2, client.GetCacheSize())
	})

	t.Run("cache size with no cache configured", func(t *testing.T) {
		mockAPI := &mockManagerAPI{}
		client := &Client{
			api:    mockAPI,
			cache:  nil, // No cache configured
			logger: nil,
		}

		assert.Equal(t, 0, client.GetCacheSize())
	})

	t.Run("cache size with expired entries", func(t *testing.T) {
		mockAPI := &mockManagerAPI{}
		client := &Client{
			api:    mockAPI,
			cache:  NewInMemoryCache(5*time.Minute, 10),
			logger: nil,
		}

		// Add entries with short TTL
		client.cache.Set("secret1", "value1", 10*time.Millisecond)
		client.cache.Set("secret2", "value2", time.Minute)

		// Wait for first entry to expire
		time.Sleep(15 * time.Millisecond)

		// Size should only count non-expired entries
		assert.Equal(t, 1, client.GetCacheSize())
	})
}

func TestClient_CacheIntegration(t *testing.T) {
	t.Run("end-to-end cache integration test", func(t *testing.T) {
		mockAPI := &mockManagerAPI{}
		client := &Client{
			api:    mockAPI,
			cache:  NewInMemoryCache(5*time.Minute, 10),
			logger: nil,
		}
		ctx := context.Background()

		callCount := 0
		secretName := "integration-test-secret"
		secretValue := "integration-test-value"

		// Mock AWS API that counts calls
		mockAPI.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			callCount++
			return &secretsmanager.GetSecretValueOutput{
				SecretString: &secretValue,
			}, nil
		}

		// First call should hit AWS
		result1, err1 := client.GetSecretCached(ctx, secretName)
		assert.NoError(t, err1)
		assert.Equal(t, secretValue, result1)
		assert.Equal(t, 1, callCount)
		assert.Equal(t, 1, client.GetCacheSize())

		// Second call should hit cache
		result2, err2 := client.GetSecretCached(ctx, secretName)
		assert.NoError(t, err2)
		assert.Equal(t, secretValue, result2)
		assert.Equal(t, 1, callCount) // Should not have increased
		assert.Equal(t, 1, client.GetCacheSize())

		// Invalidate cache
		client.InvalidateCache(secretName)
		assert.Equal(t, 0, client.GetCacheSize())

		// Third call should hit AWS again
		result3, err3 := client.GetSecretCached(ctx, secretName)
		assert.NoError(t, err3)
		assert.Equal(t, secretValue, result3)
		assert.Equal(t, 2, callCount) // Should have increased
		assert.Equal(t, 1, client.GetCacheSize())
	})
}

// Thread Safety and Concurrency Tests

func TestClient_ConcurrentSecretOperations(t *testing.T) {
	ctx := context.Background()
	var callCount int32

	mockAPI := &mockManagerAPI{
		getSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			atomic.AddInt32(&callCount, 1)

			secretName := *params.SecretId
			return &secretsmanager.GetSecretValueOutput{
				SecretString: &secretName,
			}, nil
		},
		putSecretValueFunc: func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
			atomic.AddInt32(&callCount, 1)
			return &secretsmanager.PutSecretValueOutput{}, nil
		},
	}

	client := &Client{
		api:    mockAPI,
		logger: nil,
		cache:  NewInMemoryCache(5*time.Minute, 100),
	}

	t.Run("concurrent reads from same secret", func(t *testing.T) {
		const numGoroutines = 50
		const numIterations = 10
		secretName := "concurrent-test-secret"

		var wg sync.WaitGroup
		results := make([][]string, numGoroutines)
		errors := make([][]error, numGoroutines)

		// Start concurrent goroutines
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				results[goroutineID] = make([]string, numIterations)
				errors[goroutineID] = make([]error, numIterations)

				for j := 0; j < numIterations; j++ {
					value, err := client.GetSecretCached(ctx, secretName)
					results[goroutineID][j] = value
					errors[goroutineID][j] = err
				}
			}(i)
		}

		wg.Wait()

		// Verify all results are correct
		for i := 0; i < numGoroutines; i++ {
			for j := 0; j < numIterations; j++ {
				assert.NoError(t, errors[i][j])
				assert.Equal(t, secretName, results[i][j])
			}
		}

		// Should have made at most a few API calls due to caching (race conditions may cause multiple calls)
		finalCallCount := int(atomic.LoadInt32(&callCount))
		assert.LessOrEqual(t, finalCallCount, 5) // Allow some duplicate calls due to race conditions
		assert.GreaterOrEqual(t, finalCallCount, 1)
	})

	t.Run("concurrent reads from different secrets", func(t *testing.T) {
		const numGoroutines = 20
		atomic.StoreInt32(&callCount, 0) // Reset counter

		var wg sync.WaitGroup
		results := make([]string, numGoroutines)
		errors := make([]error, numGoroutines)

		// Start concurrent goroutines reading different secrets
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(secretID int) {
				defer wg.Done()
				secretName := fmt.Sprintf("concurrent-secret-%d", secretID)
				value, err := client.GetSecret(ctx, secretName)
				results[secretID] = value
				errors[secretID] = err
			}(i)
		}

		wg.Wait()

		// Verify all results are correct
		for i := 0; i < numGoroutines; i++ {
			assert.NoError(t, errors[i])
			expected := fmt.Sprintf("concurrent-secret-%d", i)
			assert.Equal(t, expected, results[i])
		}

		// Should have made exactly numGoroutines API calls (no caching between different secrets)
		finalCallCount := int(atomic.LoadInt32(&callCount))
		assert.Equal(t, numGoroutines, finalCallCount)
	})

	t.Run("concurrent writes to different secrets", func(t *testing.T) {
		const numGoroutines = 20
		atomic.StoreInt32(&callCount, 0) // Reset counter

		var wg sync.WaitGroup
		errors := make([]error, numGoroutines)

		// Start concurrent goroutines writing to different secrets
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(secretID int) {
				defer wg.Done()
				secretName := fmt.Sprintf("write-secret-%d", secretID)
				secretValue := fmt.Sprintf("value-%d", secretID)
				err := client.PutSecret(ctx, secretName, secretValue)
				errors[secretID] = err
			}(i)
		}

		wg.Wait()

		// Verify all writes succeeded
		for i := 0; i < numGoroutines; i++ {
			assert.NoError(t, errors[i])
		}

		// Should have made exactly numGoroutines API calls
		finalCallCount := int(atomic.LoadInt32(&callCount))
		assert.Equal(t, numGoroutines, finalCallCount)
	})

	t.Run("concurrent cached operations", func(t *testing.T) {
		atomic.StoreInt32(&callCount, 0) // Reset counter
		client.ClearCache()              // Clear any previous cache entries
		const numGoroutines = 30
		const numIterations = 5
		secretName := "cached-concurrent-secret"

		var wg sync.WaitGroup
		results := make([][]string, numGoroutines)
		errors := make([][]error, numGoroutines)

		// Start concurrent goroutines using cached operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				results[goroutineID] = make([]string, numIterations)
				errors[goroutineID] = make([]error, numIterations)

				for j := 0; j < numIterations; j++ {
					value, err := client.GetSecretCached(ctx, secretName)
					results[goroutineID][j] = value
					errors[goroutineID][j] = err
				}
			}(i)
		}

		wg.Wait()

		// Verify all results are correct
		for i := 0; i < numGoroutines; i++ {
			for j := 0; j < numIterations; j++ {
				assert.NoError(t, errors[i][j])
				assert.Equal(t, secretName, results[i][j])
			}
		}

		// Should have made at most a few API calls due to caching (race conditions may cause multiple calls)
		finalCallCount := int(atomic.LoadInt32(&callCount))
		assert.LessOrEqual(t, finalCallCount, 5) // Allow some duplicate calls due to race conditions
		assert.GreaterOrEqual(t, finalCallCount, 1)
		assert.Equal(t, 1, client.GetCacheSize())
	})
}

func TestClient_ConcurrentCacheOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("concurrent cache invalidation", func(t *testing.T) {
		client, _ := NewClientWithCache(ctx, 5*time.Minute, 100)

		const numSecrets = 10
		const numGoroutines = 20

		// Pre-populate cache
		for i := 0; i < numSecrets; i++ {
			secretName := fmt.Sprintf("secret-%d", i)
			client.cache.Set(secretName, fmt.Sprintf("value-%d", i), time.Minute)
		}
		assert.Equal(t, numSecrets, client.GetCacheSize())

		var wg sync.WaitGroup
		var mu sync.Mutex
		activeGoroutines := 0

		// Start concurrent goroutines that invalidate cache entries
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				mu.Lock()
				activeGoroutines++
				mu.Unlock()

				// Each goroutine invalidates a different secret
				secretName := fmt.Sprintf("secret-%d", goroutineID%numSecrets)
				client.InvalidateCache(secretName)
			}(i)
		}

		wg.Wait()

		// Cache should be empty after invalidation
		assert.Equal(t, 0, client.GetCacheSize())
		assert.Equal(t, numGoroutines, activeGoroutines)
	})

	t.Run("concurrent cache clear", func(t *testing.T) {
		client, _ := NewClientWithCache(ctx, 5*time.Minute, 100)

		const numSecrets = 20

		// Pre-populate cache
		for i := 0; i < numSecrets; i++ {
			secretName := fmt.Sprintf("clear-secret-%d", i)
			client.cache.Set(secretName, fmt.Sprintf("value-%d", i), time.Minute)
		}
		assert.Equal(t, numSecrets, client.GetCacheSize())

		var wg sync.WaitGroup

		// Start multiple goroutines that clear cache
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				client.ClearCache()
			}()
		}

		wg.Wait()

		// Cache should be empty after clearing
		assert.Equal(t, 0, client.GetCacheSize())
	})
}

func TestClient_ThreadSafetyWithErrors(t *testing.T) {
	ctx := context.Background()
	var callCount int32
	var mu sync.Mutex

	mockAPI := &mockManagerAPI{
		getSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
			atomic.AddInt32(&callCount, 1)

			// Simulate occasional errors
			mu.Lock()
			currentCount := atomic.LoadInt32(&callCount)
			mu.Unlock()

			if currentCount%5 == 0 { // Every 5th call fails
				return nil, fmt.Errorf("ThrottlingException: Rate exceeded")
			}

			secretName := *params.SecretId
			return &secretsmanager.GetSecretValueOutput{
				SecretString: &secretName,
			}, nil
		},
	}

	client := &Client{
		api:    mockAPI,
		logger: nil,
		cache:  NewInMemoryCache(5*time.Minute, 100),
	}

	t.Run("concurrent operations with errors", func(t *testing.T) {
		const numGoroutines = 25
		const numIterations = 8

		var wg sync.WaitGroup
		results := make([][]string, numGoroutines)
		errors := make([][]error, numGoroutines)

		// Start concurrent goroutines that may encounter errors
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				results[goroutineID] = make([]string, numIterations)
				errors[goroutineID] = make([]error, numIterations)

				for j := 0; j < numIterations; j++ {
					secretName := fmt.Sprintf("error-test-secret-%d-%d", goroutineID, j)
					value, err := client.GetSecret(ctx, secretName)
					results[goroutineID][j] = value
					errors[goroutineID][j] = err
				}
			}(i)
		}

		wg.Wait()

		// Count successful vs failed operations
		successCount := 0
		errorCount := 0

		for i := 0; i < numGoroutines; i++ {
			for j := 0; j < numIterations; j++ {
				if errors[i][j] == nil {
					successCount++
					expected := fmt.Sprintf("error-test-secret-%d-%d", i, j)
					assert.Equal(t, expected, results[i][j])
				} else {
					errorCount++
					assert.Contains(t, errors[i][j].Error(), "ThrottlingException")
				}
			}
		}

		// Should have some successes and some failures
		assert.Greater(t, successCount, 0)
		assert.Greater(t, errorCount, 0)
		assert.Equal(t, numGoroutines*numIterations, successCount+errorCount)

		// Verify call count is reasonable
		finalCallCount := int(atomic.LoadInt32(&callCount))
		assert.Greater(t, finalCallCount, 0)
		assert.LessOrEqual(t, finalCallCount, numGoroutines*numIterations)
	})
}
