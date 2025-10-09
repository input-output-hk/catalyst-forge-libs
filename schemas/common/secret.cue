package common

// AWSSecretRef references a secret stored in AWS Secrets Manager.
// Discriminated by provider!: "aws".
#AWSSecretRef: {
	provider!: "aws"
	// ARN or name of secret in AWS Secrets Manager
	name: string
	// Optional key within secret
	key?: string
	// AWS region (optional, uses default if not specified)
	region?: string
}

// VaultSecretRef references a secret stored in HashiCorp Vault.
// Discriminated by provider!: "vault".
#VaultSecretRef: {
	provider!: "vault"
	// Path to secret in Vault (e.g., "secret/data/myapp/api-key")
	path: string
	// Optional key within secret (for KV v2)
	key?: string
}

// SecretRef is a universal secret reference that can point to secrets
// in different providers (AWS Secrets Manager, HashiCorp Vault).
// This is a discriminated union based on the provider field.
#SecretRef: #AWSSecretRef | #VaultSecretRef
