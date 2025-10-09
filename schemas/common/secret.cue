package common

// AWSSecretRef references a secret stored in AWS Secrets Manager.
// Discriminated by provider!: "aws".
#AWSSecretRef: {
	provider!: "aws"
	name:      string // ARN or name of secret in AWS Secrets Manager
	key?:      string // Optional key within secret
	region?:   string // AWS region (optional, uses default if not specified)
}

// VaultSecretRef references a secret stored in HashiCorp Vault.
// Discriminated by provider!: "vault".
#VaultSecretRef: {
	provider!: "vault"
	path:      string // Path to secret in Vault (e.g., "secret/data/myapp/api-key")
	key?:      string // Optional key within secret (for KV v2)
}

// SecretRef is a universal secret reference that can point to secrets
// in different providers (AWS Secrets Manager, HashiCorp Vault).
// This is a discriminated union based on the provider field.
#SecretRef: #AWSSecretRef | #VaultSecretRef
