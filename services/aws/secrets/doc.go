// Package secrets provides a high-level, testable client for AWS Secrets Manager
// featuring structured logging, optional in-memory caching, and configurable
// retry behavior.
//
// The client wraps the AWS SDK v2 `secretsmanager` service to provide:
//   - Simple methods for core operations: Get, Put, Create, Describe
//   - Pluggable caching via the `Cache` interface and `InMemoryCache`
//   - Customizable retries via the `Retryer` interface and `CustomRetryer`
//   - Consistent, security-conscious error handling with typed errors
//
// Security considerations
//
//   - The package never logs secret values; only metadata like secret names
//   - Typed errors (`ErrSecretNotFound`, `ErrSecretEmpty`, `ErrAccessDenied`) avoid
//     leaking sensitive details while remaining actionable
//   - IAM permissions should follow least privilege; typical permissions include
//     `secretsmanager:GetSecretValue`, `secretsmanager:PutSecretValue`,
//     `secretsmanager:CreateSecret`, `secretsmanager:DescribeSecret`, and when using
//     customer-managed keys: `kms:Decrypt` and `kms:GenerateDataKey`
//
// # Thread safety
//
// All exported client methods are safe for concurrent use by multiple goroutines.
// The underlying AWS SDK v2 client is thread-safe; the provided `InMemoryCache`
// is protected by a mutex; and the default/custom retryers are immutable.
//
// # Usage
//
// See the package examples for basic usage, caching, custom retry configuration,
// and error handling patterns.
package secrets
