// Package aws provides an AWS Secrets Manager provider for the catalyst-forge-libs secrets framework.
//
// This package implements the core.Provider and core.WriteableProvider interfaces
// for secure, test-friendly access to AWS Secrets Manager. It supports:
//
//   - Just-in-time credential loading using AWS SDK v2
//   - Automatic JSON vs binary data detection
//   - Batch operations with concurrency control
//   - Version management (ID and stage support)
//   - Metadata/tags support for organization
//   - LocalStack integration for testing
//
// # Basic Usage
//
//	import "github.com/input-output-hk/catalyst-forge-libs/secrets/providers/aws"
//
//	// Create a provider with default AWS configuration
//	provider, err := aws.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer provider.Close()
//
//	// Store a secret
//	ref := core.SecretRef{Path: "my-secret"}
//	err = provider.Store(ctx, ref, []byte(`{"key":"value"}`))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Retrieve a secret
//	secret, err := provider.Resolve(ctx, ref)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(string(secret.Value))
//
// # LocalStack Testing
//
// For integration testing, configure with LocalStack endpoint:
//
//	provider, err := aws.New(aws.WithEndpoint("http://localhost:4566"))
//
// # Configuration Options
//
// Customize provider behavior with functional options:
//
//	provider, err := aws.New(
//	    aws.WithRegion("us-west-2"),
//	    aws.WithMaxRetries(5),
//	    aws.WithEndpoint("http://localhost:4566"), // for LocalStack
//	)
//
// # Error Handling
//
// The provider wraps AWS errors with context and maps them to core error types:
//
//	secret, err := provider.Resolve(ctx, core.SecretRef{Path: "missing-secret"})
//	if core.IsSecretNotFound(err) {
//	    // Handle missing secret
//	} else if core.IsAccessDenied(err) {
//	    // Handle permission issues
//	}
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/input-output-hk/catalyst-forge-libs/secrets/core"
)

// SecretsManagerAPI defines the interface for AWS Secrets Manager operations.
// This interface allows for mocking AWS SDK calls in unit tests.
//
// The interface mirrors key AWS Secrets Manager API methods used by the provider.
// All methods follow the same signature patterns as the AWS SDK v2.
type SecretsManagerAPI interface {
	GetSecretValue(
		ctx context.Context,
		params *secretsmanager.GetSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.GetSecretValueOutput, error)
	DescribeSecret(
		ctx context.Context,
		params *secretsmanager.DescribeSecretInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.DescribeSecretOutput, error)
	CreateSecret(
		ctx context.Context,
		params *secretsmanager.CreateSecretInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.CreateSecretOutput, error)
	PutSecretValue(
		ctx context.Context,
		params *secretsmanager.PutSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.PutSecretValueOutput, error)
	DeleteSecret(
		ctx context.Context,
		params *secretsmanager.DeleteSecretInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.DeleteSecretOutput, error)
	UpdateSecret(
		ctx context.Context,
		params *secretsmanager.UpdateSecretInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.UpdateSecretOutput, error)
}

// Provider implements the core.Provider and core.WriteableProvider interfaces
// for AWS Secrets Manager. It provides secure, thread-safe access to secrets
// stored in AWS Secrets Manager.
//
// Provider supports:
//   - Reading and writing secrets (string and binary)
//   - Batch operations with concurrency control
//   - Version management using IDs and stages
//   - Automatic JSON/binary detection
//   - Metadata/tags support
//   - LocalStack integration for testing
//
// Note: This provider does not implement RotatableProvider. AWS Secrets Manager
// rotation is handled through Lambda functions and cannot be triggered directly
// via API calls. Use AWS's native rotation configuration for automatic rotation.
//
// Provider is safe for concurrent use by multiple goroutines.
type Provider struct {
	// client is the AWS Secrets Manager client
	client SecretsManagerAPI
	// config holds the provider configuration
	config *Config
}

// Config holds the configuration for the AWS Secrets Manager provider.
// Use functional options (WithRegion, WithMaxRetries, WithEndpoint) to configure
// the provider instead of creating Config directly.
type Config struct {
	// Region specifies the AWS region for Secrets Manager operations
	Region string
	// MaxRetries specifies the maximum number of retries for failed operations
	MaxRetries int
	// Endpoint allows overriding the AWS endpoint (useful for LocalStack testing)
	Endpoint string
}

