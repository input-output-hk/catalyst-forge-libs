// Package secrets provides a Go client for AWS Secrets Manager with caching, retry logic,
// and comprehensive error handling.
//
// # Security Considerations
//
// This package is designed with security as a first-class concern:
//
// ## IAM Permissions
//
// The following IAM permissions are required depending on the operations used:
//
// ### GetSecret Operations
// - secretsmanager:GetSecretValue - Required for GetSecret and GetSecretCached
// - secretsmanager:DescribeSecret - Required for DescribeSecret
// - kms:Decrypt - Required if the secret is encrypted with a customer-managed KMS key
//
// ### Write Operations
// - secretsmanager:PutSecretValue - Required for PutSecret
// - secretsmanager:CreateSecret - Required for CreateSecret
// - kms:GenerateDataKey - Required for CreateSecret with customer-managed KMS keys
// - kms:Decrypt - Required for PutSecret with customer-managed KMS keys
//
// ### Example IAM Policy
// ```json
//
//	{
//	  "Version": "2012-10-17",
//	  "Statement": [
//	    {
//	      "Effect": "Allow",
//	      "Action": [
//	        "secretsmanager:GetSecretValue",
//	        "secretsmanager:PutSecretValue",
//	        "secretsmanager:CreateSecret",
//	        "secretsmanager:DescribeSecret"
//	      ],
//	      "Resource": "*"
//	    },
//	    {
//	      "Effect": "Allow",
//	      "Action": [
//	        "kms:Decrypt",
//	        "kms:GenerateDataKey"
//	      ],
//	      "Resource": "*"
//	    }
//	  ]
//	}
//
// ```
//
// ## Security Best Practices
//
// - Never log secret values - This package only logs secret names and operation metadata
// - Use appropriate IAM permissions - Follow the principle of least privilege
// - Use customer-managed KMS keys for enhanced security when possible
// - Implement proper error handling to avoid information leakage through error messages
// - Use caching to reduce API calls and improve performance while maintaining security
//
// ## Thread Safety
//
// All Client methods are safe for concurrent use by multiple goroutines.
// The AWS SDK v2 client is thread-safe, and this wrapper maintains that property.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/smithy-go"
)

// AWS error code constants
const (
	ResourceNotFoundException = "ResourceNotFoundException"
	AccessDeniedException     = "AccessDeniedException"
)

// Client provides a high-level interface for interacting with AWS Secrets Manager.
// It supports caching, custom retry logic, and structured logging.
//
// Thread Safety: This struct is thread-safe for concurrent use.
// - The api field is immutable and AWS SDK v2 clients are thread-safe
// - The logger field is typically thread-safe (slog.Logger)
// - The cache field must implement thread-safe operations (see Cache interface)
// - The retryer field should be thread-safe (see Retryer interface)
//
// All Client methods are safe for concurrent access by multiple goroutines.
type Client struct {
	// api is the underlying AWS Secrets Manager client (thread-safe)
	api ManagerAPI

	// logger is used for structured logging of operations (thread-safe)
	logger *slog.Logger

	// cache provides optional caching of secret values (must be thread-safe)
	cache Cache

	// retryer provides custom retry logic for failed operations (should be thread-safe)
	retryer Retryer
}

// NewClient creates a new AWS Secrets Manager client with the provided options.
// The context is used for AWS configuration loading and should not be nil.
//
// Example usage:
//
//	ctx := context.Background()
//	client, err := NewClient(ctx,
//	    WithLogger(slog.Default()),
//	    WithCache(myCache),
//	)
func NewClient(ctx context.Context, opts ...Option) (*Client, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	// Load AWS default configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create the AWS Secrets Manager client
	api := secretsmanager.NewFromConfig(cfg)

	// Apply default options
	options := defaultOptions()

	// Apply user-provided options
	applyOptions(options, opts)

	// Create the client
	client := &Client{
		api:     api,
		logger:  options.logger,
		cache:   options.cache,
		retryer: options.retryer,
	}

	return client, nil
}

