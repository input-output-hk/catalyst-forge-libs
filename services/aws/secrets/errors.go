// Package secrets provides custom error types for AWS Secrets Manager operations.
//
// # Error Handling Security
//
// This package defines typed errors to ensure secure error handling:
//
// - Errors never expose sensitive information in their messages
// - Use errors.Is() to check for specific error types
// - Error messages are designed to provide context without leaking secrets
// - AWS SDK errors are wrapped to prevent information leakage
package secrets

import "errors"

var (
	// ErrSecretNotFound is returned when a requested secret does not exist
	// in AWS Secrets Manager. This typically occurs when attempting to retrieve
	// or describe a secret using an invalid secret name or ARN.
	//
	// Security note: This error does not expose whether the secret exists or not
	// in a way that could be used for enumeration attacks.
	ErrSecretNotFound = errors.New("secret not found")

	// ErrSecretEmpty is returned when a secret exists but contains no value.
	// This may indicate the secret was created but never populated, or the
	// secret value was deleted while the metadata remains.
	//
	// Security note: This error provides operational context without exposing
	// the existence or non-existence of secret values.
	ErrSecretEmpty = errors.New("secret value is empty")

	// ErrAccessDenied is returned when the AWS credentials do not have
	// sufficient permissions to perform the requested Secrets Manager operation.
	// This typically indicates missing IAM permissions for secretsmanager:* actions.
	//
	// Security note: This error helps identify permission issues without
	// exposing sensitive details about the AWS account or resource configuration.
	ErrAccessDenied = errors.New("access denied to secret")
)