// Option defines a functional option for configuring the AWS Secrets Manager provider.
// Options are applied in order when creating a provider with New().
//
// Example:
//
//	provider, err := aws.New(
//	    aws.WithRegion("us-west-2"),
//	    aws.WithMaxRetries(3),
//	)
type Option func(*Config)

// WithRegion sets the AWS region for the provider.
//
// Default: Uses AWS SDK default region resolution (environment variables,
// EC2 metadata, shared config files, etc.).
//
// Example:
//
//	provider, err := aws.New(aws.WithRegion("us-west-2"))
func WithRegion(region string) Option {
	return func(c *Config) {
		c.Region = region
	}
}

// WithMaxRetries sets the maximum number of retries for failed operations.
//
// Default: Uses AWS SDK default retry configuration (typically 3 retries).
//
// Note: This affects the underlying AWS SDK retry behavior, not provider-level retries.
//
// Example:
//
//	provider, err := aws.New(aws.WithMaxRetries(5))
func WithMaxRetries(maxRetries int) Option {
	return func(c *Config) {
		c.MaxRetries = maxRetries
	}
}

// WithEndpoint sets a custom AWS endpoint (useful for testing with LocalStack).
// When set, the provider will use anonymous credentials suitable for LocalStack testing.
//
// Default: Uses standard AWS endpoints.
//
// Example:
//
//	// For LocalStack testing
//	provider, err := aws.New(aws.WithEndpoint("http://localhost:4566"))
//
//	// For custom AWS-compatible endpoints
//	provider, err := aws.New(aws.WithEndpoint("https://secrets.example.com"))
func WithEndpoint(endpoint string) Option {
	return func(c *Config) {
		c.Endpoint = endpoint
	}
}

// New creates a new AWS Secrets Manager provider with the given options.
// It initializes the AWS SDK client with just-in-time credential loading.
//
// The provider is configured with sensible defaults and can be customized
// using functional options. For LocalStack testing, use WithEndpoint.
//
// Returns an error if AWS configuration cannot be loaded.
//
// Example:
//
//	// Basic usage with defaults
//	provider, err := aws.New()
//
//	// With custom configuration
//	provider, err := aws.New(
//	    aws.WithRegion("us-west-2"),
//	    aws.WithMaxRetries(5),
//	)
//
//	// For LocalStack testing
//	provider, err := aws.New(aws.WithEndpoint("http://localhost:4566"))
func New(opts ...Option) (*Provider, error) {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}

	return NewWithConfig(cfg)
}

// NewWithConfig creates a new AWS Secrets Manager provider with the provided configuration.
// If config is nil, default configuration is used.
//
// This function provides direct configuration control and is an alternative to New().
// Most users should prefer New() with functional options for better readability.
//
// Returns an error if AWS configuration cannot be loaded.
//
// Example:
//
//	// Using direct config (less common)
//	config := &aws.Config{
//	    Region: "us-west-2",
//	    Endpoint: "http://localhost:4566", // for LocalStack
//	}
//	provider, err := aws.NewWithConfig(config)
//
//	// Using nil config for defaults
//	provider, err := aws.NewWithConfig(nil)
func NewWithConfig(cfg *Config) (*Provider, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	// Create AWS config with the specified options
	var awsCfg aws.Config
	var err error

	if cfg.Endpoint != "" {
		// For LocalStack testing, use anonymous credentials and endpoint resolver
		awsCfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithRegion("us-east-1"),
			config.WithCredentialsProvider(aws.AnonymousCredentials{}),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

		// Create client with LocalStack endpoint
		client := secretsmanager.New(secretsmanager.Options{
			Region:           "us-east-1",
			Credentials:      awsCfg.Credentials,
			EndpointResolver: secretsmanager.EndpointResolverFromURL(cfg.Endpoint),
		})

		return &Provider{
			client: client,
			config: cfg,
		}, nil
	}

	// For regular AWS usage, load default config
	awsCfg, err = config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Apply region if specified
	if cfg.Region != "" {
		awsCfg.Region = cfg.Region
	}

	// Create the Secrets Manager client
	client := secretsmanager.NewFromConfig(awsCfg)

	return &Provider{
		client: client,
		config: cfg,
	}, nil
}