// NewClientWithConfig creates a new AWS Secrets Manager client with a custom AWS configuration.
// This is useful for testing with LocalStack or other custom AWS endpoints.
// The context and config parameters should not be nil.
//
// Example usage:
//
//	ctx := context.Background()
//	cfg := aws.Config{...}
//	client, err := NewClientWithConfig(ctx, cfg,
//	    WithLogger(slog.Default()),
//	    WithCache(myCache),
//	)
func NewClientWithConfig(ctx context.Context, cfg *aws.Config, opts ...Option) (*Client, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("config region cannot be empty")
	}

	// Create the AWS Secrets Manager client
	api := secretsmanager.NewFromConfig(*cfg)

	// Apply default options
	options := defaultOptions()

	// Apply user-provided options
	applyOptions(options, opts)

	// Create the client
	client := &Client{
		api:     api,
		logger:  options.logger,
		cache:   options.cache,
		retryer: options.retryer,
	}

	return client, nil
}

// NewClientWithLocalStack creates a new AWS Secrets Manager client configured for LocalStack.
// This is a convenience function for integration testing.
func NewClientWithLocalStack(ctx context.Context, endpointURL string, opts ...Option) (*Client, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if endpointURL == "" {
		return nil, fmt.Errorf("endpoint URL cannot be empty")
	}

	// Create config with LocalStack endpoint
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create Secrets Manager client with LocalStack endpoint
	api := secretsmanager.New(secretsmanager.Options{
		Region:           "us-east-1",
		Credentials:      cfg.Credentials,
		EndpointResolver: secretsmanager.EndpointResolverFromURL(endpointURL),
	})

	// Apply default options
	options := defaultOptions()

	// Apply user-provided options
	applyOptions(options, opts)

	// Create the client
	client := &Client{
		api:     api,
		logger:  options.logger,
		cache:   options.cache,
		retryer: options.retryer,
	}

	return client, nil
}

// NewClientWithCache creates a new AWS Secrets Manager client with built-in caching enabled.
// This is a convenience constructor that automatically configures an in-memory cache
// with the specified TTL and size limits.
//
// The cacheTTL parameter specifies how long cached values should be retained.
// The cacheSize parameter limits the number of cached entries (0 = unlimited).
// Other options can be provided via opts for logger and retryer configuration.
//
// Example usage:
//
//	ctx := context.Background()
//	client, err := NewClientWithCache(ctx, 5*time.Minute, 100,
//	    WithLogger(slog.Default()),
//	    WithCustomRetryer(myRetryer),
//	)
func NewClientWithCache(ctx context.Context, cacheTTL time.Duration, cacheSize int, opts ...Option) (*Client, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if cacheTTL <= 0 {
		return nil, fmt.Errorf("cache TTL must be positive")
	}

	// Create the cache
	cache := NewInMemoryCache(cacheTTL, cacheSize)

	// Add the cache option to the provided options
	opts = append(opts, WithCache(cache))

	// Create client with cache enabled
	return NewClient(ctx, opts...)
}

// NewClientWithCacheAndConfig creates a new AWS Secrets Manager client with built-in caching
// enabled and a custom AWS configuration. This is useful for testing with LocalStack.
//
// The cacheTTL parameter specifies how long cached values should be retained.
// The cacheSize parameter limits the number of cached entries (0 = unlimited).
// Other options can be provided via opts for logger and retryer configuration.
func NewClientWithCacheAndConfig(
	ctx context.Context,
	cfg *aws.Config,
	cacheTTL time.Duration,
	cacheSize int,
	opts ...Option,
) (*Client, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if cacheTTL <= 0 {
		return nil, fmt.Errorf("cache TTL must be positive")
	}

	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("config region cannot be empty")
	}

	// Create the cache
	cache := NewInMemoryCache(cacheTTL, cacheSize)

	// Add the cache option to the provided options
	opts = append(opts, WithCache(cache))

	// Create client with cache enabled
	return NewClientWithConfig(ctx, cfg, opts...)
}

