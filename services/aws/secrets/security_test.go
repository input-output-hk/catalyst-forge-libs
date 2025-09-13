// Package secrets provides comprehensive security tests for AWS Secrets Manager operations.
package secrets

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogs captures log output for security validation
type logCapture struct {
	logs []string
}

func (c *logCapture) Write(p []byte) (n int, err error) {
	c.logs = append(c.logs, string(p))
	return len(p), nil
}

// TestSecurity_NoSecretValuesInLogs verifies that secret values are never logged
func TestSecurity_NoSecretValuesInLogs(t *testing.T) {
	tests := []struct {
		name        string
		secretValue string
		operation   func(ctx context.Context, client *Client) error
	}{
		{
			name:        "GetSecret does not log secret values",
			secretValue: "super-secret-password-12345",
			operation: func(ctx context.Context, client *Client) error {
				_, err := client.GetSecret(ctx, "test-secret")
				return err
			},
		},
		{
			name:        "PutSecret does not log secret values",
			secretValue: "new-super-secret-password-67890",
			operation: func(ctx context.Context, client *Client) error {
				return client.PutSecret(ctx, "test-secret", "new-super-secret-password-67890")
			},
		},
		{
			name:        "CreateSecret does not log secret values",
			secretValue: "database-connection-string-super-long",
			operation: func(ctx context.Context, client *Client) error {
				return client.CreateSecret(ctx, "test-secret", "database-connection-string-super-long", "")
			},
		},
		{
			name:        "GetSecretCached does not log secret values",
			secretValue: "cached-secret-api-key-token",
			operation: func(ctx context.Context, client *Client) error {
				_, err := client.GetSecretCached(ctx, "test-secret")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a log capture
			logCapture := &logCapture{}
			logger := slog.New(slog.NewTextHandler(logCapture, &slog.HandlerOptions{}))

			// Create mock API
			mockAPI := &mockManagerAPI{
				getSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return &secretsmanager.GetSecretValueOutput{
						SecretString: &tt.secretValue,
					}, nil
				},
				putSecretValueFunc: func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
					return &secretsmanager.PutSecretValueOutput{}, nil
				},
				createSecretFunc: func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					return &secretsmanager.CreateSecretOutput{}, nil
				},
			}

			// Create client with logger
			client := &Client{
				api:    mockAPI,
				logger: logger,
			}

			// Execute operation
			ctx := context.Background()
			err := tt.operation(ctx, client)

			// Assert no error occurred (we're only testing logging)
			assert.NoError(t, err)

			// Verify secret value is not in any log output
			for _, log := range logCapture.logs {
				assert.NotContains(t, log, tt.secretValue,
					"Secret value '%s' found in log output: %s", tt.secretValue, log)
			}
		})
	}
}

// TestSecurity_ErrorMessagesDontLeakSecrets verifies that error messages don't leak sensitive information
func TestSecurity_ErrorMessagesDontLeakSecrets(t *testing.T) {
	tests := []struct {
		name         string
		secretName   string
		secretValue  string
		errorMessage string
	}{
		{
			name:         "AWS API error doesn't leak secret name in wrapped error",
			secretName:   "prod/database/super-secret-connection-string",
			secretValue:  "postgresql://user:password@host:5432/db",
			errorMessage: "ResourceNotFoundException: Secrets Manager can't find the specified secret.",
		},
		{
			name:         "Access denied error doesn't leak secret details",
			secretName:   "prod/api-keys/stripe-secret-key",
			secretValue:  "secretValue",
			errorMessage: "AccessDeniedException: User is not authorized to perform this action.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock API that returns an error
			mockAPI := &mockManagerAPI{
				getSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return nil, &smithyAPIError{
						code:    "ResourceNotFoundException",
						message: tt.errorMessage,
					}
				},
			}

			// Create client
			client := &Client{
				api:    mockAPI,
				logger: slog.Default(),
			}

			// Execute operation
			ctx := context.Background()
			_, err := client.GetSecret(ctx, tt.secretName)

			// Assert error occurred
			require.Error(t, err)

			// Verify error message doesn't contain secret name or value
			errMsg := err.Error()
			assert.NotContains(t, errMsg, tt.secretName,
				"Secret name found in error message: %s", errMsg)
			assert.NotContains(t, errMsg, tt.secretValue,
				"Secret value found in error message: %s", errMsg)
		})
	}
}

