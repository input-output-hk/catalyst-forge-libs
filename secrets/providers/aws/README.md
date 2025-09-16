# AWS Secrets Manager Provider

AWS Secrets Manager provider implementation for the catalyst-forge-libs secrets framework. Provides secure, test-friendly access to AWS Secrets Manager with just-in-time credential loading and proper error handling.

## Features

- **Provider Interface**: Implements `core.Provider` and `core.WriteableProvider` interfaces
- **AWS SDK v2**: Uses official AWS SDK v2 with credential chain support
- **Test-First Design**: Mock-friendly interfaces for unit testing
- **Security-First**: Just-in-time credential loading, no secret value logging
- **Thread-Safe**: Safe for concurrent use with proper mutex usage
- **Context Support**: Respects context cancellation throughout operations

## Installation

```bash
go get github.com/input-output-hk/catalyst-forge-libs/secrets/providers/aws
```

## Quickstart

```go
ctx := context.Background()

// Create provider with default AWS configuration
provider, err := aws.New(ctx)
if err != nil {
    // handle error
}

// Store a secret
ref := core.SecretRef{Path: "app/db/password"}
err = provider.Store(ctx, ref, []byte("my-secret-password"))

// Retrieve a secret
secret, err := provider.Resolve(ctx, ref)
if err != nil {
    // handle error
}
password := secret.String() // Auto-cleared after use
```

## Usage

### Basic Provider

```go
// Default provider using AWS credential chain
provider, err := aws.New(ctx)
```

### Custom Configuration

```go
// Provider with custom AWS config
cfg := &aws.Config{
    Region: "us-west-2",
}
provider, err := aws.NewWithConfig(ctx, cfg)
```

### Functional Options

```go
// Provider with custom options
provider, err := aws.New(ctx,
    aws.WithRegion("us-east-1"),
    aws.WithMaxRetries(3),
)
```

## Security

- **Just-in-Time Credentials**: AWS credentials loaded only when needed
- **No Secret Logging**: Secret values never appear in logs or errors
- **Memory Safety**: Secrets cleared after use when `AutoClear` is enabled
- **IAM Best Practices**: Follow least-privilege access patterns

### Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:DescribeSecret",
        "secretsmanager:CreateSecret",
        "secretsmanager:PutSecretValue",
        "secretsmanager:DeleteSecret",
        "secretsmanager:UpdateSecret"
      ],
      "Resource": "*"
    }
  ]
}
```

## Testing

### Unit Tests
Unit tests use mocked AWS clients for isolation:
```bash
go test ./...
```

### Integration Tests
Integration tests require LocalStack:
```bash
go test -tags=integration ./...
```

## Thread Safety

All provider methods are thread-safe and can be used concurrently from multiple goroutines.

## Error Handling

```go
secret, err := provider.Resolve(ctx, ref)
if err != nil {
    switch {
    case errors.Is(err, core.ErrSecretNotFound):
        // Secret doesn't exist
    default:
        // Other provider errors
    }
}
```

## API Reference

- Implements `core.Provider` and `core.WriteableProvider` interfaces
- Key methods: `Resolve`, `ResolveBatch`, `Store`, `Delete`, `Rotate`
- Thread-safe for concurrent use
- Context-aware with proper cancellation support

## License

Apache-2.0. See `LICENSE` at the repository root.
