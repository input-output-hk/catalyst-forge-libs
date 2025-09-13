package secrets

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/smithy-go"
)

// fakeAPI is a minimal in-memory implementation of ManagerAPI for examples.
type fakeAPI struct {
	store map[string]*secretsmanager.GetSecretValueOutput
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{store: make(map[string]*secretsmanager.GetSecretValueOutput)}
}

func (f *fakeAPI) GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	if in == nil || in.SecretId == nil || *in.SecretId == "" {
		return nil, fmt.Errorf("invalid secret id")
	}
	out, ok := f.store[*in.SecretId]
	if !ok {
		return nil, &fakeAPIError{code: ResourceNotFoundException, message: "not found"}
	}
	return out, nil
}

func (f *fakeAPI) PutSecretValue(ctx context.Context, in *secretsmanager.PutSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	if in == nil || in.SecretId == nil || *in.SecretId == "" {
		return nil, fmt.Errorf("invalid secret id")
	}
	if in.SecretString == nil || *in.SecretString == "" {
		return nil, fmt.Errorf("invalid secret value")
	}
	f.store[*in.SecretId] = &secretsmanager.GetSecretValueOutput{SecretString: in.SecretString}
	return &secretsmanager.PutSecretValueOutput{}, nil
}

func (f *fakeAPI) CreateSecret(ctx context.Context, in *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	if in == nil || in.Name == nil || *in.Name == "" {
		return nil, fmt.Errorf("invalid secret name")
	}
	if in.SecretString == nil || *in.SecretString == "" {
		return nil, fmt.Errorf("invalid secret value")
	}
	if _, exists := f.store[*in.Name]; exists {
		return nil, fmt.Errorf("already exists")
	}
	f.store[*in.Name] = &secretsmanager.GetSecretValueOutput{SecretString: in.SecretString}
	return &secretsmanager.CreateSecretOutput{ARN: aws.String("arn:aws:secretsmanager:region:acct:secret:" + *in.Name)}, nil
}

func (f *fakeAPI) DescribeSecret(ctx context.Context, in *secretsmanager.DescribeSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DescribeSecretOutput, error) {
	if in == nil || in.SecretId == nil || *in.SecretId == "" {
		return nil, fmt.Errorf("invalid secret id")
	}
	if _, ok := f.store[*in.SecretId]; !ok {
		return nil, &fakeAPIError{code: ResourceNotFoundException, message: "not found"}
	}
	return &secretsmanager.DescribeSecretOutput{Name: in.SecretId, ARN: aws.String("arn:aws:secretsmanager:region:acct:secret:" + *in.SecretId)}, nil
}

// fakeAPIError implements smithy.APIError to exercise error mapping in examples.
type fakeAPIError struct{ code, message string }

func (e *fakeAPIError) Error() string                 { return e.code + ": " + e.message }
func (e *fakeAPIError) ErrorCode() string             { return e.code }
func (e *fakeAPIError) ErrorMessage() string          { return e.message }
func (e *fakeAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

// Example_basic demonstrates basic client usage with a custom AWS config and logger.
func Example_basic() {
	ctx := context.Background()

	// Build a custom config (region is required for NewClientWithConfig)
	cfg := aws.Config{Region: "us-east-1"}

	client, err := NewClientWithConfig(ctx, &cfg, WithLogger(slog.Default()))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Swap underlying API with a fake to keep the example deterministic.
	client.api = newFakeAPI()

	// Create and then get a secret
	_ = client.CreateSecret(ctx, "example/secret", "s3cr3t", "")
	val, err := client.GetSecret(ctx, "example/secret")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("value:", val)
	// Output:
	// value: s3cr3t
}

// Example_caching demonstrates using the built-in in-memory cache.
func Example_caching() {
	ctx := context.Background()
	client, err := NewClientWithCache(ctx, 2*time.Minute, 100)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	client.api = newFakeAPI()

	// Seed a value and read twice; second read will be a cache hit
	_ = client.CreateSecret(ctx, "cached/secret", "cached-value", "")
	_, _ = client.GetSecretCached(ctx, "cached/secret")
	val, _ := client.GetSecretCached(ctx, "cached/secret")
	fmt.Println("cached:", val)
	// Output:
	// cached: cached-value
}

// Example_customRetry demonstrates configuring a custom retryer.
func Example_customRetry() {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}
	client, err := NewClientWithConfig(ctx, &cfg, WithCustomRetryer(&CustomRetryer{maxAttempts: 5, baseDelay: 50 * time.Millisecond, maxDelay: 500 * time.Millisecond}))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	client.api = newFakeAPI()

	_ = client.CreateSecret(ctx, "retry/secret", "ok", "")
	val, _ := client.GetSecret(ctx, "retry/secret")
	fmt.Println("value:", val)
	// Output:
	// value: ok
}

// Example_errorHandling demonstrates typed error handling without exposing sensitive data.
func Example_errorHandling() {
	ctx := context.Background()
	cfg := aws.Config{Region: "us-east-1"}
	client, err := NewClientWithConfig(ctx, &cfg)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	client.api = newFakeAPI()

	// Attempt to get a non-existent secret
	_, err = client.GetSecret(ctx, "missing/secret")
	if err != nil {
		// Map smithy error to typed error via handleError
		if err == ErrSecretNotFound {
			fmt.Println("not found")
		} else {
			fmt.Println("other error")
		}
	}
	// Output:
	// not found
}