// TestSecurity_InputValidation verifies that input validation prevents injection attacks
func TestSecurity_InputValidation(t *testing.T) {
	tests := []struct {
		name        string
		secretName  string
		secretValue string
		shouldError bool
		errorCheck  func(t *testing.T, err error)
	}{
		{
			name:        "empty secret name is rejected",
			secretName:  "",
			secretValue: "some-value",
			shouldError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "secret name cannot be empty")
			},
		},
		{
			name:        "empty secret value is rejected for PutSecret",
			secretName:  "test-secret",
			secretValue: "",
			shouldError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "secret value cannot be empty")
			},
		},
		{
			name:        "very long secret name is accepted",
			secretName:  strings.Repeat("a", 512), // AWS limit is 512 chars
			secretValue: "some-value",
			shouldError: false,
		},
		{
			name:        "special characters in secret name are accepted",
			secretName:  "test-secret_with.special:chars",
			secretValue: "some-value",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock API
			mockAPI := &mockManagerAPI{
				putSecretValueFunc: func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
					return &secretsmanager.PutSecretValueOutput{}, nil
				},
			}

			// Create client
			client := &Client{
				api:    mockAPI,
				logger: slog.Default(),
			}

			// Execute operation
			ctx := context.Background()
			err := client.PutSecret(ctx, tt.secretName, tt.secretValue)

			// Check error expectations
			if tt.shouldError {
				require.Error(t, err)
				tt.errorCheck(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSecurity_ContextValidation verifies that nil context is properly rejected
// Note: nil context usage is intentional here for testing purposes
//
//nolint:staticcheck // nil context is intentionally passed to test validation
func TestSecurity_ContextValidation(t *testing.T) {
	tests := []struct {
		name      string
		operation func(client *Client) error
	}{
		{
			name: "GetSecret rejects nil context",
			operation: func(client *Client) error {
				_, err := client.GetSecret(nil, "test-secret")
				return err
			},
		},
		{
			name: "PutSecret rejects nil context",
			operation: func(client *Client) error {
				return client.PutSecret(nil, "test-secret", "value")
			},
		},
		{
			name: "CreateSecret rejects nil context",
			operation: func(client *Client) error {
				return client.CreateSecret(nil, "test-secret", "value", "")
			},
		},
		{
			name: "DescribeSecret rejects nil context",
			operation: func(client *Client) error {
				_, err := client.DescribeSecret(nil, "test-secret")
				return err
			},
		},
		{
			name: "GetSecretCached rejects nil context",
			operation: func(client *Client) error {
				_, err := client.GetSecretCached(nil, "test-secret")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock API
			mockAPI := &mockManagerAPI{}

			// Create client
			client := &Client{
				api:    mockAPI,
				logger: slog.Default(),
			}

			// Execute operation with nil context
			err := tt.operation(client)

			// Assert error occurred
			require.Error(t, err)
			assert.Contains(t, err.Error(), "context cannot be nil")
		})
	}
}

// TestSecurity_LogLevels verifies appropriate log levels are used for different operations
func TestSecurity_LogLevels(t *testing.T) {
	tests := []struct {
		name           string
		operation      func(ctx context.Context, client *Client) error
		expectedLevels map[string]bool // level -> should appear
	}{
		{
			name: "successful GetSecret logs at INFO level",
			operation: func(ctx context.Context, client *Client) error {
				_, err := client.GetSecret(ctx, "test-secret")
				return err
			},
			expectedLevels: map[string]bool{
				"INFO":  true,
				"ERROR": false,
			},
		},
		{
			name: "failed GetSecret logs at ERROR level",
			operation: func(ctx context.Context, client *Client) error {
				_, err := client.GetSecret(ctx, "missing-secret")
				return err
			},
			expectedLevels: map[string]bool{
				"INFO":  true, // Initial INFO log before error occurs
				"ERROR": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a log capture
			logCapture := &logCapture{}
			logger := slog.New(slog.NewTextHandler(logCapture, &slog.HandlerOptions{}))

			// Create mock API
			mockAPI := &mockManagerAPI{
				getSecretValueFunc: func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					if *params.SecretId == "missing-secret" {
						return nil, &smithyAPIError{
							code:    "ResourceNotFoundException",
							message: "Secret not found",
						}
					}
					return &secretsmanager.GetSecretValueOutput{
						SecretString: stringPtr("test-value"),
					}, nil
				},
			}

			// Create client with logger
			client := &Client{
				api:    mockAPI,
				logger: logger,
			}

			// Execute operation
			ctx := context.Background()
			_ = tt.operation(ctx, client) // Ignore error, we're testing logs

			// Check log levels
			logOutput := strings.Join(logCapture.logs, "\n")
			for level, shouldAppear := range tt.expectedLevels {
				if shouldAppear {
					assert.Contains(t, logOutput, "level="+level,
						"Expected log level %s not found in output", level)
				} else {
					assert.NotContains(t, logOutput, "level="+level,
						"Unexpected log level %s found in output", level)
				}
			}
		})
	}
}

// smithyAPIError implements a mock smithy API error for testing
type smithyAPIError struct {
	code    string
	message string
}

func (e *smithyAPIError) Error() string {
	return e.message
}

func (e *smithyAPIError) ErrorCode() string {
	return e.code
}

func (e *smithyAPIError) ErrorMessage() string {
	return e.message
}
