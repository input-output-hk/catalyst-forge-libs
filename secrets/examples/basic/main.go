// Package main demonstrates basic usage of the secrets management module.
// This example shows how to:
// - Set up a Manager with a memory provider
// - Store and resolve secrets
// - Use secrets safely with automatic cleanup
// - Handle errors appropriately
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/input-output-hk/catalyst-forge-libs/secrets"
	"github.com/input-output-hk/catalyst-forge-libs/secrets/providers/memory"
)

func main() {
	ctx := context.Background()

	// 1. Create a new Manager with auto-clear enabled for security
	config := &secrets.Config{
		DefaultProvider: "memory", // Use memory provider as default
		AutoClear:       true,     // Automatically clear secrets after use
		EnableAudit:     false,    // Disable audit for this simple example
	}

	manager := secrets.NewManager(config)
	defer func() {
		if err := manager.Close(); err != nil {
			log.Printf("Error closing manager: %v", err)
		}
	}()

	// 2. Register the memory provider
	memoryProvider := memory.New()
	if err := manager.RegisterProvider("memory", memoryProvider); err != nil {
		log.Fatalf("Failed to register memory provider: %v", err)
	}

	// 3. Store a secret
	dbPassword := []byte("my-secret-database-password")
	ref := secrets.SecretRef{
		Path:    "database/password",
		Version: "", // Use default (latest) version
		Metadata: map[string]string{
			"service": "postgresql",
		},
	}

	if err := memoryProvider.Store(ctx, ref, dbPassword); err != nil {
		log.Fatalf("Failed to store secret: %v", err)
	}

	fmt.Println("✓ Secret stored successfully")

	// 4. Resolve the secret (this creates a copy with auto-clear enabled)
	resolvedSecret, err := manager.Resolve(ctx, ref)
	if err != nil {
		log.Fatalf("Failed to resolve secret: %v", err)
	}

	fmt.Printf("✓ Secret resolved: version=%s, created=%s\n",
		resolvedSecret.Version, resolvedSecret.CreatedAt.Format("2006-01-02 15:04:05"))

	// 5. Use the secret safely - String() method will consume it if AutoClear is enabled
	password := resolvedSecret.String()
	fmt.Printf("✓ Retrieved password: %s\n", password)

	// 6. Try to use the secret again - it should be cleared
	password2 := resolvedSecret.String()
	fmt.Printf("✓ Second retrieval (should be empty): %q\n", password2)

	// 7. Demonstrate batch operations
	refs := []secrets.SecretRef{
		{Path: "database/password"},
		{Path: "api/key", Version: "v1"},
		{Path: "nonexistent/secret"}, // This won't exist
	}

	batchResults, err := manager.ResolveBatch(ctx, refs)
	if err != nil {
		log.Printf("Batch operation had errors: %v", err)
	}

	fmt.Printf("✓ Batch operation found %d secrets:\n", len(batchResults))
	for path, secret := range batchResults {
		fmt.Printf("  - %s (version: %s)\n", path, secret.Version)
		// Consume the secrets
		secret.String()
	}

	// 8. Check if secrets exist without retrieving them
	exists, err := manager.Exists(ctx, secrets.SecretRef{Path: "database/password"})
	if err != nil {
		log.Printf("Error checking existence: %v", err)
	} else {
		fmt.Printf("✓ Secret exists check: %v\n", exists)
	}

	fmt.Println("✓ All operations completed successfully!")
}
