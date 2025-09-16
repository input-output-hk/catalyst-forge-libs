package aws

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/input-output-hk/catalyst-forge-libs/secrets/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSecretsManagerClient implements SecretsManagerAPI for testing
type mockSecretsManagerClient struct {
	getSecretValueFunc func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	describeSecretFunc func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error)
	createSecretFunc   func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	putSecretValueFunc func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	deleteSecretFunc   func(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error)
	updateSecretFunc   func(ctx context.Context, params *secretsmanager.UpdateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.UpdateSecretOutput, error)
}

func (m *mockSecretsManagerClient) GetSecretValue(
	ctx context.Context,
	params *secretsmanager.GetSecretValueInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.GetSecretValueOutput, error) {
	if m.getSecretValueFunc != nil {
		return m.getSecretValueFunc(ctx, params, optFns...)
	}
	return nil, errors.New("GetSecretValue not implemented")
}

func (m *mockSecretsManagerClient) DescribeSecret(
	ctx context.Context,
	params *secretsmanager.DescribeSecretInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.DescribeSecretOutput, error) {
	if m.describeSecretFunc != nil {
		return m.describeSecretFunc(ctx, params, optFns...)
	}
	return nil, errors.New("DescribeSecret not implemented")
}

func (m *mockSecretsManagerClient) CreateSecret(
	ctx context.Context,
	params *secretsmanager.CreateSecretInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.CreateSecretOutput, error) {
	if m.createSecretFunc != nil {
		return m.createSecretFunc(ctx, params, optFns...)
	}
	return nil, errors.New("CreateSecret not implemented")
}

func (m *mockSecretsManagerClient) PutSecretValue(
	ctx context.Context,
	params *secretsmanager.PutSecretValueInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.PutSecretValueOutput, error) {
	if m.putSecretValueFunc != nil {
		return m.putSecretValueFunc(ctx, params, optFns...)
	}
	return nil, errors.New("PutSecretValue not implemented")
}

func (m *mockSecretsManagerClient) DeleteSecret(
	ctx context.Context,
	params *secretsmanager.DeleteSecretInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.DeleteSecretOutput, error) {
	if m.deleteSecretFunc != nil {
		return m.deleteSecretFunc(ctx, params, optFns...)
	}
	return nil, errors.New("DeleteSecret not implemented")
}

func (m *mockSecretsManagerClient) UpdateSecret(
	ctx context.Context,
	params *secretsmanager.UpdateSecretInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.UpdateSecretOutput, error) {
	if m.updateSecretFunc != nil {
		return m.updateSecretFunc(ctx, params, optFns...)
	}
	return nil, errors.New("UpdateSecret not implemented")
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		opts        []Option
		expectError bool
		validate    func(t *testing.T, p *Provider, err error)
	}{
		{
			name: "success with default config",
			opts: []Option{},
			validate: func(t *testing.T, p *Provider, err error) {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.NotNil(t, p.client)
				assert.NotNil(t, p.config)
				assert.Equal(t, "", p.config.Region)    // default empty
				assert.Equal(t, 0, p.config.MaxRetries) // default 0
			},
		},
		{
			name: "success with region option",
			opts: []Option{WithRegion("us-east-1")},
			validate: func(t *testing.T, p *Provider, err error) {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, "us-east-1", p.config.Region)
			},
		},
		{
			name: "success with max retries option",
			opts: []Option{WithMaxRetries(3)},
			validate: func(t *testing.T, p *Provider, err error) {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, 3, p.config.MaxRetries)
			},
		},
		{
			name: "success with endpoint option",
			opts: []Option{WithEndpoint("http://localhost:4566")},
			validate: func(t *testing.T, p *Provider, err error) {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, "http://localhost:4566", p.config.Endpoint)
			},
		},
		{
			name: "success with multiple options",
			opts: []Option{
				WithRegion("eu-west-1"),
				WithMaxRetries(5),
				WithEndpoint("http://test:4566"),
			},
			validate: func(t *testing.T, p *Provider, err error) {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, "eu-west-1", p.config.Region)
				assert.Equal(t, 5, p.config.MaxRetries)
				assert.Equal(t, "http://test:4566", p.config.Endpoint)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.opts...)
			tt.validate(t, p, err)
		})
	}
}

func TestNewWithConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		validate    func(t *testing.T, p *Provider, err error)
	}{
		{
			name: "success with valid config",
			config: &Config{
				Region:     "us-west-2",
				MaxRetries: 2,
				Endpoint:   "http://localhost:4566",
			},
			validate: func(t *testing.T, p *Provider, err error) {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, "us-west-2", p.config.Region)
				assert.Equal(t, 2, p.config.MaxRetries)
				assert.Equal(t, "http://localhost:4566", p.config.Endpoint)
			},
		},
		{
			name:        "success with nil config uses defaults",
			config:      nil,
			expectError: false,
			validate: func(t *testing.T, p *Provider, err error) {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.NotNil(t, p.config)
				assert.Equal(t, "", p.config.Region)
				assert.Equal(t, 0, p.config.MaxRetries)
				assert.Equal(t, "", p.config.Endpoint)
			},
		},
		{
			name:   "success with empty config",
			config: &Config{},
			validate: func(t *testing.T, p *Provider, err error) {
				require.NoError(t, err)
				require.NotNil(t, p)
				assert.Equal(t, "", p.config.Region)
				assert.Equal(t, 0, p.config.MaxRetries)
				assert.Equal(t, "", p.config.Endpoint)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewWithConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, p)
			} else {
				tt.validate(t, p, err)
			}
		})
	}
}

func TestProvider_Name(t *testing.T) {
	p, err := New()
	require.NoError(t, err)
	require.NotNil(t, p)

	name := p.Name()
	assert.Equal(t, "aws", name)
}

