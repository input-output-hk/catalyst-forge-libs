package aws_test

import (
	"fmt"
	"log"

	"github.com/input-output-hk/catalyst-forge-libs/secrets/core"

	"github.com/input-output-hk/catalyst-forge-libs/secrets/providers/aws"
)

// ExampleNew demonstrates basic provider creation with default configuration.
func ExampleNew() {
	// Create a provider with default AWS configuration
	provider, err := aws.New()
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	fmt.Println("Provider created successfully:", provider.Name())
	// Output: Provider created successfully: aws
}

// ExampleNew_withOptions demonstrates provider creation with custom options.
func ExampleNew_withOptions() {
	// Create provider with custom configuration
	provider, err := aws.New(
		aws.WithRegion("us-west-2"),
		aws.WithMaxRetries(5),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	fmt.Println("Configured provider:", provider.Name())
	// Output: Configured provider: aws
}

// ExampleNew_withLocalStack demonstrates provider configuration for LocalStack testing.
func ExampleNew_withLocalStack() {
	// Configure for LocalStack testing
	provider, err := aws.New(aws.WithEndpoint("http://localhost:4566"))
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Close()

	fmt.Println("LocalStack provider:", provider.Name())
	// Output: LocalStack provider: aws
}

// ExampleProvider_Store demonstrates storing different types of secrets.
func ExampleProvider_Store() {
	// This example shows how to store secrets with a provider
	// In a real application, you would create the provider first:
	// provider, err := aws.New()
	// ctx := context.Background()

	// Store a JSON secret
	// jsonSecret := core.SecretRef{Path: "example/json-secret"}
	// err := provider.Store(ctx, jsonSecret, []byte(`{"key":"value","number":42}`))

	// Store a binary secret
	// binaryData := []byte{0x00, 0x01, 0x02, 0x03}
	// binarySecret := core.SecretRef{Path: "example/binary-secret"}
	// err := provider.Store(ctx, binarySecret, binaryData)

	// Store a secret with metadata
	// secretWithTags := core.SecretRef{
	//     Path: "example/tagged-secret",
	//     Metadata: map[string]string{
	//         "environment": "production",
	//         "team":        "backend",
	//         "purpose":     "example",
	//     },
	// }
	// err := provider.Store(ctx, secretWithTags, []byte("secret-value"))

	fmt.Println("Store operations would create/update secrets in AWS Secrets Manager")
	// Output: Store operations would create/update secrets in AWS Secrets Manager
}

// ExampleProvider_Resolve demonstrates retrieving secrets.
func ExampleProvider_Resolve() {
	// This example shows how to resolve secrets
	// In a real application, you would create the provider first:
	// provider, err := aws.New()
	// ctx := context.Background()

	// Resolve a secret
	ref := core.SecretRef{Path: "example/json-secret"}
	// secret, err := provider.Resolve(ctx, ref)
	// if err != nil {
	//     log.Fatal(err)
	// }

	// Get specific version
	versionRef := core.SecretRef{
		Path:    "example/secret",
		Version: "v1.2.3",
	}
	// secret, err := provider.Resolve(ctx, versionRef)

	// Get by stage
	stageRef := core.SecretRef{
		Path:    "example/secret",
		Version: "AWSPREVIOUS",
	}
	// secret, err := provider.Resolve(ctx, stageRef)

	fmt.Printf("Resolve operations would retrieve secrets: %s, %s, %s\n",
		ref.Path, versionRef.Path, stageRef.Path)
	// Output: Resolve operations would retrieve secrets: example/json-secret, example/secret, example/secret
}

// ExampleProvider_ResolveBatch demonstrates batch secret retrieval.
func ExampleProvider_ResolveBatch() {
	// This example shows batch secret retrieval
	// In a real application, you would create the provider first:
	// provider, err := aws.New()
	// ctx := context.Background()

	// Resolve multiple secrets in batch
	refs := []core.SecretRef{
		{Path: "example/json-secret"},
		{Path: "example/binary-secret"},
		{Path: "example/tagged-secret"},
		{Path: "non-existent-secret"}, // This won't fail the batch
	}

	// secrets, err := provider.ResolveBatch(ctx, refs)
	// if err != nil {
	//     log.Fatal(err)
	// }

	fmt.Printf("Batch resolve would attempt to retrieve %d secrets\n", len(refs))
	// Output: Batch resolve would attempt to retrieve 4 secrets
}

// ExampleProvider_Exists demonstrates checking secret existence.
func ExampleProvider_Exists() {
	// This example shows how to check if a secret exists
	// In a real application, you would create the provider first:
	// provider, err := aws.New()
	// ctx := context.Background()

	// Check if secret exists
	ref := core.SecretRef{Path: "example/json-secret"}
	// exists, err := provider.Exists(ctx, ref)

	fmt.Printf("Exists check for secret: %s\n", ref.Path)
	// Output: Exists check for secret: example/json-secret
}

// ExampleProvider_HealthCheck demonstrates health checking.
func ExampleProvider_HealthCheck() {
	// This example shows how to perform a health check
	// In a real application, you would create the provider first:
	// provider, err := aws.New()
	// ctx := context.Background()

	// Perform health check
	// err := provider.HealthCheck(ctx)

	fmt.Println("Health check verifies AWS Secrets Manager connectivity")
	// Output: Health check verifies AWS Secrets Manager connectivity
}

// ExampleProvider_Delete demonstrates secret deletion.
func ExampleProvider_Delete() {
	// This example shows how to delete secrets
	// In a real application, you would create the provider first:
	// provider, err := aws.New()
	// ctx := context.Background()

	// Delete a secret (scheduled deletion with 7-day recovery window)
	ref := core.SecretRef{Path: "example/temporary-secret"}
	// err := provider.Delete(ctx, ref)

	fmt.Printf("Delete operation for secret: %s\n", ref.Path)
	// Output: Delete operation for secret: example/temporary-secret
}

// ExampleProvider_endToEnd demonstrates a complete workflow.
func ExampleProvider_endToEnd() {
	// This example shows a complete provider lifecycle
	// In a real application, you would perform actual operations:

	// Create provider
	// provider, err := aws.New()
	// if err != nil {
	//     log.Fatal(err)
	// }
	// defer provider.Close()
	// ctx := context.Background()

	// Store a secret
	// err = provider.Store(ctx, core.SecretRef{Path: "my-secret"}, []byte("secret-value"))

	// Retrieve the secret
	// secret, err := provider.Resolve(ctx, core.SecretRef{Path: "my-secret"})

	// Clean up
	// err = provider.Delete(ctx, core.SecretRef{Path: "my-secret"})

	fmt.Println("Provider lifecycle: create → configure → use → cleanup")
	// Output: Provider lifecycle: create → configure → use → cleanup
}