// handleError processes errors from AWS SDK operations, providing consistent
// error handling and wrapping with operational context.
//
// It preserves custom package errors (ErrSecretNotFound, ErrSecretEmpty, ErrAccessDenied)
// while wrapping other errors with operation context for better debugging.
func (c *Client) handleError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Check if this is one of our custom errors - preserve them as-is
	if errors.Is(err, ErrSecretNotFound) ||
		errors.Is(err, ErrSecretEmpty) ||
		errors.Is(err, ErrAccessDenied) {
		return err
	}

	// For smithy API errors, provide structured error information
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return fmt.Errorf("%s operation failed: %s: %s",
			operation, apiErr.ErrorCode(), apiErr.ErrorMessage())
	}

	// For all other errors, wrap with operation context
	return fmt.Errorf("%s operation failed: %w", operation, err)
}

// GetSecret retrieves the value of a secret from AWS Secrets Manager.
//
// The method accepts a context as the first parameter for timeout and cancellation control.
// It returns the secret value as a string, or an error if the operation fails.
//
// The method supports both string and binary secrets:
// - For string secrets, it returns the SecretString value
// - For binary secrets, it returns the SecretBinary value as a string
//
// Example usage:
//
//	ctx := context.Background()
//	client, err := NewClient(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	value, err := client.GetSecret(ctx, "my-secret")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(value)
func (c *Client) GetSecret(ctx context.Context, secretName string) (string, error) {
	// Validate inputs
	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}
	if secretName == "" {
		return "", fmt.Errorf("secret name cannot be empty")
	}

	// Log the operation start (without sensitive data)
	if c.logger != nil {
		c.logger.InfoContext(ctx, "retrieving secret",
			"secret_name", secretName)
	}

	// Prepare the request parameters
	input := &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	}

	// Make the API call
	output, err := c.api.GetSecretValue(ctx, input)
	if err != nil {
		// Handle specific AWS errors
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case ResourceNotFoundException:
				return "", c.handleError(ErrSecretNotFound, "GetSecret")
			case AccessDeniedException:
				return "", c.handleError(ErrAccessDenied, "GetSecret")
			}
		}

		// Log error and handle generic errors
		if c.logger != nil {
			c.logger.ErrorContext(ctx, "failed to retrieve secret",
				"secret_name", secretName,
				"error", err)
		}
		return "", c.handleError(err, "GetSecret")
	}

	// Extract the secret value
	var secretValue string
	switch {
	case output.SecretString != nil:
		secretValue = *output.SecretString
	case output.SecretBinary != nil:
		secretValue = string(output.SecretBinary)
	default:
		// Secret exists but has no value
		return "", c.handleError(ErrSecretEmpty, "GetSecret")
	}

	// Log successful retrieval (without exposing the secret value)
	if c.logger != nil {
		c.logger.InfoContext(ctx, "secret retrieved successfully",
			"secret_name", secretName)
	}

	return secretValue, nil
}

// PutSecret updates the value of an existing secret in AWS Secrets Manager.
//
// The method accepts a context as the first parameter for timeout and cancellation control.
// It updates the secret with the provided value. If the secret doesn't exist,
// this operation will fail.
//
// The method supports both string and binary secrets:
// - For string secrets, the value is passed as SecretString
// - For binary secrets, the value is passed as SecretBinary
//
// Example usage:
//
//	ctx := context.Background()
//	client, err := NewClient(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	err = client.PutSecret(ctx, "my-secret", "new-secret-value")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) PutSecret(ctx context.Context, secretName, secretValue string) error {
	// Validate inputs
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
	if secretName == "" {
		return fmt.Errorf("secret name cannot be empty")
	}
	if secretValue == "" {
		return fmt.Errorf("secret value cannot be empty")
	}

	// Log the operation start (without sensitive data)
	if c.logger != nil {
		c.logger.InfoContext(ctx, "updating secret",
			"secret_name", secretName)
	}

	// Prepare the request parameters
	input := &secretsmanager.PutSecretValueInput{
		SecretId:     &secretName,
		SecretString: &secretValue,
	}

	// Make the API call
	_, err := c.api.PutSecretValue(ctx, input)
	if err != nil {
		// Handle specific AWS errors
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case ResourceNotFoundException:
				return c.handleError(ErrSecretNotFound, "PutSecret")
			case AccessDeniedException:
				return c.handleError(ErrAccessDenied, "PutSecret")
			}
		}

		// Log error and handle generic errors
		if c.logger != nil {
			c.logger.ErrorContext(ctx, "failed to update secret",
				"secret_name", secretName,
				"error", err)
		}
		return c.handleError(err, "PutSecret")
	}

	// Log successful update (without exposing the secret value)
	if c.logger != nil {
		c.logger.InfoContext(ctx, "secret updated successfully",
			"secret_name", secretName)
	}

	return nil
}