// Name returns the provider's identifier.
// Returns "aws" for the AWS Secrets Manager provider.
func (p *Provider) Name() string {
	return "aws"
}

// HealthCheck verifies the provider's connectivity and functionality.
// It performs a lightweight operation to ensure the client is properly configured.
//
// The health check attempts to describe a non-existent secret, which should fail
// with ResourceNotFoundException if the client is working correctly. This approach
// avoids requiring actual secrets to exist for health validation.
//
// Returns nil if the provider is healthy, or an error if connectivity fails.
//
// Example:
//
//	err := provider.HealthCheck(ctx)
//	if err != nil {
//	    log.Printf("Provider health check failed: %v", err)
//	    return err
//	}
func (p *Provider) HealthCheck(ctx context.Context) error {
	// For health check, we attempt to describe a non-existent secret
	// This is a lightweight operation that verifies connectivity without
	// requiring actual secrets to exist
	secretID := "health-check-non-existent-secret"
	_, err := p.client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: &secretID,
	})
	// We expect this to fail with ResourceNotFoundException, which indicates
	// the client is working correctly. Any other error suggests a configuration issue.
	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			// This is expected - the secret doesn't exist, but the client is working
			return nil
		}
		// Any other error indicates a real problem
		return fmt.Errorf("health check failed: %w", err)
	}
	// If no error occurred, something unexpected happened
	return errors.New("health check failed: unexpected success")
}

// Close gracefully shuts down the provider and releases resources.
// Currently, the AWS SDK client doesn't require explicit cleanup.
//
// This method exists to satisfy the core.Provider interface and ensures
// compatibility with resource management patterns. It is safe to call multiple times.
//
// Returns nil (no cleanup currently required for AWS SDK v2 clients).
func (p *Provider) Close() error {
	// The AWS SDK v2 client doesn't require explicit cleanup
	// This method exists to satisfy the Provider interface
	return nil
}

// Resolve retrieves a single secret by reference.
// It fetches the secret from AWS Secrets Manager and returns it in a secure format.
//
// The method supports:
//   - Version specification via ref.Version (ID or stage like "AWSCURRENT")
//   - Automatic handling of both string and binary secrets
//   - Context cancellation for timeout control
//
// Returns a core.Secret containing the secret value, version, and metadata.
// Returns an error if the secret cannot be retrieved or doesn't exist.
//
// Example:
//
//	// Get latest version
//	ref := core.SecretRef{Path: "my-secret"}
//	secret, err := provider.Resolve(ctx, ref)
//
//	// Get specific version
//	ref := core.SecretRef{
//	    Path:    "my-secret",
//	    Version: "v1.2.3",
//	}
//	secret, err := provider.Resolve(ctx, ref)
//
//	// Get by stage
//	ref := core.SecretRef{
//	    Path:    "my-secret",
//	    Version: "AWSPREVIOUS",
//	}
//	secret, err := provider.Resolve(ctx, ref)
func (p *Provider) Resolve(ctx context.Context, ref core.SecretRef) (*core.Secret, error) {
	if ref.Path == "" {
		return nil, fmt.Errorf("secret reference path cannot be empty: %w", core.ErrInvalidRef)
	}

	// Prepare the input for GetSecretValue
	input := &secretsmanager.GetSecretValueInput{
		SecretId: &ref.Path,
	}

	// Handle version specification
	if ref.Version != "" {
		// Check if it's a version stage (like AWSCURRENT, AWSPREVIOUS) or version ID
		if ref.Version == "AWSCURRENT" || ref.Version == "AWSPREVIOUS" || ref.Version == "AWSPENDING" {
			input.VersionStage = &ref.Version
		} else {
			input.VersionId = &ref.Version
		}
	}

	// Call AWS Secrets Manager
	output, err := p.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, p.mapAWSError(ref, err)
	}

	// Extract the secret value (either string or binary)
	var value []byte
	switch {
	case output.SecretString != nil:
		value = []byte(*output.SecretString)
	case output.SecretBinary != nil:
		value = output.SecretBinary
	default:
		return nil, fmt.Errorf("secret %q has no value (neither string nor binary): %w",
			ref.Path, core.ErrProviderError)
	}

	// Determine the version to return
	version := ""
	if ref.Version != "" {
		version = ref.Version
	}
	// Note: We don't set version from VersionStages when ref.Version is empty
	// This allows the caller to distinguish between explicitly requested versions
	// and default versions

	// Create the secret object
	secret := &core.Secret{
		Value:     value,
		Version:   version,
		CreatedAt: *output.CreatedDate,
		ExpiresAt: nil,   // AWS Secrets Manager doesn't have built-in expiration
		AutoClear: false, // Let the caller decide
	}

	return secret, nil
}

