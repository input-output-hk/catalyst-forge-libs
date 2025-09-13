## AWS Secrets Manager Client for Go

High-level, test-friendly client for AWS Secrets Manager with structured logging, optional in-memory caching, and configurable retry behavior.

### Features

- **Simple API**: `GetSecret`, `PutSecret`, `CreateSecret`, `DescribeSecret`, `GetSecretCached`
- **Caching**: Pluggable cache via `Cache` interface and `InMemoryCache`
- **Retries**: Custom retry policy via `Retryer` and `CustomRetryer`
- **Typed errors**: `ErrSecretNotFound`, `ErrSecretEmpty`, `ErrAccessDenied`
- **Security-first**: Never logs secret values; clear, non-leaky errors
- **Thread-safe**: Safe for concurrent use

### Installation

```bash
go get github.com/input-output-hk/catalyst-forge-libs/services/aws/secrets
```

### Quickstart

```go
ctx := context.Background()
client, err := secrets.NewClient(ctx)
if err != nil { /* handle */ }

// Create, then read a secret
_ = client.CreateSecret(ctx, "app/db/password", "s3cr3t", "")
value, err := client.GetSecret(ctx, "app/db/password")
if err != nil { /* handle */ }
fmt.Println(value)
```

### Usage

#### Basic client with logger

```go
cfg := aws.Config{Region: "us-east-1"}
client, err := secrets.NewClientWithConfig(ctx, &cfg, secrets.WithLogger(slog.Default()))
```

#### Caching

```go
client, err := secrets.NewClientWithCache(ctx, 5*time.Minute, 100)
val, err := client.GetSecretCached(ctx, "app/db/password")
client.InvalidateCache("app/db/password")
client.ClearCache()
```

#### Custom retry policy

```go
retryer := &secrets.CustomRetryer{maxAttempts: 8, baseDelay: 100 * time.Millisecond, maxDelay: 5 * time.Second}
client, err := secrets.NewClient(ctx, secrets.WithCustomRetryer(retryer))
```

#### Error handling

```go
val, err := client.GetSecret(ctx, "missing/secret")
switch {
case err == nil:
    fmt.Println(val)
case errors.Is(err, secrets.ErrSecretNotFound):
    // not found
case errors.Is(err, secrets.ErrAccessDenied):
    // permission issue
case errors.Is(err, secrets.ErrSecretEmpty):
    // empty value
default:
    // generic failure
}
```

### LocalStack (integration testing)

Use LocalStack in tests via `NewClientWithLocalStack` or by passing a custom `aws.Config` endpoint.

```go
client, err := secrets.NewClientWithLocalStack(ctx, "http://localhost:4566")
```

See `integration_test.go` for a complete LocalStack suite using Testcontainers.

### Security

- Never logs secret values; only names and operation metadata
- Prefer least-privileged IAM policies
- When using customer-managed KMS keys, ensure `kms:Decrypt` and `kms:GenerateDataKey` permissions

Example minimal policy (adjust `Resource` as needed):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:PutSecretValue",
        "secretsmanager:CreateSecret",
        "secretsmanager:DescribeSecret"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": ["kms:Decrypt", "kms:GenerateDataKey"],
      "Resource": "*"
    }
  ]
}
```

### Thread Safety

All exported client methods are safe for concurrent use. The AWS SDK v2 client is thread-safe. `InMemoryCache` uses a mutex; retryers are immutable.

### Logging

Use `WithLogger(*slog.Logger)` to enable structured logging. The package never logs secret values.

### Testing

- Unit tests use interfaces and mocks (`go:generate` with `moq`) for isolation
- Integration tests run against LocalStack with Testcontainers
- Examples are runnable and self-contained (`examples_test.go`)

Run tests and examples:

```bash
go test ./...
go test -run Example -v
```

### API Reference

- Package docs and examples are available via `go doc` and in-source comments
- Key types: `Client`, `Cache`, `InMemoryCache`, `Retryer`, `CustomRetryer`
- Key errors: `ErrSecretNotFound`, `ErrSecretEmpty`, `ErrAccessDenied`

### License

Apache-2.0. See `LICENSE` at the repository root.


