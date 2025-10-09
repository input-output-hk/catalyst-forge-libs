// Package common provides helper methods for working with common discriminated unions.
//
//nolint:revive // "common" is an appropriate package name for common types
package common

import (
	"encoding/json"
	"fmt"
)

// Provider returns the discriminator value for this SecretRef.
// Returns an empty string if the provider field is missing or not a string.
func (sr SecretRef) Provider() string {
	if p, ok := sr["provider"].(string); ok {
		return p
	}
	return ""
}

// AsAWS attempts to convert the SecretRef to an AWSSecretRef.
// Returns the secret reference and true if successful, nil and false otherwise.
func (sr SecretRef) AsAWS() (*AWSSecretRef, bool) {
	if sr.Provider() != "aws" {
		return nil, false
	}

	// Re-marshal and unmarshal to convert map[string]any to AWSSecretRef
	data, err := json.Marshal(sr)
	if err != nil {
		return nil, false
	}

	var aws AWSSecretRef
	if err := json.Unmarshal(data, &aws); err != nil {
		return nil, false
	}

	return &aws, true
}

// AsVault attempts to convert the SecretRef to a VaultSecretRef.
// Returns the secret reference and true if successful, nil and false otherwise.
func (sr SecretRef) AsVault() (*VaultSecretRef, bool) {
	if sr.Provider() != "vault" {
		return nil, false
	}

	data, err := json.Marshal(sr)
	if err != nil {
		return nil, false
	}

	var vault VaultSecretRef
	if err := json.Unmarshal(data, &vault); err != nil {
		return nil, false
	}

	return &vault, true
}

// Validate checks if the SecretRef has a valid provider discriminator.
// Returns an error if the provider is missing or unknown.
func (sr SecretRef) Validate() error {
	provider := sr.Provider()
	if provider == "" {
		return fmt.Errorf("secret reference missing 'provider' field")
	}

	switch provider {
	case "aws", "vault":
		return nil
	default:
		return fmt.Errorf("unknown secret provider: %q", provider)
	}
}
