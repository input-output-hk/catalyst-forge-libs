// Package secrets provides tests for functional options configuration
// of the AWS Secrets Manager client.
package secrets

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithLogger(t *testing.T) {
	tests := []struct {
		name     string
		logger   *slog.Logger
		validate func(t *testing.T, opts *clientOptions)
	}{
		{
			name:   "with custom logger",
			logger: slog.New(slog.NewTextHandler(nil, nil)),
			validate: func(t *testing.T, opts *clientOptions) {
				assert.NotNil(t, opts.logger)
				// Test that the logger is enabled for info level
				assert.True(t, opts.logger.Enabled(context.TODO(), slog.LevelInfo))
			},
		},
		{
			name:   "with nil logger",
			logger: nil,
			validate: func(t *testing.T, opts *clientOptions) {
				assert.Nil(t, opts.logger)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &clientOptions{}
			option := WithLogger(tt.logger)
			option(opts)
			tt.validate(t, opts)
		})
	}
}

func TestWithCache(t *testing.T) {
	tests := []struct {
		name     string
		cache    Cache
		validate func(t *testing.T, opts *clientOptions)
	}{
		{
			name:  "with custom cache",
			cache: &mockCache{ttl: time.Minute},
			validate: func(t *testing.T, opts *clientOptions) {
				assert.NotNil(t, opts.cache)
				assert.Equal(t, time.Minute, opts.cache.(*mockCache).ttl)
			},
		},
		{
			name:  "with nil cache",
			cache: nil,
			validate: func(t *testing.T, opts *clientOptions) {
				assert.Nil(t, opts.cache)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &clientOptions{}
			option := WithCache(tt.cache)
			option(opts)
			tt.validate(t, opts)
		})
	}
}

