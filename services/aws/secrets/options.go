// Package secrets provides functional options for configuring the AWS Secrets Manager client.
package secrets

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/aws/smithy-go"
)

// Cache defines the interface for caching secret values.
// Implementations should be thread-safe for concurrent access.
type Cache interface {
	// Get retrieves a value from the cache by key.
	// Returns the value and true if found, nil and false if not found.
	Get(key string) (any, bool)

	// Set stores a value in the cache with the specified key and TTL.
	Set(key string, value any, ttl time.Duration)
}

// Retryer defines the interface for retry logic in AWS SDK operations.
// This interface matches the AWS SDK v2 aws.Retryer interface.
type Retryer interface {
	// MaxAttempts returns the maximum number of retry attempts.
	MaxAttempts() int

	// RetryDelay returns the delay duration for the given attempt number and error.
	RetryDelay(attempt int, err error) (time.Duration, error)

	// IsErrorRetryable determines if the given error should be retried.
	IsErrorRetryable(error) bool
}

// CustomRetryer implements the Retryer interface with configurable retry behavior.
// It provides exponential backoff with jitter and checks for specific AWS error codes.
//
// Thread Safety: This struct is thread-safe for concurrent use.
// All fields are immutable configuration values that are set at creation time
// and never modified. The random number generation used for jitter is also thread-safe.
type CustomRetryer struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

// MaxAttempts returns the maximum number of retry attempts.
func (r *CustomRetryer) MaxAttempts() int {
	return r.maxAttempts
}

// RetryDelay returns the delay duration for the given attempt number and error,
// implementing exponential backoff with jitter to prevent thundering herd problems.
func (r *CustomRetryer) RetryDelay(attempt int, err error) (time.Duration, error) {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	baseDelay := time.Duration(math.Pow(2, float64(attempt-1))) * r.baseDelay

	// Add jitter (±25%) to prevent thundering herd
	// Use a random value between -25% and +25% of the base delay
	jitterRange := int64(float64(baseDelay) * 0.25)
	if jitterRange > 0 {
		jitter := time.Duration(rand.Int63n(2*jitterRange) - jitterRange)
		baseDelay += jitter
	}

	// Cap at maximum delay (after adding jitter)
	if baseDelay > r.maxDelay {
		baseDelay = r.maxDelay
	}

	// Ensure delay is not negative
	if baseDelay < 0 {
		baseDelay = 0
	}

	return baseDelay, nil
}

// IsErrorRetryable determines if the given error should be retried.
// It checks for specific AWS error codes that indicate transient failures.
func (r *CustomRetryer) IsErrorRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for smithy API errors (AWS SDK v2 error type)
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		errorCode := apiErr.ErrorCode()

		// Retry on throttling and throughput exceeded errors
		switch errorCode {
		case "ThrottlingException",
			"ProvisionedThroughputExceededException",
			"RequestLimitExceeded",
			"TooManyRequestsException":
			return true
		}

		// Don't retry on permanent errors
		switch errorCode {
		case "AccessDeniedException",
			"UnauthorizedOperation",
			"InvalidParameterException",
			"ValidationException":
			return false
		}
	}

	// Check for network-related errors that are typically transient
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return false // Don't retry on context timeouts/cancellations
	}

	// For other errors, use a conservative approach - don't retry by default
	// This prevents infinite retry loops on permanent failures
	return false
}

// GetRetryToken attempts to deduct the retry cost from the retry token pool.
// For our simple implementation, we always allow retries and return a no-op release function.
func (r *CustomRetryer) GetRetryToken(ctx context.Context, opErr error) (releaseToken func(error) error, err error) {
	return func(error) error { return nil }, nil
}

// GetInitialToken returns the initial attempt token that can increment the retry token pool.
// For our simple implementation, we return a no-op release function.
func (r *CustomRetryer) GetInitialToken() (releaseToken func(error) error) {
	return func(error) error { return nil }
}

// clientOptions holds configuration options for the AWS Secrets Manager client.
type clientOptions struct {
	logger  *slog.Logger
	cache   Cache
	retryer Retryer
}

// Option is a functional option for configuring the Client.
type Option func(*clientOptions)

// WithLogger configures the client with a custom logger.
// If logger is nil, logging will be disabled.
func WithLogger(logger *slog.Logger) Option {
	return func(opts *clientOptions) {
		opts.logger = logger
	}
}

// WithCache configures the client with a cache implementation.
// If cache is nil, caching will be disabled.
func WithCache(cache Cache) Option {
	return func(opts *clientOptions) {
		opts.cache = cache
	}
}

// WithCustomRetryer configures the client with a custom retryer.
// If retryer is nil, default AWS SDK retry behavior will be used.
func WithCustomRetryer(retryer Retryer) Option {
	return func(opts *clientOptions) {
		opts.retryer = retryer
	}
}

// defaultOptions returns the default configuration options.
func defaultOptions() *clientOptions {
	return &clientOptions{
		logger:  nil, // No default logger
		cache:   nil, // No default cache
		retryer: nil, // Use AWS SDK defaults
	}
}

// applyOptions applies the given options to the client options.
func applyOptions(opts *clientOptions, options []Option) {
	for _, option := range options {
		option(opts)
	}
}
