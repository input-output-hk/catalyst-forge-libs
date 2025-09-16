// Package main demonstrates provider registration and management.
// This example shows how to:
// - Register multiple providers with the Manager
// - Use provider-specific resolution
// - Perform health checks on providers
// - Handle provider-specific operations
// - Gracefully close providers
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/input-output-hk/catalyst-forge-libs/secrets"
	"github.com/input-output-hk/catalyst-forge-libs/secrets/providers/memory"
)

func main() {
	ctx := context.Background()

	// 1. Create a Manager with audit logging enabled
	config := &secrets.Config{
		DefaultProvider: "primary", // Set primary memory provider as default
		AutoClear:       true,
		EnableAudit:     true,                 // Enable audit logging
		AuditLogger:     &simpleAuditLogger{}, // Simple audit logger implementation
	}

	manager := secrets.NewManager(config)

	// Ensure manager is closed at the end
	defer func() {
		fmt.Println("🔒 Closing manager...")
		if err := manager.Close(); err != nil {
			log.Printf("Error closing manager: %v", err)
		}
	}()

	// 2. Create and register multiple memory providers
	fmt.Println("📝 Registering providers...")

	primaryProvider := memory.New()
	if err := manager.RegisterProvider("primary", primaryProvider); err != nil {
		log.Fatalf("Failed to register primary provider: %v", err)
	}
	fmt.Println("✓ Registered 'primary' provider")

	secondaryProvider := memory.New()
	if err := manager.RegisterProvider("secondary", secondaryProvider); err != nil {
		log.Fatalf("Failed to register secondary provider: %v", err)
	}
	fmt.Println("✓ Registered 'secondary' provider")

	// 3. Store secrets in different providers
	fmt.Println("\n💾 Storing secrets...")

	// Store in primary provider
	primarySecret := []byte("primary-secret-value")
	primaryRef := secrets.SecretRef{
		Path:    "app/config",
		Version: "v1.0",
		Metadata: map[string]string{
			"environment": "production",
		},
	}

	if err := primaryProvider.Store(ctx, primaryRef, primarySecret); err != nil {
		log.Fatalf("Failed to store in primary provider: %v", err)
	}
	fmt.Println("✓ Stored secret in 'primary' provider")

	// Store in secondary provider
	secondarySecret := []byte("secondary-secret-value")
	secondaryRef := secrets.SecretRef{
		Path:    "app/config",
		Version: "v2.0",
		Metadata: map[string]string{
			"environment": "staging",
		},
	}

	if err := secondaryProvider.Store(ctx, secondaryRef, secondarySecret); err != nil {
		log.Fatalf("Failed to store in secondary provider: %v", err)
	}
	fmt.Println("✓ Stored secret in 'secondary' provider")

	// 4. Demonstrate provider-specific resolution
	fmt.Println("\n🔍 Resolving secrets from specific providers...")

	// Resolve from primary provider
	secret, err := manager.ResolveFrom(ctx, "primary", primaryRef)
	if err != nil {
		log.Fatalf("Failed to resolve from primary provider: %v", err)
	}
	fmt.Printf("✓ Primary provider secret: %s\n", secret.String())

	// Resolve from secondary provider
	secret, err = manager.ResolveFrom(ctx, "secondary", secondaryRef)
	if err != nil {
		log.Fatalf("Failed to resolve from secondary provider: %v", err)
	}
	fmt.Printf("✓ Secondary provider secret: %s\n", secret.String())

	// 5. Demonstrate default provider resolution
	fmt.Println("\n🎯 Using default provider...")

	defaultSecret, err := manager.Resolve(ctx, primaryRef)
	if err != nil {
		log.Fatalf("Failed to resolve from default provider: %v", err)
	}
	fmt.Printf("✓ Default provider resolution: %s\n", defaultSecret.String())

	// 6. Perform health checks
	fmt.Println("\n🏥 Performing health checks...")

	if err := primaryProvider.HealthCheck(ctx); err != nil {
		log.Printf("Primary provider health check failed: %v", err)
	} else {
		fmt.Println("✓ Primary provider is healthy")
	}

	if err := secondaryProvider.HealthCheck(ctx); err != nil {
		log.Printf("Secondary provider health check failed: %v", err)
	} else {
		fmt.Println("✓ Secondary provider is healthy")
	}

	// 7. Demonstrate existence checks
	fmt.Println("\n❓ Checking secret existence...")

	exists, err := manager.ExistsFrom(ctx, "primary", primaryRef)
	if err != nil {
		log.Printf("Error checking existence: %v", err)
	} else {
		fmt.Printf("✓ Secret exists in primary provider: %v\n", exists)
	}

	// Check non-existent secret
	nonExistentRef := secrets.SecretRef{Path: "nonexistent/path"}
	exists, err = manager.Exists(ctx, nonExistentRef)
	if err != nil {
		log.Printf("Error checking existence: %v", err)
	} else {
		fmt.Printf("✓ Non-existent secret check: %v\n", exists)
	}

	// 8. Demonstrate error handling
	fmt.Println("\n🚨 Demonstrating error handling...")

	// Try to resolve from non-existent provider
	_, err = manager.ResolveFrom(ctx, "nonexistent", primaryRef)
	if err != nil {
		fmt.Printf("✓ Expected error for non-existent provider: %v\n", err)
	}

	// Try to resolve non-existent secret
	_, err = manager.Resolve(ctx, nonExistentRef)
	if err != nil {
		fmt.Printf("✓ Expected error for non-existent secret: %v\n", err)
	}

	fmt.Println("\n🎉 All provider registration examples completed successfully!")
}

// simpleAuditLogger is a basic implementation of AuditLogger for demonstration
type simpleAuditLogger struct{}

func (l *simpleAuditLogger) LogAccess(
	ctx context.Context,
	action string,
	ref secrets.SecretRef,
	success bool,
	err error,
) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}

	fmt.Printf("[AUDIT %s] %s %s %s@%s\n",
		timestamp, status, action, ref.Path, ref.Version)
}
