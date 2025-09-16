# Secrets Module Usage Examples

This directory contains comprehensive examples demonstrating how to use the secrets management module effectively. Each example is self-contained with its own `go.mod` file and focuses on specific usage patterns.

## Examples Overview

### [basic/](basic/)
**Basic Usage Example**
- Setting up a Manager with a memory provider
- Storing and resolving secrets
- Using secrets safely with automatic cleanup
- Basic error handling
- Batch operations
- Existence checks

Run with: `cd basic && go run main.go`

### [provider_registration/](provider_registration/)
**Provider Registration and Management**
- Registering multiple providers with the Manager
- Provider-specific resolution
- Health checks on providers
- Provider cleanup and lifecycle management
- Audit logging integration

Run with: `cd provider_registration && go run main.go`

### [error_handling/](error_handling/)
**Comprehensive Error Handling Patterns**
- Handling different types of errors (not found, access denied, invalid ref)
- Error wrapping and unwrapping with `errors.Is()`
- Provider-specific errors
- Batch operation partial failures
- Graceful error recovery and retry logic
- Context cancellation handling

Run with: `cd error_handling && go run main.go`

## Running the Examples

Each example can be run independently:

```bash
# From the examples directory
cd basic
go run main.go

cd ../provider_registration
go run main.go

cd ../error_handling
go run main.go
```

## Key Concepts Demonstrated

### Security Best Practices
- **Auto-clear**: Secrets are automatically zeroed after use
- **Copy-on-read**: Returned values are copies to prevent external modification
- **Memory zeroing**: Explicit cleanup prevents sensitive data leakage

### Provider Management
- **Multiple providers**: Support for different secret backends
- **Health monitoring**: Provider health checks and status verification
- **Graceful shutdown**: Proper cleanup of provider resources

### Error Handling
- **Structured errors**: Well-defined error types for different failure scenarios
- **Error chaining**: Wrapped errors maintain context through the call stack
- **Partial results**: Batch operations can succeed partially
- **Context awareness**: Proper handling of cancellation and timeouts

### Usage Patterns
- **Just-in-time resolution**: Fetch secrets only when needed
- **Secure references**: Use `SecretRef` instead of storing secret values
- **Batch operations**: Efficiently resolve multiple secrets
- **Audit logging**: Track all secret access for compliance

## Integration with Real Providers

While these examples use the memory provider for simplicity, the same patterns apply to real providers:

- **AWS Secrets Manager**: Use for cloud-native AWS deployments
- **HashiCorp Vault**: For enterprise secret management
- **Azure Key Vault**: For Microsoft Azure environments
- **Kubernetes Secrets**: For containerized applications

## Testing

The examples serve as integration tests for the secrets module. They verify:
- All interfaces work correctly
- Error handling behaves as expected
- Memory management is secure
- Provider lifecycle is managed properly

## Security Considerations

When using these examples in production:
- Enable audit logging for compliance
- Use real secret providers instead of memory
- Implement proper authentication and authorization
- Configure TLS for all provider connections
- Set appropriate timeouts and retry policies