func TestProvider_HealthCheck(t *testing.T) {
	mockClient := &mockSecretsManagerClient{
		describeSecretFunc: func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
			// Return ResourceNotFoundException as expected for health check
			return nil, &types.ResourceNotFoundException{
				Message: aws.String("Secrets Manager can't find the specified secret."),
			}
		},
	}

	p := &Provider{
		client: mockClient,
		config: &Config{},
	}

	err := p.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestProvider_ResolveBatch(t *testing.T) {
	tests := []struct {
		name        string
		refs        []core.SecretRef
		mockSetup   func(*mockSecretsManagerClient)
		expectError bool
		validate    func(t *testing.T, results map[string]*core.Secret, err error)
	}{
		{
			name: "success with multiple secrets",
			refs: []core.SecretRef{
				{Path: "secret1"},
				{Path: "secret2"},
				{Path: "secret3"},
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					switch *params.SecretId {
					case "secret1":
						return &secretsmanager.GetSecretValueOutput{
							SecretString:  aws.String(`"value1"`),
							VersionStages: []string{"AWSCURRENT"},
							CreatedDate:   aws.Time(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
						}, nil
					case "secret2":
						return &secretsmanager.GetSecretValueOutput{
							SecretString:  aws.String(`"value2"`),
							VersionStages: []string{"AWSCURRENT"},
							CreatedDate:   aws.Time(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
						}, nil
					case "secret3":
						return &secretsmanager.GetSecretValueOutput{
							SecretBinary:  []byte{0x01, 0x02, 0x03},
							VersionStages: []string{"AWSCURRENT"},
							CreatedDate:   aws.Time(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
						}, nil
					}
					return nil, fmt.Errorf("unexpected secret ID: %s", *params.SecretId)
				}
			},
			validate: func(t *testing.T, results map[string]*core.Secret, err error) {
				assert.NoError(t, err)
				assert.Len(t, results, 3)
				assert.Equal(t, []byte(`"value1"`), results["secret1"].Value)
				assert.Equal(t, []byte(`"value2"`), results["secret2"].Value)
				assert.Equal(t, []byte{0x01, 0x02, 0x03}, results["secret3"].Value)
			},
		},
		{
			name: "partial success with some secrets missing",
			refs: []core.SecretRef{
				{Path: "existing-secret"},
				{Path: "missing-secret"},
				{Path: "another-existing"},
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					switch *params.SecretId {
					case "existing-secret":
						return &secretsmanager.GetSecretValueOutput{
							SecretString:  aws.String(`"exists"`),
							VersionStages: []string{"AWSCURRENT"},
							CreatedDate:   aws.Time(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
						}, nil
					case "missing-secret":
						return nil, &types.ResourceNotFoundException{
							Message: aws.String("Secret not found"),
						}
					case "another-existing":
						return &secretsmanager.GetSecretValueOutput{
							SecretString:  aws.String(`"another"`),
							VersionStages: []string{"AWSCURRENT"},
							CreatedDate:   aws.Time(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
						}, nil
					}
					return nil, fmt.Errorf("unexpected secret ID: %s", *params.SecretId)
				}
			},
			validate: func(t *testing.T, results map[string]*core.Secret, err error) {
				assert.NoError(t, err) // Partial failures should not cause overall error
				assert.Len(t, results, 2)
				assert.Contains(t, results, "existing-secret")
				assert.Contains(t, results, "another-existing")
				assert.NotContains(t, results, "missing-secret")
				assert.Equal(t, []byte(`"exists"`), results["existing-secret"].Value)
				assert.Equal(t, []byte(`"another"`), results["another-existing"].Value)
			},
		},
		{
			name: "empty refs slice",
			refs: []core.SecretRef{},
			mockSetup: func(m *mockSecretsManagerClient) {
				// Should not call the mock
			},
			validate: func(t *testing.T, results map[string]*core.Secret, err error) {
				assert.NoError(t, err)
				assert.Empty(t, results)
			},
		},
		{
			name: "single secret resolution",
			refs: []core.SecretRef{
				{Path: "single-secret", Version: "v1.0"},
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					assert.Equal(t, "single-secret", *params.SecretId)
					assert.Equal(t, "v1.0", *params.VersionId)
					return &secretsmanager.GetSecretValueOutput{
						SecretString:  aws.String(`"single"`),
						VersionStages: []string{"v1.0"},
						CreatedDate:   aws.Time(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					}, nil
				}
			},
			validate: func(t *testing.T, results map[string]*core.Secret, err error) {
				assert.NoError(t, err)
				assert.Len(t, results, 1)
				assert.Equal(t, []byte(`"single"`), results["single-secret"].Value)
				assert.Equal(t, "v1.0", results["single-secret"].Version)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSecretsManagerClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			p := &Provider{
				client: mockClient,
				config: &Config{},
			}

			results, err := p.ResolveBatch(context.Background(), tt.refs)

			if tt.expectError {
				assert.Error(t, err)
				if tt.validate != nil {
					tt.validate(t, results, err)
				}
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, results, err)
				}
			}
		})
	}
}

func TestProvider_Exists(t *testing.T) {
	tests := []struct {
		name        string
		ref         core.SecretRef
		mockSetup   func(*mockSecretsManagerClient)
		expectError bool
		expected    bool
		errorCheck  func(t *testing.T, err error)
	}{
		{
			name:     "secret exists",
			ref:      core.SecretRef{Path: "existing-secret"},
			expected: true,
			mockSetup: func(m *mockSecretsManagerClient) {
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					assert.Equal(t, "existing-secret", *params.SecretId)
					return &secretsmanager.DescribeSecretOutput{
						Name: aws.String("existing-secret"),
						ARN: aws.String(
							"arn:aws:secretsmanager:us-east-1:123456789012:secret:existing-secret",
						),
					}, nil
				}
			},
		},
		{
			name:     "secret does not exist",
			ref:      core.SecretRef{Path: "non-existent-secret"},
			expected: false,
			mockSetup: func(m *mockSecretsManagerClient) {
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &types.ResourceNotFoundException{
						Message: aws.String("Secrets Manager can't find the specified secret."),
					}
				}
			},
		},
		{
			name:        "access denied error",
			ref:         core.SecretRef{Path: "access-denied-secret"},
			expectError: true,
			expected:    false,
			mockSetup: func(m *mockSecretsManagerClient) {
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &types.InvalidParameterException{
						Message: aws.String("Access denied"),
					}
				}
			},
			errorCheck: func(t *testing.T, err error) {
				assert.True(t, core.IsProviderError(err))
			},
		},
		{
			name:        "generic AWS error",
			ref:         core.SecretRef{Path: "error-secret"},
			expectError: true,
			expected:    false,
			mockSetup: func(m *mockSecretsManagerClient) {
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			errorCheck: func(t *testing.T, err error) {
				assert.True(t, core.IsProviderError(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSecretsManagerClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			p := &Provider{
				client: mockClient,
				config: &Config{},
			}

			exists, err := p.Exists(context.Background(), tt.ref)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, tt.expected, exists)
				if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, exists)
			}
		})
	}
}

