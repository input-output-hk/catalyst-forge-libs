package core

import (
	"bytes"
	"testing"
	"time"
)

// TestSecretClearMemoryZeroing tests that Clear() actually zeros memory
func TestSecretClearMemoryZeroing(t *testing.T) {
	// Create a secret with known data
	originalValue := []byte("super-secret-password")
	secret := &Secret{
		Value:     make([]byte, len(originalValue)),
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false,
	}
	copy(secret.Value, originalValue)

	// Verify initial state
	if !bytes.Equal(secret.Value, originalValue) {
		t.Fatal("Initial secret value not set correctly")
	}

	// Clear the secret
	secret.Clear()

	// Verify memory is zeroed - all bytes should be 0
	for i, b := range secret.Value {
		if b != 0 {
			t.Errorf("Memory not zeroed at position %d: expected 0, got %d", i, b)
		}
	}

	// Verify Value slice is nil
	if secret.Value != nil {
		t.Error("Value slice should be nil after Clear()")
	}
}

// TestSecretClearNilValue tests Clear() on nil value
func TestSecretClearNilValue(t *testing.T) {
	secret := &Secret{
		Value:     nil,
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false,
	}

	// Should not panic
	secret.Clear()

	if secret.Value != nil {
		t.Error("Value should remain nil")
	}
}

// TestSecretAutoClearString tests consume-on-read behavior for String()
func TestSecretAutoClearString(t *testing.T) {
	originalValue := []byte("test-password")
	secret := &Secret{
		Value:     make([]byte, len(originalValue)),
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: true,
	}
	copy(secret.Value, originalValue)

	// Call String() - should consume the secret
	result := secret.String()

	if result != "test-password" {
		t.Errorf("Expected 'test-password', got '%s'", result)
	}

	// Verify secret was consumed
	if secret.Value != nil {
		t.Error("Secret should be cleared after String() with AutoClear=true")
	}
}

// TestSecretAutoClearBytes tests consume-on-read behavior for Bytes()
func TestSecretAutoClearBytes(t *testing.T) {
	originalValue := []byte("test-password")
	secret := &Secret{
		Value:     make([]byte, len(originalValue)),
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: true,
	}
	copy(secret.Value, originalValue)

	// Call Bytes() - should consume the secret
	result := secret.Bytes()

	if !bytes.Equal(result, originalValue) {
		t.Errorf("Expected %v, got %v", originalValue, result)
	}

	// Verify secret was consumed
	if secret.Value != nil {
		t.Error("Secret should be cleared after Bytes() with AutoClear=true")
	}
}

// TestSecretAutoClearUnmarshalJSON tests consume-on-read behavior for UnmarshalJSON()
func TestSecretAutoClearUnmarshalJSON(t *testing.T) {
	jsonData := `{
		"value": "dGVzdC1wYXNzd29yZA==",
		"version": "v1",
		"created_at": "2023-01-01T00:00:00Z",
		"auto_clear": true
	}`

	secret := &Secret{}
	err := secret.UnmarshalJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	// Verify secret was consumed due to AutoClear=true
	if secret.Value != nil {
		t.Error("Secret should be cleared after UnmarshalJSON() with AutoClear=true")
	}

	// Verify other fields were set correctly
	if secret.Version != "v1" {
		t.Errorf("Expected version 'v1', got '%s'", secret.Version)
	}
	if !secret.AutoClear {
		t.Error("AutoClear should be true")
	}
}

// TestSecretNoAutoClear tests that methods don't consume when AutoClear=false
func TestSecretNoAutoClear(t *testing.T) {
	originalValue := []byte("test-password")
	secret := &Secret{
		Value:     make([]byte, len(originalValue)),
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false,
	}
	copy(secret.Value, originalValue)

	// Call String() - should NOT consume the secret
	result := secret.String()
	if result != "test-password" {
		t.Errorf("Expected 'test-password', got '%s'", result)
	}
	if secret.Value == nil {
		t.Error("Secret should NOT be cleared after String() with AutoClear=false")
	}

	// Call Bytes() - should NOT consume the secret
	resultBytes := secret.Bytes()
	if !bytes.Equal(resultBytes, originalValue) {
		t.Errorf("Expected %v, got %v", originalValue, resultBytes)
	}
	if secret.Value == nil {
		t.Error("Secret should NOT be cleared after Bytes() with AutoClear=false")
	}
}

