//nolint:revive // "common" is an appropriate package name for common types
package common

import (
	"encoding/json"
	"testing"
)

func TestSecretRef_Provider(t *testing.T) {
	tests := []struct {
		name     string
		ref      SecretRef
		expected string
	}{
		{
			name:     "aws provider",
			ref:      SecretRef{"provider": "aws"},
			expected: "aws",
		},
		{
			name:     "vault provider",
			ref:      SecretRef{"provider": "vault"},
			expected: "vault",
		},
		{
			name:     "missing provider",
			ref:      SecretRef{},
			expected: "",
		},
		{
			name:     "non-string provider",
			ref:      SecretRef{"provider": 123},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.Provider()
			if got != tt.expected {
				t.Errorf("Provider() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSecretRef_AsAWS(t *testing.T) {
	t.Run("valid aws secret", func(t *testing.T) {
		jsonData := `{
			"provider": "aws",
			"name": "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret",
			"key": "api-key",
			"region": "us-east-1"
		}`

		var ref SecretRef
		if err := json.Unmarshal([]byte(jsonData), &ref); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		aws, ok := ref.AsAWS()
		if !ok {
			t.Fatal("AsAWS() returned false, expected true")
		}
		if aws == nil {
			t.Fatal("AsAWS() returned nil secret")
		}
		if aws.Provider != "aws" {
			t.Errorf("Provider = %q, want %q", aws.Provider, "aws")
		}
		if aws.Name != "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret" {
			t.Errorf("Name = %q, want ARN", aws.Name)
		}
		if aws.Key != "api-key" {
			t.Errorf("Key = %q, want %q", aws.Key, "api-key")
		}
		if aws.Region != "us-east-1" {
			t.Errorf("Region = %q, want %q", aws.Region, "us-east-1")
		}
	})

	t.Run("aws secret minimal", func(t *testing.T) {
		jsonData := `{
			"provider": "aws",
			"name": "my-secret"
		}`

		var ref SecretRef
		if err := json.Unmarshal([]byte(jsonData), &ref); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		aws, ok := ref.AsAWS()
		if !ok {
			t.Fatal("AsAWS() returned false, expected true")
		}
		if aws.Name != "my-secret" {
			t.Errorf("Name = %q, want %q", aws.Name, "my-secret")
		}
	})

	t.Run("wrong provider", func(t *testing.T) {
		ref := SecretRef{"provider": "vault"}
		aws, ok := ref.AsAWS()
		if ok {
			t.Error("AsAWS() returned true for vault provider, expected false")
		}
		if aws != nil {
			t.Error("AsAWS() returned non-nil secret for wrong provider")
		}
	})
}

func TestSecretRef_AsVault(t *testing.T) {
	t.Run("valid vault secret", func(t *testing.T) {
		jsonData := `{
			"provider": "vault",
			"path": "secret/data/myapp/credentials",
			"key": "password"
		}`

		var ref SecretRef
		if err := json.Unmarshal([]byte(jsonData), &ref); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		vault, ok := ref.AsVault()
		if !ok {
			t.Fatal("AsVault() returned false, expected true")
		}
		if vault == nil {
			t.Fatal("AsVault() returned nil secret")
		}
		if vault.Provider != "vault" {
			t.Errorf("Provider = %q, want %q", vault.Provider, "vault")
		}
		if vault.Path != "secret/data/myapp/credentials" {
			t.Errorf("Path = %q, want %q", vault.Path, "secret/data/myapp/credentials")
		}
		if vault.Key != "password" {
			t.Errorf("Key = %q, want %q", vault.Key, "password")
		}
	})

	t.Run("vault secret minimal", func(t *testing.T) {
		jsonData := `{
			"provider": "vault",
			"path": "secret/data/api-key"
		}`

		var ref SecretRef
		if err := json.Unmarshal([]byte(jsonData), &ref); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		vault, ok := ref.AsVault()
		if !ok {
			t.Fatal("AsVault() returned false, expected true")
		}
		if vault.Path != "secret/data/api-key" {
			t.Errorf("Path = %q, want %q", vault.Path, "secret/data/api-key")
		}
	})

	t.Run("wrong provider", func(t *testing.T) {
		ref := SecretRef{"provider": "aws"}
		vault, ok := ref.AsVault()
		if ok {
			t.Error("AsVault() returned true for aws provider, expected false")
		}
		if vault != nil {
			t.Error("AsVault() returned non-nil secret for wrong provider")
		}
	})
}

func TestSecretRef_Validate(t *testing.T) {
	tests := []struct {
		name      string
		ref       SecretRef
		wantError bool
	}{
		{
			name:      "valid aws provider",
			ref:       SecretRef{"provider": "aws"},
			wantError: false,
		},
		{
			name:      "valid vault provider",
			ref:       SecretRef{"provider": "vault"},
			wantError: false,
		},
		{
			name:      "missing provider",
			ref:       SecretRef{},
			wantError: true,
		},
		{
			name:      "unknown provider",
			ref:       SecretRef{"provider": "unknown"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Validate()
			if tt.wantError && err == nil {
				t.Error("Validate() error = nil, wantError = true")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Validate() error = %v, wantError = false", err)
			}
		})
	}
}

func TestAWSSecretRef_RoundTrip(t *testing.T) {
	// Test that we can marshal an AWSSecretRef, convert to SecretRef, and back
	original := &AWSSecretRef{
		Provider: "aws",
		Name:     "my-secret",
		Key:      "password",
		Region:   "us-west-2",
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal to SecretRef
	var ref SecretRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Failed to unmarshal to SecretRef: %v", err)
	}

	// Convert back to AWSSecretRef
	result, ok := ref.AsAWS()
	if !ok {
		t.Fatal("AsAWS() returned false")
	}

	// Verify fields
	if result.Provider != original.Provider {
		t.Errorf("Provider = %q, want %q", result.Provider, original.Provider)
	}
	if result.Name != original.Name {
		t.Errorf("Name = %q, want %q", result.Name, original.Name)
	}
	if result.Key != original.Key {
		t.Errorf("Key = %q, want %q", result.Key, original.Key)
	}
	if result.Region != original.Region {
		t.Errorf("Region = %q, want %q", result.Region, original.Region)
	}
}

func TestVaultSecretRef_RoundTrip(t *testing.T) {
	// Test that we can marshal a VaultSecretRef, convert to SecretRef, and back
	original := &VaultSecretRef{
		Provider: "vault",
		Path:     "secret/data/app/creds",
		Key:      "token",
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal to SecretRef
	var ref SecretRef
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatalf("Failed to unmarshal to SecretRef: %v", err)
	}

	// Convert back to VaultSecretRef
	result, ok := ref.AsVault()
	if !ok {
		t.Fatal("AsVault() returned false")
	}

	// Verify fields
	if result.Provider != original.Provider {
		t.Errorf("Provider = %q, want %q", result.Provider, original.Provider)
	}
	if result.Path != original.Path {
		t.Errorf("Path = %q, want %q", result.Path, original.Path)
	}
	if result.Key != original.Key {
		t.Errorf("Key = %q, want %q", result.Key, original.Key)
	}
}