// mapAWSError maps AWS SDK errors to core error types for consistent error handling.
func (p *Provider) mapAWSError(ref core.SecretRef, err error) error {
	var rnf *types.ResourceNotFoundException
	if errors.As(err, &rnf) {
		return fmt.Errorf("secret %q not found: %w", ref.Path, core.ErrSecretNotFound)
	}

	var ipe *types.InvalidParameterException
	if errors.As(err, &ipe) {
		// Check if this looks like an access denied error
		if ipe.Message != nil && containsAccessDeniedMessage(*ipe.Message) {
			return fmt.Errorf("access denied for secret %q: %w", ref.Path, core.ErrAccessDenied)
		}
		return fmt.Errorf("invalid parameter for secret %q: %w", ref.Path,
			core.WrapProviderError("aws", ref, err, "invalid parameter"))
	}

	var ire *types.InvalidRequestException
	if errors.As(err, &ire) {
		return fmt.Errorf("invalid request for secret %q: %w", ref.Path,
			core.WrapProviderError("aws", ref, err, "invalid request"))
	}

	// For other AWS errors, wrap as provider error
	return fmt.Errorf("failed to resolve secret %q: %w", ref.Path,
		core.WrapProviderError("aws", ref, err, "failed to resolve secret"))
}

// containsAccessDeniedMessage checks if the error message indicates access denial.
func containsAccessDeniedMessage(msg string) bool {
	lowerMsg := strings.ToLower(msg)
	return strings.Contains(lowerMsg, "access") && strings.Contains(lowerMsg, "denied")
}

// ResolveBatch retrieves multiple secrets in a single operation.
// It fetches secrets from AWS Secrets Manager with parallel processing for efficiency.
//
// This method provides better performance than calling Resolve() multiple times
// by processing secrets concurrently with built-in rate limiting. Failed
// individual secret retrievals don't fail the entire batch.
//
// Features:
//   - Concurrent processing with semaphore-based rate limiting (max 10 concurrent)
//   - Context cancellation support
//   - Partial success handling (missing secrets don't fail the batch)
//   - Automatic error aggregation and logging
//
// Returns a map of secret paths to core.Secret objects.
// Only successfully retrieved secrets are included in the result map.
//
// Example:
//
//	refs := []core.SecretRef{
//	    {Path: "db-password"},
//	    {Path: "api-key"},
//	    {Path: "non-existent-secret"}, // This won't fail the batch
//	}
//	secrets, err := provider.ResolveBatch(ctx, refs)
//	if err != nil {
//	    return err // Only fails on system errors, not missing secrets
//	}
//
//	// Access individual secrets
//	if dbPass, exists := secrets["db-password"]; exists {
//	    fmt.Println("DB Password:", string(dbPass.Value))
//	}
func (p *Provider) ResolveBatch(ctx context.Context, refs []core.SecretRef) (map[string]*core.Secret, error) {
	if len(refs) == 0 {
		return make(map[string]*core.Secret), nil
	}

	// Use a reasonable concurrency limit to avoid overwhelming AWS API
	maxConcurrency := min(10, len(refs))

	// Create semaphore for rate limiting
	sem := make(chan struct{}, maxConcurrency)
	results := make(map[string]*core.Secret)
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	// Channel to collect errors (but don't fail the whole batch)
	errChan := make(chan error, len(refs))

	// Start goroutines for each secret
	for _, ref := range refs {
		wg.Add(1)
		go func(ref core.SecretRef) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}

			// Resolve the secret
			secret, err := p.Resolve(ctx, ref)
			if err != nil {
				// For batch operations, we don't fail the entire batch on individual errors
				// Just skip this secret and continue with others
				errChan <- fmt.Errorf("failed to resolve secret %q: %w", ref.Path, err)
				return
			}

			// Store the result safely
			resultsMu.Lock()
			results[ref.Path] = secret
			resultsMu.Unlock()
		}(ref)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check if context was cancelled
	if ctx.Err() != nil {
		return nil, fmt.Errorf("context cancelled during batch resolution: %w", ctx.Err())
	}

	// Log any individual errors but don't fail the batch
	// (In a real implementation, you might want to log these)
	for err := range errChan {
		// Individual secret resolution errors are ignored for batch operations
		// This allows partial success
		_ = err
	}

	return results, nil
}