func TestProvider_Close(t *testing.T) {
	p, err := New()
	require.NoError(t, err)
	require.NotNil(t, p)

	err = p.Close()
	assert.NoError(t, err)
}

func TestProvider_Store(t *testing.T) {
	tests := []struct {
		name        string
		ref         core.SecretRef
		value       []byte
		mockSetup   func(*mockSecretsManagerClient)
		expectError bool
		errorCheck  func(t *testing.T, err error)
		validate    func(t *testing.T, mockClient *mockSecretsManagerClient)
	}{
		{
			name: "success creating new secret with JSON string",
			ref: core.SecretRef{
				Path: "new-secret",
			},
			value: []byte(`{"username":"test","password":"secret123"}`),
			mockSetup: func(m *mockSecretsManagerClient) {
				// First call to Exists should return false (secret doesn't exist)
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &types.ResourceNotFoundException{
						Message: aws.String("Secret not found"),
					}
				}
				// CreateSecret should be called
				m.createSecretFunc = func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					assert.Equal(t, "new-secret", *params.Name)
					assert.Equal(
						t,
						`{"username":"test","password":"secret123"}`,
						*params.SecretString,
					)
					assert.Nil(t, params.SecretBinary)
					return &secretsmanager.CreateSecretOutput{
						Name: aws.String("new-secret"),
						ARN: aws.String(
							"arn:aws:secretsmanager:us-east-1:123456789012:secret:new-secret",
						),
					}, nil
				}
			},
			validate: func(t *testing.T, mockClient *mockSecretsManagerClient) {
				// Ensure CreateSecret was called, not PutSecretValue
				assert.NotNil(t, mockClient.createSecretFunc)
			},
		},
		{
			name: "success updating existing secret",
			ref: core.SecretRef{
				Path: "existing-secret",
			},
			value: []byte("updated-secret-value"),
			mockSetup: func(m *mockSecretsManagerClient) {
				// First call to Exists should return true (secret exists)
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return &secretsmanager.DescribeSecretOutput{
						Name: aws.String("existing-secret"),
					}, nil
				}
				// PutSecretValue should be called for updates
				m.putSecretValueFunc = func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
					assert.NotNil(t, params.SecretId)
					assert.Equal(t, "existing-secret", *params.SecretId)
					assert.NotNil(t, params.SecretString)
					assert.Equal(t, "updated-secret-value", *params.SecretString)
					assert.Nil(t, params.SecretBinary)
					return &secretsmanager.PutSecretValueOutput{
						Name:      aws.String("existing-secret"),
						VersionId: aws.String("v2"),
					}, nil
				}
			},
			validate: func(t *testing.T, mockClient *mockSecretsManagerClient) {
				// Ensure PutSecretValue was called, not CreateSecret
				assert.NotNil(t, mockClient.putSecretValueFunc)
			},
		},
		{
			name: "success storing binary secret",
			ref: core.SecretRef{
				Path: "binary-secret",
			},
			value: []byte{0x00, 0x01, 0x02, 0x03},
			mockSetup: func(m *mockSecretsManagerClient) {
				// Secret doesn't exist
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &types.ResourceNotFoundException{
						Message: aws.String("Secret not found"),
					}
				}
				// CreateSecret should be called with binary data
				m.createSecretFunc = func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					assert.Equal(t, "binary-secret", *params.Name)
					assert.Nil(t, params.SecretString)
					assert.Equal(t, []byte{0x00, 0x01, 0x02, 0x03}, params.SecretBinary)
					return &secretsmanager.CreateSecretOutput{
						Name: aws.String("binary-secret"),
					}, nil
				}
			},
		},
		{
			name: "success with metadata/tags",
			ref: core.SecretRef{
				Path: "tagged-secret",
				Metadata: map[string]string{
					"Environment": "production",
					"Team":        "backend",
					"Application": "api-server",
				},
			},
			value: []byte("secret-with-tags"),
			mockSetup: func(m *mockSecretsManagerClient) {
				// Secret doesn't exist
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &types.ResourceNotFoundException{
						Message: aws.String("Secret not found"),
					}
				}
				// CreateSecret should be called with tags
				m.createSecretFunc = func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					assert.Equal(t, "tagged-secret", *params.Name)
					assert.Equal(t, "secret-with-tags", *params.SecretString)

					// Verify tags are set correctly
					assert.Len(t, params.Tags, 3)

					// Check that all expected tags are present (order may vary)
					tagMap := make(map[string]string)
					for _, tag := range params.Tags {
						tagMap[*tag.Key] = *tag.Value
					}
					assert.Equal(t, "production", tagMap["Environment"])
					assert.Equal(t, "backend", tagMap["Team"])
					assert.Equal(t, "api-server", tagMap["Application"])

					return &secretsmanager.CreateSecretOutput{
						Name: aws.String("tagged-secret"),
					}, nil
				}
			},
		},
		{
			name: "error empty path",
			ref: core.SecretRef{
				Path: "",
			},
			value:       []byte("some-value"),
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "empty")
			},
		},
		{
			name: "error access denied on exists check",
			ref: core.SecretRef{
				Path: "access-denied-secret",
			},
			value:       []byte("some-value"),
			expectError: true,
			mockSetup: func(m *mockSecretsManagerClient) {
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &types.InvalidParameterException{
						Message: aws.String("Access denied"),
					}
				}
			},
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.True(t, core.IsProviderError(err))
			},
		},
		{
			name: "error create secret fails",
			ref: core.SecretRef{
				Path: "failing-secret",
			},
			value:       []byte("some-value"),
			expectError: true,
			mockSetup: func(m *mockSecretsManagerClient) {
				// Secret doesn't exist
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &types.ResourceNotFoundException{
						Message: aws.String("Secret not found"),
					}
				}
				// CreateSecret fails
				m.createSecretFunc = func(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
					return nil, &types.InvalidParameterException{
						Message: aws.String("Invalid secret name"),
					}
				}
			},
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.True(t, core.IsProviderError(err))
			},
		},
		{
			name: "error update secret fails",
			ref: core.SecretRef{
				Path: "existing-failing-secret",
			},
			value:       []byte("updated-value"),
			expectError: true,
			mockSetup: func(m *mockSecretsManagerClient) {
				// Secret exists
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return &secretsmanager.DescribeSecretOutput{
						Name: aws.String("existing-failing-secret"),
					}, nil
				}
				// PutSecretValue fails
				m.putSecretValueFunc = func(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.True(t, core.IsProviderError(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSecretsManagerClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			p := &Provider{
				client: mockClient,
				config: &Config{},
			}

			err := p.Store(context.Background(), tt.ref, tt.value)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, mockClient)
				}
			}
		})
	}
}