// CreateSecret creates a new secret in AWS Secrets Manager.
//
// The method accepts a context as the first parameter for timeout and cancellation control.
// It creates a new secret with the provided name and value. If a KMS key ID is provided,
// the secret will be encrypted using that key.
//
// Example usage:
//
//	ctx := context.Background()
//	client, err := NewClient(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	err = client.CreateSecret(ctx, "my-secret", "secret-value", "alias/aws/secretsmanager")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) CreateSecret(ctx context.Context, secretName, secretValue, kmsKeyID string) error {
	// Validate inputs
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
	if secretName == "" {
		return fmt.Errorf("secret name cannot be empty")
	}
	if secretValue == "" {
		return fmt.Errorf("secret value cannot be empty")
	}

	// Log the operation start (without sensitive data)
	if c.logger != nil {
		c.logger.InfoContext(ctx, "creating secret",
			"secret_name", secretName)
	}

	// Prepare the request parameters
	input := &secretsmanager.CreateSecretInput{
		Name:         &secretName,
		SecretString: &secretValue,
	}

	// Add KMS key if provided
	if kmsKeyID != "" {
		input.KmsKeyId = &kmsKeyID
	}

	// Make the API call
	_, err := c.api.CreateSecret(ctx, input)
	if err != nil {
		// Handle specific AWS errors
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == AccessDeniedException {
				return c.handleError(ErrAccessDenied, "CreateSecret")
			}
		}

		// Log error and handle generic errors
		if c.logger != nil {
			c.logger.ErrorContext(ctx, "failed to create secret",
				"secret_name", secretName,
				"error", err)
		}
		return c.handleError(err, "CreateSecret")
	}

	// Log successful creation (without exposing the secret value)
	if c.logger != nil {
		c.logger.InfoContext(ctx, "secret created successfully",
			"secret_name", secretName)
	}

	return nil
}

// DescribeSecret retrieves metadata about a secret from AWS Secrets Manager.
//
// The method accepts a context as the first parameter for timeout and cancellation control.
// It returns secret metadata without exposing the secret value itself.
//
// The metadata includes information such as:
// - Secret name and ARN
// - Description
// - Creation and last modified dates
// - Tags
// - KMS key information
//
// Example usage:
//
//	ctx := context.Background()
//	client, err := NewClient(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	metadata, err := client.DescribeSecret(ctx, "my-secret")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Secret: %s, Created: %s\n", *metadata.Name, metadata.CreatedDate)
func (c *Client) DescribeSecret(ctx context.Context, secretName string) (*secretsmanager.DescribeSecretOutput, error) {
	// Validate inputs
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if secretName == "" {
		return nil, fmt.Errorf("secret name cannot be empty")
	}

	// Log the operation start
	if c.logger != nil {
		c.logger.InfoContext(ctx, "describing secret",
			"secret_name", secretName)
	}

	// Prepare the request parameters
	input := &secretsmanager.DescribeSecretInput{
		SecretId: &secretName,
	}

	// Make the API call
	output, err := c.api.DescribeSecret(ctx, input)
	if err != nil {
		// Handle specific AWS errors
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case ResourceNotFoundException:
				return nil, c.handleError(ErrSecretNotFound, "DescribeSecret")
			case AccessDeniedException:
				return nil, c.handleError(ErrAccessDenied, "DescribeSecret")
			}
		}

		// Log error and handle generic errors
		if c.logger != nil {
			c.logger.ErrorContext(ctx, "failed to describe secret",
				"secret_name", secretName,
				"error", err)
		}
		return nil, c.handleError(err, "DescribeSecret")
	}

	// Log successful description
	if c.logger != nil {
		c.logger.InfoContext(ctx, "secret described successfully",
			"secret_name", secretName)
	}

	return output, nil
}

