// Package main demonstrates comprehensive error handling patterns with the secrets module.
// This example shows how to:
// - Handle different types of errors appropriately
// - Use error wrapping and unwrapping
// - Implement graceful error recovery
// - Handle batch operation partial failures
// - Check error types using errors.Is()
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/input-output-hk/catalyst-forge-libs/secrets"
	"github.com/input-output-hk/catalyst-forge-libs/secrets/providers/memory"
)

func main() {
	ctx := context.Background()

	// 1. Set up the manager and provider
	config := &secrets.Config{
		DefaultProvider: "memory",
		AutoClear:       true,
		EnableAudit:     false,
	}

	manager := secrets.NewManager(config)
	defer func() {
		if err := manager.Close(); err != nil {
			log.Printf("Error closing manager: %v", err)
		}
	}()

	provider := memory.New()
	if err := manager.RegisterProvider("memory", provider); err != nil {
		log.Fatalf("Failed to register provider: %v", err)
	}

	fmt.Println("🚀 Starting error handling demonstration...")

	// 2. Demonstrate Secret Not Found errors
	fmt.Println("\n❌ 1. Secret Not Found Errors")

	nonExistentRef := secrets.SecretRef{Path: "nonexistent/secret"}
	_, err := manager.Resolve(ctx, nonExistentRef)
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			fmt.Printf("✓ Correctly identified secret not found: %v\n", err)
		} else {
			fmt.Printf("⚠ Unexpected error type: %v\n", err)
		}
	}

	// 3. Demonstrate Invalid Reference errors
	fmt.Println("\n❌ 2. Invalid Reference Errors")

	invalidRef := secrets.SecretRef{Path: ""} // Empty path
	_, err = manager.Resolve(ctx, invalidRef)
	if err != nil {
		if errors.Is(err, secrets.ErrInvalidRef) {
			fmt.Printf("✓ Correctly identified invalid reference: %v\n", err)
		} else {
			fmt.Printf("⚠ Unexpected error type: %v\n", err)
		}
	}

	// 4. Demonstrate Provider Not Found errors
	fmt.Println("\n❌ 3. Provider Not Found Errors")

	_, err = manager.ResolveFrom(ctx, "nonexistent-provider", nonExistentRef)
	if err != nil {
		fmt.Printf("✓ Provider not found error: %v\n", err)
		// Check if it's a provider error
		if secrets.IsProviderError(err) {
			fmt.Println("✓ Correctly identified as provider error")
		}
	}

	// 5. Demonstrate successful operations and then error recovery
	fmt.Println("\n✅ 4. Error Recovery Patterns")

	// Store a valid secret first
	validSecret := []byte("valid-secret-value")
	validRef := secrets.SecretRef{Path: "valid/secret"}
	if err := provider.Store(ctx, validRef, validSecret); err != nil {
		log.Fatalf("Failed to store valid secret: %v", err)
	}
	fmt.Println("✓ Stored valid secret for testing")

	// Now try different operations
	secret, err := manager.Resolve(ctx, validRef)
	if err != nil {
		fmt.Printf("✗ Unexpected error: %v\n", err)
	} else {
		fmt.Printf("✓ Successfully resolved secret: %s\n", secret.String())
	}

	// 6. Demonstrate batch operation error handling
	fmt.Println("\n📦 5. Batch Operation Error Handling")

	batchRefs := []secrets.SecretRef{
		{Path: "valid/secret"},       // Should succeed
		{Path: "nonexistent/secret"}, // Should fail
		{Path: "another/valid"},      // Should fail (doesn't exist)
	}

	results, err := manager.ResolveBatch(ctx, batchRefs)
	if err != nil {
		fmt.Printf("✓ Batch operation completed with partial failures: %v\n", err)
	} else {
		fmt.Println("✓ Batch operation completed successfully")
	}

	fmt.Printf("✓ Successfully resolved %d out of %d secrets:\n", len(results), len(batchRefs))
	for path, secret := range results {
		fmt.Printf("  - %s: %s\n", path, secret.String())
	}

	// 7. Demonstrate context cancellation
	fmt.Println("\n⏰ 6. Context Cancellation Handling")

	// Create a context that will be cancelled
	cancelCtx, cancel := context.WithCancel(ctx)

	// Cancel immediately to simulate timeout
	cancel()

	_, err = manager.Resolve(cancelCtx, validRef)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Printf("✓ Correctly handled context cancellation: %v\n", err)
		} else {
			fmt.Printf("⚠ Unexpected context error: %v\n", err)
		}
	}

	// 8. Demonstrate error chaining and unwrapping
	fmt.Println("\n🔗 7. Error Chaining and Unwrapping")

	_, err = manager.ResolveFrom(ctx, "memory", secrets.SecretRef{Path: "missing/secret"})
	if err != nil {
		fmt.Printf("✓ Original error: %v\n", err)

		// Try to unwrap the error
		unwrapped := err
		for unwrapped != nil {
			fmt.Printf("  Unwrapped: %T - %v\n", unwrapped, unwrapped)
			unwrapped = errors.Unwrap(unwrapped)
		}
	}

	// 9. Demonstrate graceful degradation
	fmt.Println("\n🛡️ 8. Graceful Degradation")

	fmt.Println("✓ Implementing retry logic with exponential backoff...")
	if err := retryResolve(manager, ctx, nonExistentRef, 3); err != nil {
		fmt.Printf("✓ Retry logic completed (expected failure): %v\n", err)
	}

	fmt.Println("\n🎯 Error handling demonstration completed!")
	fmt.Println("Key takeaways:")
	fmt.Println("- Use errors.Is() to check for specific error types")
	fmt.Println("- Use secrets.IsProviderError() for provider-specific errors")
	fmt.Println("- Handle batch operations gracefully with partial results")
	fmt.Println("- Implement retry logic for transient failures")
	fmt.Println("- Always check context cancellation")
}

// retryResolve demonstrates a simple retry pattern
func retryResolve(manager *secrets.Manager, ctx context.Context, ref secrets.SecretRef, maxAttempts int) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		fmt.Printf("  Attempt %d/%d...\n", attempt, maxAttempts)

		secret, err := manager.Resolve(ctx, ref)
		if err == nil {
			fmt.Printf("    ✓ Success on attempt %d\n", attempt)
			_ = secret.String() // Consume the secret
			return nil
		}

		lastErr = err

		// Don't retry on certain error types
		if errors.Is(err, secrets.ErrInvalidRef) {
			fmt.Printf("    ✗ Non-retryable error: %v\n", err)
			return err
		}

		// Simple exponential backoff (in real code, use a proper backoff library)
		if attempt < maxAttempts {
			fmt.Printf("    ⏳ Retrying in %dms...\n", attempt*100)
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
		}
	}

	fmt.Printf("    ✗ Failed after %d attempts\n", maxAttempts)
	return lastErr
}