// TestSecretCopyOnReadBytes tests that Bytes() returns a copy that doesn't affect original
func TestSecretCopyOnReadBytes(t *testing.T) {
	originalValue := []byte("test-password")
	secret := &Secret{
		Value:     make([]byte, len(originalValue)),
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false,
	}
	copy(secret.Value, originalValue)

	// Get bytes copy
	result := secret.Bytes()

	// Modify the returned copy
	result[0] = 'X'

	// Verify original secret value is unchanged
	if !bytes.Equal(secret.Value, originalValue) {
		t.Error("Original secret value should be unchanged when returned copy is modified")
	}

	// Verify they are different slices (different memory locations)
	if len(result) != len(secret.Value) || &result[0] == &secret.Value[0] {
		t.Error("Bytes() should return a copy, not the original slice")
	}
}

// TestSecretCopyOnReadString tests that String() creates a new string
func TestSecretCopyOnReadString(t *testing.T) {
	originalValue := []byte("test-password")
	secret := &Secret{
		Value:     make([]byte, len(originalValue)),
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false,
	}
	copy(secret.Value, originalValue)

	// Get string value
	result := secret.String()

	// Verify string content
	if result != "test-password" {
		t.Errorf("Expected 'test-password', got '%s'", result)
	}

	// Verify original secret value is still intact
	if !bytes.Equal(secret.Value, originalValue) {
		t.Error("Original secret value should be unchanged after String()")
	}
}

// TestSecretStringNilValue tests String() with nil value
func TestSecretStringNilValue(t *testing.T) {
	secret := &Secret{
		Value:     nil,
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false,
	}

	result := secret.String()
	if result != "" {
		t.Errorf("Expected empty string for nil value, got '%s'", result)
	}
}

// TestSecretBytesNilValue tests Bytes() with nil value
func TestSecretBytesNilValue(t *testing.T) {
	secret := &Secret{
		Value:     nil,
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false,
	}

	result := secret.Bytes()
	if result != nil {
		t.Errorf("Expected nil for nil value, got %v", result)
	}
}

// TestSecretUnmarshalJSON tests normal JSON unmarshaling
func TestSecretUnmarshalJSON(t *testing.T) {
	jsonData := `{
		"value": "dGVzdC1wYXNzd29yZA==",
		"version": "v1",
		"created_at": "2023-01-01T00:00:00Z",
		"expires_at": "2023-12-31T23:59:59Z",
		"auto_clear": false
	}`

	secret := &Secret{}
	err := secret.UnmarshalJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	// Verify fields were set correctly
	if !bytes.Equal(secret.Value, []byte("test-password")) {
		t.Errorf("Expected value 'test-password', got %s", secret.Value)
	}
	if secret.Version != "v1" {
		t.Errorf("Expected version 'v1', got '%s'", secret.Version)
	}
	if secret.AutoClear {
		t.Error("AutoClear should be false")
	}
	if secret.ExpiresAt == nil {
		t.Error("ExpiresAt should be set")
	}
}

// TestSecretUnmarshalJSONInvalid tests invalid JSON
func TestSecretUnmarshalJSONInvalid(t *testing.T) {
	invalidJSON := `{"invalid": json}`

	secret := &Secret{}
	err := secret.UnmarshalJSON([]byte(invalidJSON))
	if err == nil {
		t.Error("UnmarshalJSON should fail with invalid JSON")
	}
}

// TestSecretRefStruct tests basic SecretRef struct functionality
func TestSecretRefStruct(t *testing.T) {
	ref := SecretRef{
		Path:     "db/password",
		Version:  "v2",
		Metadata: map[string]string{"env": "prod"},
	}

	if ref.Path != "db/password" {
		t.Errorf("Expected path 'db/password', got '%s'", ref.Path)
	}
	if ref.Version != "v2" {
		t.Errorf("Expected version 'v2', got '%s'", ref.Version)
	}
	if ref.Metadata["env"] != "prod" {
		t.Errorf("Expected metadata env='prod', got '%s'", ref.Metadata["env"])
	}
}

// BenchmarkSecretClear benchmarks the Clear() method
func BenchmarkSecretClear(b *testing.B) {
	secret := &Secret{
		Value:     make([]byte, 1024), // 1KB secret
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		secret.Clear()
		// Reset for next iteration
		secret.Value = make([]byte, 1024)
	}
}

// BenchmarkSecretString benchmarks the String() method
func BenchmarkSecretString(b *testing.B) {
	secret := &Secret{
		Value:     []byte("benchmark-secret-value"),
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = secret.String()
	}
}

// BenchmarkSecretBytes benchmarks the Bytes() method
func BenchmarkSecretBytes(b *testing.B) {
	secret := &Secret{
		Value:     []byte("benchmark-secret-value"),
		Version:   "v1",
		CreatedAt: time.Now(),
		AutoClear: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = secret.Bytes()
		// Reset value since AutoClear is false
		secret.Value = []byte("benchmark-secret-value")
	}
}