// GetSecretCached retrieves the value of a secret from AWS Secrets Manager with caching.
// This method first checks the cache for the secret value. If found and not expired,
// it returns the cached value. Otherwise, it fetches the value from AWS Secrets Manager
// and caches it for future requests.
//
// The method accepts a context as the first parameter for timeout and cancellation control.
// It returns the secret value as a string, or an error if the operation fails.
//
// This method provides better performance for frequently accessed secrets by avoiding
// repeated AWS API calls, while still ensuring data freshness through TTL-based expiration.
//
// Example usage:
//
//	ctx := context.Background()
//	client, err := NewClientWithCache(ctx, 5*time.Minute, 100)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	value, err := client.GetSecretCached(ctx, "my-secret")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(value) // Value is cached for 5 minutes
func (c *Client) GetSecretCached(ctx context.Context, secretName string) (string, error) {
	// Validate inputs
	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}
	if secretName == "" {
		return "", fmt.Errorf("secret name cannot be empty")
	}

	// Check if caching is enabled
	if c.cache == nil {
		// Fall back to regular GetSecret if no cache is configured
		return c.GetSecret(ctx, secretName)
	}

	// Check cache first
	if cachedValue, found := c.cache.Get(secretName); found {
		if strValue, ok := cachedValue.(string); ok {
			// Log cache hit (without exposing the secret value)
			if c.logger != nil {
				c.logger.InfoContext(ctx, "cache hit for secret",
					"secret_name", secretName)
			}
			return strValue, nil
		}
	}

	// Cache miss or invalid cached value - fetch from AWS
	value, err := c.GetSecret(ctx, secretName)
	if err != nil {
		return "", err
	}

	// Cache the successful result
	c.cache.Set(secretName, value, 0) // Use default TTL

	// Log cache storage (without exposing the secret value)
	if c.logger != nil {
		c.logger.InfoContext(ctx, "secret cached successfully",
			"secret_name", secretName)
	}

	return value, nil
}

// InvalidateCache removes a specific secret from the cache.
// This forces the next GetSecretCached call to fetch the value from AWS Secrets Manager.
//
// This method is useful when you know that a secret has been updated and you want
// to ensure the next request gets the latest value.
//
// Example usage:
//
//	ctx := context.Background()
//	client, err := NewClientWithCache(ctx, 5*time.Minute, 100)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Invalidate a specific secret
//	client.InvalidateCache("my-secret")
//
//	// Next call will fetch from AWS
//	value, err := client.GetSecretCached(ctx, "my-secret")
func (c *Client) InvalidateCache(secretName string) {
	if c.cache == nil || secretName == "" {
		return
	}

	// Invalidate the cache entry
	if inMemoryCache, ok := c.cache.(*InMemoryCache); ok {
		inMemoryCache.Delete(secretName)
	}

	// Log cache invalidation
	if c.logger != nil {
		c.logger.InfoContext(context.Background(), "cache invalidated for secret",
			"secret_name", secretName)
	}
}

// ClearCache removes all cached secrets.
// This forces all subsequent GetSecretCached calls to fetch values from AWS Secrets Manager.
//
// This method is useful for bulk cache invalidation scenarios, such as when
// multiple secrets have been updated or when you want to ensure all data is fresh.
//
// Example usage:
//
//	ctx := context.Background()
//	client, err := NewClientWithCache(ctx, 5*time.Minute, 100)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Clear all cached values
//	client.ClearCache()
//
//	// Next calls will fetch from AWS
//	value1, err := client.GetSecretCached(ctx, "secret1")
//	value2, err := client.GetSecretCached(ctx, "secret2")
func (c *Client) ClearCache() {
	if c.cache == nil {
		return
	}

	// Clear the entire cache
	if inMemoryCache, ok := c.cache.(*InMemoryCache); ok {
		inMemoryCache.Clear()
	}

	// Log cache clearing
	if c.logger != nil {
		c.logger.InfoContext(context.Background(), "entire cache cleared")
	}
}

// GetCacheSize returns the current number of entries in the cache.
// This includes only non-expired entries. Returns 0 if caching is disabled.
func (c *Client) GetCacheSize() int {
	if c.cache == nil {
		return 0
	}

	if inMemoryCache, ok := c.cache.(*InMemoryCache); ok {
		return inMemoryCache.Size()
	}

	return 0
}
