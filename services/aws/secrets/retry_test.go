// Package secrets provides tests for retry logic in AWS Secrets Manager operations.
package secrets

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCustomRetryer(t *testing.T) {
	tests := []struct {
		name string
		want func(t *testing.T, r aws.Retryer)
	}{
		{
			name: "should configure max attempts",
			want: func(t *testing.T, r aws.Retryer) {
				assert.Equal(t, 10, r.MaxAttempts())
			},
		},
		{
			name: "should configure max backoff delay",
			want: func(t *testing.T, r aws.Retryer) {
				// Test that backoff delay is properly capped at 30 seconds
				// We test with a high attempt number to ensure capping works
				delay, err := r.RetryDelay(10, nil)
				require.NoError(t, err)
				assert.LessOrEqual(t, delay, 30*time.Second)
			},
		},
		{
			name: "should retry on throttling exception",
			want: func(t *testing.T, r aws.Retryer) {
				// Create a mock error that simulates AWS ThrottlingException
				throttlingErr := &mockAWSError{code: "ThrottlingException"}
				assert.True(t, r.IsErrorRetryable(throttlingErr))
			},
		},
		{
			name: "should retry on throughput exceeded exception",
			want: func(t *testing.T, r aws.Retryer) {
				// Create a mock error that simulates AWS ProvisionedThroughputExceededException
				throughputErr := &mockAWSError{code: "ProvisionedThroughputExceededException"}
				assert.True(t, r.IsErrorRetryable(throughputErr))
			},
		},
		{
			name: "should not retry on non-retryable errors",
			want: func(t *testing.T, r aws.Retryer) {
				// Create a mock error that simulates a non-retryable AWS error
				accessDeniedErr := &mockAWSError{code: "AccessDeniedException"}
				assert.False(t, r.IsErrorRetryable(accessDeniedErr))
			},
		},
		{
			name: "should implement exponential backoff",
			want: func(t *testing.T, r aws.Retryer) {
				// Test that delays increase exponentially
				delay1, err := r.RetryDelay(1, nil)
				require.NoError(t, err)
				delay2, err := r.RetryDelay(2, nil)
				require.NoError(t, err)
				delay3, err := r.RetryDelay(3, nil)
				require.NoError(t, err)

				// Each subsequent delay should be larger (exponential backoff)
				assert.Greater(t, delay2, delay1)
				assert.Greater(t, delay3, delay2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryer := createCustomRetryer()
			require.NotNil(t, retryer)
			tt.want(t, retryer)
		})
	}
}
