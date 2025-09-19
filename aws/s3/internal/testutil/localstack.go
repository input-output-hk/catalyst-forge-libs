// Package testutil provides LocalStack integration test utilities.
package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/wait"
)

// LocalStackContainer wraps LocalStack container for testing.
type LocalStackContainer struct {
	container *localstack.LocalStackContainer
	endpoint  string
	region    string
}

// NewLocalStackContainer creates and starts a new LocalStack container.
// It automatically sets up S3 service and returns a container ready for testing.
func NewLocalStackContainer(ctx context.Context, t *testing.T) (*LocalStackContainer, error) {
	t.Helper()

	// Create LocalStack container with S3 service enabled
	container, err := localstack.Run(ctx,
		"localstack/localstack:latest",
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/_localstack/health").
				WithPort("4566").
				WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start LocalStack container: %w", err)
	}

	// Get the S3 endpoint
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "4566")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	endpoint := fmt.Sprintf("http://%s:%s", host, port.Port())

	return &LocalStackContainer{
		container: container,
		endpoint:  endpoint,
		region:    "us-east-1",
	}, nil
}

// GetS3Client returns an S3 client configured to use LocalStack.
func (c *LocalStackContainer) GetS3Client(ctx context.Context) (*s3.Client, error) {
	// Load AWS config for LocalStack
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(c.region),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(
			func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     "test",
					SecretAccessKey: "test",
				}, nil
			})),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create S3 client with path-style addressing and custom endpoint
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(c.endpoint)
	})

	return client, nil
}

// Endpoint returns the LocalStack endpoint URL.
func (c *LocalStackContainer) Endpoint() string {
	return c.endpoint
}

// Region returns the AWS region used by LocalStack.
func (c *LocalStackContainer) Region() string {
	return c.region
}

// Terminate stops and removes the LocalStack container.
func (c *LocalStackContainer) Terminate(ctx context.Context) error {
	if c.container != nil {
		if err := c.container.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate container: %w", err)
		}
	}
	return nil
}

// SetupLocalStackTest is a helper that sets up LocalStack for a test.
// It returns an S3 client and a cleanup function that should be deferred.
func SetupLocalStackTest(t *testing.T) (*s3.Client, func()) {
	t.Helper()

	// Skip if running in CI without Docker
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create LocalStack container
	container, err := NewLocalStackContainer(ctx, t)
	if err != nil {
		t.Fatalf("Failed to create LocalStack container: %v", err)
	}

	// Get S3 client
	client, err := container.GetS3Client(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("Failed to create S3 client: %v", err)
	}

	// Return client and cleanup function
	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate LocalStack container: %v", err)
		}
	}

	return client, cleanup
}

// CreateTestBucketInLocalStack creates a test bucket in LocalStack.
func CreateTestBucketInLocalStack(
	ctx context.Context, client *s3.Client, bucketName string,
) error {
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}
	return nil
}

// CleanupTestBucketInLocalStack deletes all objects and removes a test bucket.
func CleanupTestBucketInLocalStack(
	ctx context.Context, client *s3.Client, bucketName string,
) error {
	// List and delete all objects
	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	}

	for {
		listOutput, err := client.ListObjectsV2(ctx, listInput)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		if len(listOutput.Contents) == 0 {
			break
		}

		// Build delete request
		var objects []types.ObjectIdentifier
		for _, obj := range listOutput.Contents {
			objects = append(objects, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		deleteInput := &s3.DeleteObjectsInput{
			Bucket: aws.String(bucketName),
			Delete: &types.Delete{
				Objects: objects,
			},
		}

		if _, err := client.DeleteObjects(ctx, deleteInput); err != nil {
			return fmt.Errorf("failed to delete objects: %w", err)
		}

		if !aws.ToBool(listOutput.IsTruncated) {
			break
		}
		listInput.ContinuationToken = listOutput.NextContinuationToken
	}

	// Delete the bucket
	_, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}
	return nil
}
