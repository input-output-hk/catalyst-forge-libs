// Package secrets defines interfaces for AWS Secrets Manager operations.
package secrets

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

//go:generate go run github.com/matryer/moq@v0.5.3 -out ./mocks/secretsmanager_api.go -pkg mocks . ManagerAPI

// ManagerAPI defines the interface for AWS Secrets Manager operations.
// This interface abstracts the AWS SDK v2 SecretsManager client to enable
// testing with mocks and to provide a stable API surface.
type ManagerAPI interface {
	// GetSecretValue retrieves the value of a secret from AWS Secrets Manager.
	GetSecretValue(
		ctx context.Context,
		params *secretsmanager.GetSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.GetSecretValueOutput, error)

	// PutSecretValue stores or updates a secret value in AWS Secrets Manager.
	PutSecretValue(
		ctx context.Context,
		params *secretsmanager.PutSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.PutSecretValueOutput, error)

	// CreateSecret creates a new secret in AWS Secrets Manager.
	CreateSecret(
		ctx context.Context,
		params *secretsmanager.CreateSecretInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.CreateSecretOutput, error)

	// DescribeSecret retrieves metadata about a secret without exposing its value.
	DescribeSecret(
		ctx context.Context,
		params *secretsmanager.DescribeSecretInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.DescribeSecretOutput, error)
}