func TestProvider_Delete(t *testing.T) {
	tests := []struct {
		name        string
		ref         core.SecretRef
		mockSetup   func(*mockSecretsManagerClient)
		expectError bool
		errorCheck  func(t *testing.T, err error)
		validate    func(t *testing.T, mockClient *mockSecretsManagerClient)
	}{
		{
			name: "success deleting existing secret with scheduled deletion",
			ref: core.SecretRef{
				Path: "existing-secret",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				// Secret exists for deletion
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return &secretsmanager.DescribeSecretOutput{
						Name: aws.String("existing-secret"),
					}, nil
				}
				// DeleteSecret should be called with scheduled deletion
				m.deleteSecretFunc = func(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
					assert.Equal(t, "existing-secret", *params.SecretId)
					assert.NotNil(t, params.RecoveryWindowInDays)
					assert.Equal(t, int64(7), *params.RecoveryWindowInDays) // 7-day recovery window
					assert.Nil(
						t,
						params.ForceDeleteWithoutRecovery,
					) // Should not force delete
					return &secretsmanager.DeleteSecretOutput{
						Name:         aws.String("existing-secret"),
						DeletionDate: aws.Time(time.Now().Add(7 * 24 * time.Hour)),
					}, nil
				}
			},
			validate: func(t *testing.T, mockClient *mockSecretsManagerClient) {
				// Ensure DeleteSecret was called
				assert.NotNil(t, mockClient.deleteSecretFunc)
			},
		},
		{
			name: "success deleting non-existent secret (no error)",
			ref: core.SecretRef{
				Path: "non-existent-secret",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				// Secret doesn't exist
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &types.ResourceNotFoundException{
						Message: aws.String("Secret not found"),
					}
				}
				// DeleteSecret should not be called
			},
			validate: func(t *testing.T, mockClient *mockSecretsManagerClient) {
				// Ensure DeleteSecret was NOT called
				assert.Nil(t, mockClient.deleteSecretFunc)
			},
		},
		{
			name: "success deleting already scheduled for deletion (no error)",
			ref: core.SecretRef{
				Path: "scheduled-for-deletion-secret",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				// Secret exists but is scheduled for deletion
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &types.InvalidRequestException{
						Message: aws.String(
							"You tried to perform an operation on a secret that's currently marked deleted.",
						),
					}
				}
				// DeleteSecret should not be called
			},
			validate: func(t *testing.T, mockClient *mockSecretsManagerClient) {
				// Ensure DeleteSecret was NOT called
				assert.Nil(t, mockClient.deleteSecretFunc)
			},
		},
		{
			name: "error empty path",
			ref: core.SecretRef{
				Path: "",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "empty")
			},
		},
		{
			name: "error access denied on exists check",
			ref: core.SecretRef{
				Path: "access-denied-secret",
			},
			expectError: true,
			mockSetup: func(m *mockSecretsManagerClient) {
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return nil, &types.InvalidParameterException{
						Message: aws.String("Access denied"),
					}
				}
			},
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.True(t, core.IsProviderError(err))
			},
		},
		{
			name: "error delete secret fails",
			ref: core.SecretRef{
				Path: "failing-delete-secret",
			},
			expectError: true,
			mockSetup: func(m *mockSecretsManagerClient) {
				// Secret exists
				m.describeSecretFunc = func(ctx context.Context, params *secretsmanager.DescribeSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
					return &secretsmanager.DescribeSecretOutput{
						Name: aws.String("failing-delete-secret"),
					}, nil
				}
				// DeleteSecret fails
				m.deleteSecretFunc = func(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
					return nil, &types.InvalidRequestException{
						Message: aws.String("Cannot delete secret"),
					}
				}
			},
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.True(t, core.IsProviderError(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSecretsManagerClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			p := &Provider{
				client: mockClient,
				config: &Config{},
			}

			err := p.Delete(context.Background(), tt.ref)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, mockClient)
				}
			}
		})
	}
}