func TestWithCustomRetryer(t *testing.T) {
	tests := []struct {
		name     string
		retryer  Retryer
		validate func(t *testing.T, opts *clientOptions)
	}{
		{
			name:    "with custom retryer",
			retryer: &CustomRetryer{maxAttempts: 5},
			validate: func(t *testing.T, opts *clientOptions) {
				assert.NotNil(t, opts.retryer)
				assert.Equal(t, 5, opts.retryer.(*CustomRetryer).maxAttempts)
			},
		},
		{
			name:    "with nil retryer",
			retryer: nil,
			validate: func(t *testing.T, opts *clientOptions) {
				assert.Nil(t, opts.retryer)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &clientOptions{}
			option := WithCustomRetryer(tt.retryer)
			option(opts)
			tt.validate(t, opts)
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()

	assert.Nil(t, opts.logger)
	assert.Nil(t, opts.cache)
	assert.Nil(t, opts.retryer)
}

func TestApplyOptions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	cache := &mockCache{ttl: time.Hour}
	retryer := &CustomRetryer{maxAttempts: 3}

	opts := &clientOptions{}
	options := []Option{
		WithLogger(logger),
		WithCache(cache),
		WithCustomRetryer(retryer),
	}

	applyOptions(opts, options)

	assert.Equal(t, logger, opts.logger)
	assert.Equal(t, cache, opts.cache)
	assert.Equal(t, retryer, opts.retryer)
}

func TestOptionComposition(t *testing.T) {
	// Test that multiple options can be composed together
	logger := slog.New(slog.NewTextHandler(nil, nil))
	cache := &mockCache{ttl: 30 * time.Minute}

	opts := &clientOptions{}
	applyOptions(opts, []Option{
		WithLogger(logger),
		WithCache(cache),
		// Skip retryer to test partial configuration
	})

	assert.Equal(t, logger, opts.logger)
	assert.Equal(t, cache, opts.cache)
	assert.Nil(t, opts.retryer)
}

// CustomRetryer tests

func TestCustomRetryer_MaxAttempts(t *testing.T) {
	tests := []struct {
		name        string
		maxAttempts int
		expected    int
	}{
		{
			name:        "zero max attempts",
			maxAttempts: 0,
			expected:    0,
		},
		{
			name:        "positive max attempts",
			maxAttempts: 5,
			expected:    5,
		},
		{
			name:        "large max attempts",
			maxAttempts: 100,
			expected:    100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryer := &CustomRetryer{maxAttempts: tt.maxAttempts}
			assert.Equal(t, tt.expected, retryer.MaxAttempts())
		})
	}
}

func TestCustomRetryer_RetryDelay(t *testing.T) {
	tests := []struct {
		name        string
		maxAttempts int
		baseDelay   time.Duration
		maxDelay    time.Duration
		attempt     int
		checkDelay  func(t *testing.T, delay time.Duration, err error)
	}{
		{
			name:        "first attempt delay",
			maxAttempts: 5,
			baseDelay:   time.Second,
			maxDelay:    30 * time.Second,
			attempt:     1,
			checkDelay: func(t *testing.T, delay time.Duration, err error) {
				require.NoError(t, err)
				// For attempt 1: baseDelay * 2^(1-1) = 1s * 1 = 1s
				// With jitter (±25%), should be between 0.75s and 1.25s
				assert.True(t, delay >= 750*time.Millisecond, "delay should be at least 0.75s with jitter")
				assert.True(t, delay <= 1250*time.Millisecond, "delay should be at most 1.25s with jitter")
			},
		},
		{
			name:        "exponential backoff",
			maxAttempts: 5,
			baseDelay:   time.Second,
			maxDelay:    30 * time.Second,
			attempt:     3,
			checkDelay: func(t *testing.T, delay time.Duration, err error) {
				require.NoError(t, err)
				// For attempt 3: baseDelay * 2^(3-1) = 1s * 4 = 4s
				// With jitter (±25%), should be between 3s and 5s
				assert.True(t, delay >= 3*time.Second, "delay should be at least 3s with jitter")
				assert.True(t, delay <= 5*time.Second, "delay should be at most 5s with jitter")
			},
		},
		{
			name:        "max delay cap",
			maxAttempts: 10,
			baseDelay:   time.Second,
			maxDelay:    10 * time.Second,
			attempt:     6,
			checkDelay: func(t *testing.T, delay time.Duration, err error) {
				require.NoError(t, err)
				// For attempt 6: baseDelay * 2^(6-1) = 1s * 32 = 32s, but capped at 10s
				// With jitter (±25%), should be between 7.5s and 12.5s, but capped at 10s
				assert.True(t, delay >= 7500*time.Millisecond, "delay should be at least 7.5s with jitter")
				assert.True(t, delay <= 10*time.Second, "delay should be at most 10s (capped)")
			},
		},
		{
			name:        "jitter variation",
			maxAttempts: 5,
			baseDelay:   time.Second,
			maxDelay:    30 * time.Second,
			attempt:     2,
			checkDelay: func(t *testing.T, delay time.Duration, err error) {
				require.NoError(t, err)
				// For attempt 2: baseDelay * 2^(2-1) = 1s * 2 = 2s
				// With jitter, should be between 1.5s and 2.5s
				assert.True(t, delay >= 1500*time.Millisecond, "delay should be at least 1.5s with jitter")
				assert.True(t, delay <= 2500*time.Millisecond, "delay should be at most 2.5s with jitter")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryer := &CustomRetryer{
				maxAttempts: tt.maxAttempts,
				baseDelay:   tt.baseDelay,
				maxDelay:    tt.maxDelay,
			}
			delay, err := retryer.RetryDelay(tt.attempt, nil)
			tt.checkDelay(t, delay, err)
		})
	}
}

func TestCustomRetryer_IsErrorRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "throttling exception",
			err:      &mockAWSError{code: "ThrottlingException"},
			expected: true,
		},
		{
			name:     "throughput exceeded exception",
			err:      &mockAWSError{code: "ProvisionedThroughputExceededException"},
			expected: true,
		},
		{
			name:     "request limit exceeded",
			err:      &mockAWSError{code: "RequestLimitExceeded"},
			expected: true,
		},
		{
			name:     "too many requests exception",
			err:      &mockAWSError{code: "TooManyRequestsException"},
			expected: true,
		},
		{
			name:     "access denied exception",
			err:      &mockAWSError{code: "AccessDeniedException"},
			expected: false,
		},
		{
			name:     "unauthorized operation",
			err:      &mockAWSError{code: "UnauthorizedOperation"},
			expected: false,
		},
		{
			name:     "invalid parameter exception",
			err:      &mockAWSError{code: "InvalidParameterException"},
			expected: false,
		},
		{
			name:     "validation exception",
			err:      &mockAWSError{code: "ValidationException"},
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: false,
		},
		{
			name:     "unknown AWS error code",
			err:      &mockAWSError{code: "UnknownErrorCode"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryer := &CustomRetryer{}
			result := retryer.IsErrorRetryable(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockAWSError implements smithy.APIError for testing
type mockAWSError struct {
	code string
}

func (e *mockAWSError) Error() string {
	return "mock AWS error: " + e.code
}

func (e *mockAWSError) ErrorCode() string {
	return e.code
}

func (e *mockAWSError) ErrorMessage() string {
	return "mock error message"
}

func (e *mockAWSError) ErrorFault() smithy.ErrorFault {
	return smithy.FaultUnknown
}