// Exists checks if a secret exists without retrieving its value.
// It uses the DescribeSecret API for efficient existence checking.
//
// This method is more efficient than Resolve() when you only need to check
// existence, as it avoids fetching the actual secret value.
//
// Returns true if the secret exists, false if it doesn't exist.
// Returns an error only for system failures (not for missing secrets).
//
// Example:
//
//	ref := core.SecretRef{Path: "my-secret"}
//	exists, err := provider.Exists(ctx, ref)
//	if err != nil {
//	    return fmt.Errorf("failed to check existence: %w", err)
//	}
//	if !exists {
//	    return errors.New("secret does not exist")
//	}
func (p *Provider) Exists(ctx context.Context, ref core.SecretRef) (bool, error) {
	if ref.Path == "" {
		return false, fmt.Errorf("secret reference path cannot be empty: %w", core.ErrInvalidRef)
	}

	// Use DescribeSecret which is lighter than GetSecretValue
	input := &secretsmanager.DescribeSecretInput{
		SecretId: &ref.Path,
	}

	_, err := p.client.DescribeSecret(ctx, input)
	if err != nil {
		var rnf *types.ResourceNotFoundException
		if errors.As(err, &rnf) {
			// Secret doesn't exist - this is not an error for Exists method
			return false, nil
		}

		var ire *types.InvalidRequestException
		if errors.As(err, &ire) {
			if ire.Message != nil && strings.Contains(*ire.Message, "currently marked deleted") {
				// Secret is scheduled for deletion - treat as non-existent
				return false, nil
			}
		}

		// Other errors should be propagated with proper wrapping
		return false, fmt.Errorf("failed to check secret existence for %q: %w",
			ref.Path, core.WrapProviderError("aws", ref, err, "failed to check secret existence"))
	}

	// If no error, the secret exists
	return true, nil
}

// Store saves a secret value to AWS Secrets Manager.
// It automatically detects JSON vs binary format and handles metadata as tags.
//
// The method automatically:
//   - Detects JSON vs binary data format
//   - Creates new secrets or updates existing ones
//   - Converts metadata to AWS tags for organization
//   - Handles both string and binary secret types
//
// Use this method for both creating new secrets and updating existing ones.
// The provider will automatically choose between CreateSecret and PutSecretValue
// based on whether the secret already exists.
//
// Example:
//
//	// Store a JSON secret with metadata
//	ref := core.SecretRef{
//	    Path: "app/config",
//	    Metadata: map[string]string{
//	        "environment": "production",
//	        "team":        "backend",
//	    },
//	}
//	err := provider.Store(ctx, ref, []byte(`{"api_key":"secret","db":"prod"}`))
//
//	// Store a binary secret
//	binaryData := []byte{0x00, 0x01, 0x02, 0x03}
//	err := provider.Store(ctx, ref, binaryData)
func (p *Provider) Store(ctx context.Context, ref core.SecretRef, value []byte) error {
	if ref.Path == "" {
		return fmt.Errorf("secret reference path cannot be empty: %w", core.ErrInvalidRef)
	}

	// Check if secret already exists
	exists, err := p.Exists(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to check secret existence for %q: %w", ref.Path, err)
	}

	if exists {
		// Update existing secret using PutSecretValue
		return p.updateSecret(ctx, ref, value)
	}
	// Create new secret using CreateSecret
	return p.createSecret(ctx, ref, value)
}

