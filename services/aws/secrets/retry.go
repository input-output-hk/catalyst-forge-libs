// Package secrets provides retry logic for AWS Secrets Manager operations.
package secrets

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// createCustomRetryer creates a custom retry configuration for AWS Secrets Manager operations.
// It configures maximum attempts, backoff delay, and specific error codes that should trigger retries.
//
// Returns a retryer configured with:
// - Maximum 10 attempts (including initial attempt)
// - Maximum backoff delay of 30 seconds
// - Exponential backoff starting with 100ms base delay
// - Retries on ThrottlingException and ProvisionedThroughputExceededException
//
//nolint:ireturn // AWS SDK v2 uses interface for flexibility and testability
func createCustomRetryer() aws.Retryer {
	return &CustomRetryer{
		maxAttempts: 10,
		baseDelay:   100 * time.Millisecond, // Base delay for exponential backoff
		maxDelay:    30 * time.Second,
	}
}