func TestProvider_Resolve(t *testing.T) {
	tests := []struct {
		name        string
		ref         core.SecretRef
		mockSetup   func(*mockSecretsManagerClient)
		expectError bool
		errorCheck  func(t *testing.T, err error)
		validate    func(t *testing.T, secret *core.Secret)
	}{
		{
			name: "success with string secret",
			ref: core.SecretRef{
				Path: "test-secret",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					assert.Equal(t, "test-secret", *params.SecretId)
					return &secretsmanager.GetSecretValueOutput{
						SecretString:  aws.String(`"secret-value"`),
						VersionStages: []string{"AWSCURRENT"},
						CreatedDate:   aws.Time(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					}, nil
				}
			},
			validate: func(t *testing.T, secret *core.Secret) {
				assert.NotNil(t, secret)
				assert.Equal(t, []byte(`"secret-value"`), secret.Value)
				assert.Equal(t, "", secret.Version) // Version not specified in ref
				assert.True(t, secret.CreatedAt.Equal(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)))
				assert.Nil(t, secret.ExpiresAt)
			},
		},
		{
			name: "success with binary secret",
			ref: core.SecretRef{
				Path: "test-binary-secret",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					assert.Equal(t, "test-binary-secret", *params.SecretId)
					return &secretsmanager.GetSecretValueOutput{
						SecretBinary:  []byte{0x01, 0x02, 0x03, 0x04},
						VersionStages: []string{"AWSCURRENT"},
						CreatedDate:   aws.Time(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					}, nil
				}
			},
			validate: func(t *testing.T, secret *core.Secret) {
				assert.NotNil(t, secret)
				assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, secret.Value)
				assert.Equal(t, "", secret.Version)
				assert.True(t, secret.CreatedAt.Equal(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)))
				assert.Nil(t, secret.ExpiresAt)
			},
		},
		{
			name: "success with version ID",
			ref: core.SecretRef{
				Path:    "test-secret",
				Version: "v1.0",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					assert.Equal(t, "test-secret", *params.SecretId)
					assert.Equal(t, "v1.0", *params.VersionId)
					return &secretsmanager.GetSecretValueOutput{
						SecretString:  aws.String(`"versioned-secret"`),
						VersionStages: []string{"v1.0"},
						CreatedDate:   aws.Time(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					}, nil
				}
			},
			validate: func(t *testing.T, secret *core.Secret) {
				assert.NotNil(t, secret)
				assert.Equal(t, []byte(`"versioned-secret"`), secret.Value)
				assert.Equal(t, "v1.0", secret.Version)
			},
		},
		{
			name: "success with version stage",
			ref: core.SecretRef{
				Path:    "test-secret",
				Version: "AWSPREVIOUS",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					assert.Equal(t, "test-secret", *params.SecretId)
					assert.Equal(t, "AWSPREVIOUS", *params.VersionStage)
					return &secretsmanager.GetSecretValueOutput{
						SecretString:  aws.String(`"previous-secret"`),
						VersionStages: []string{"AWSPREVIOUS"},
						CreatedDate:   aws.Time(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
					}, nil
				}
			},
			validate: func(t *testing.T, secret *core.Secret) {
				assert.NotNil(t, secret)
				assert.Equal(t, []byte(`"previous-secret"`), secret.Value)
				assert.Equal(t, "AWSPREVIOUS", secret.Version)
			},
		},
		{
			name: "error secret not found",
			ref: core.SecretRef{
				Path: "non-existent-secret",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return nil, &types.ResourceNotFoundException{
						Message: aws.String("Secrets Manager can't find the specified secret."),
					}
				}
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, core.ErrSecretNotFound))
			},
		},
		{
			name: "error access denied",
			ref: core.SecretRef{
				Path: "access-denied-secret",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return nil, &types.InvalidParameterException{
						Message: aws.String("Access denied"),
					}
				}
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, core.ErrAccessDenied))
			},
		},
		{
			name: "error invalid request",
			ref: core.SecretRef{
				Path: "invalid-secret",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return nil, &types.InvalidRequestException{
						Message: aws.String("Invalid request"),
					}
				}
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.True(t, core.IsProviderError(err))
			},
		},
		{
			name: "error generic AWS error",
			ref: core.SecretRef{
				Path: "error-secret",
			},
			mockSetup: func(m *mockSecretsManagerClient) {
				m.getSecretValueFunc = func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
					return nil, fmt.Errorf("generic AWS error")
				}
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.True(t, core.IsProviderError(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockSecretsManagerClient{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockClient)
			}

			p := &Provider{
				client: mockClient,
				config: &Config{},
			}

			secret, err := p.Resolve(context.Background(), tt.ref)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, secret)
				if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, secret)
				}
			}
		})
	}
}