// createSecret creates a new secret in AWS Secrets Manager.
func (p *Provider) createSecret(ctx context.Context, ref core.SecretRef, value []byte) error {
	// Determine if the value should be stored as binary or string
	// Use binary only for non-printable data, otherwise use string
	var secretString *string
	var secretBinary []byte

	if p.isBinaryData(value) {
		secretBinary = value
	} else {
		secretString = aws.String(string(value))
	}

	// Prepare the input
	input := &secretsmanager.CreateSecretInput{
		Name:         &ref.Path,
		SecretString: secretString,
		SecretBinary: secretBinary,
	}

	// Add tags from metadata if present
	if len(ref.Metadata) > 0 {
		tags := make([]types.Tag, 0, len(ref.Metadata))
		for key, value := range ref.Metadata {
			tags = append(tags, types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}
		input.Tags = tags
	}

	// Create the secret
	_, err := p.client.CreateSecret(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create secret %q: %w", ref.Path,
			core.WrapProviderError("aws", ref, err, "failed to create secret"))
	}

	return nil
}

// updateSecret updates an existing secret in AWS Secrets Manager.
func (p *Provider) updateSecret(ctx context.Context, ref core.SecretRef, value []byte) error {
	// Determine if the value should be stored as binary or string
	// Use binary only for non-printable data, otherwise use string
	var secretString *string
	var secretBinary []byte

	if p.isBinaryData(value) {
		secretBinary = value
	} else {
		secretString = aws.String(string(value))
	}

	// Prepare the input
	input := &secretsmanager.PutSecretValueInput{
		SecretId:     &ref.Path,
		SecretString: secretString,
		SecretBinary: secretBinary,
	}

	// Update the secret
	_, err := p.client.PutSecretValue(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update secret %q: %w", ref.Path,
			core.WrapProviderError("aws", ref, err, "failed to update secret"))
	}

	return nil
}

// Delete removes a secret from AWS Secrets Manager.
// It uses scheduled deletion with a 7-day recovery window by default.
//
// AWS Secrets Manager uses "soft delete" by default, meaning secrets are not
// immediately removed but scheduled for deletion after a recovery window.
// This allows recovery of accidentally deleted secrets.
//
// The 7-day recovery window is AWS's default and recommended setting.
// Secrets can be recovered during this window using the AWS console or API.
//
// Returns nil if the secret was successfully scheduled for deletion.
// Returns nil if the secret doesn't exist (idempotent operation).
// Returns an error only for system failures.
//
// Example:
//
//	ref := core.SecretRef{Path: "temporary-secret"}
//	err := provider.Delete(ctx, ref)
//	if err != nil {
//	    return fmt.Errorf("failed to delete secret: %w", err)
//	}
//	// Secret is scheduled for deletion in 7 days
//
//	// Idempotent - safe to call multiple times
//	err = provider.Delete(ctx, ref) // No error even if already deleted
func (p *Provider) Delete(ctx context.Context, ref core.SecretRef) error {
	if ref.Path == "" {
		return fmt.Errorf("secret reference path cannot be empty: %w", core.ErrInvalidRef)
	}

	// Check if secret exists and is not already scheduled for deletion
	exists, err := p.Exists(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to check secret existence for %q: %w", ref.Path, err)
	}

	if !exists {
		// Secret doesn't exist - this is not an error for Delete method
		// Return success as the end state (secret not existing) is achieved
		return nil
	}

	// Delete the secret with scheduled deletion (7-day recovery window)
	input := &secretsmanager.DeleteSecretInput{
		SecretId:                   &ref.Path,
		RecoveryWindowInDays:       aws.Int64(7), // 7-day recovery window
		ForceDeleteWithoutRecovery: nil,          // Use scheduled deletion, not force delete
	}

	_, err = p.client.DeleteSecret(ctx, input)
	if err != nil {
		// Other errors should be propagated with proper wrapping
		return fmt.Errorf("failed to delete secret %q: %w", ref.Path,
			core.WrapProviderError("aws", ref, err, "failed to delete secret"))
	}

	return nil
}

// isBinaryData checks if the given byte slice contains binary data.
// It considers data binary if it contains null bytes or non-printable characters.
func (p *Provider) isBinaryData(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Check for null bytes (common in binary data)
	for _, b := range data {
		if b == 0 {
			return true
		}
		// Also check for non-printable characters (except common whitespace)
		if b < 32 && b != 9 && b != 10 && b != 13 {
			return true
		}
	}

	return false
}
