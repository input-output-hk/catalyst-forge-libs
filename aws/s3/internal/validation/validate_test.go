package validation

import (
	"strings"
	"testing"
)

func TestValidateBucketName(t *testing.T) {
	tests := []struct {
		name      string
		bucket    string
		wantError bool
		errMsg    string
	}{
		// Valid bucket names
		{"valid_simple", "my-bucket", false, ""},
		{"valid_with_numbers", "my-bucket123", false, ""},
		{"valid_with_dots", "my.bucket", false, ""},
		{"valid_with_hyphens", "my-bucket-name", false, ""},
		{"valid_min_length", "abc", false, ""},
		{"valid_max_length", strings.Repeat("a", 63), false, ""},

		// Invalid bucket names
		{"empty", "", true, "bucket name cannot be empty"},
		{"too_short", "ab", true, "bucket name must be between 3 and 63 characters long"},
		{
			"too_long",
			strings.Repeat("a", 64),
			true,
			"bucket name must be between 3 and 63 characters long",
		},
		{
			"starts_with_hyphen",
			"-bucket",
			true,
			"bucket name cannot start or end with a hyphen or dot",
		},
		{
			"ends_with_hyphen",
			"bucket-",
			true,
			"bucket name cannot start or end with a hyphen or dot",
		},
		{
			"starts_with_dot",
			".bucket",
			true,
			"bucket name cannot start or end with a hyphen or dot",
		},
		{"ends_with_dot", "bucket.", true, "bucket name cannot start or end with a hyphen or dot"},
		{
			"contains_uppercase",
			"MyBucket",
			true,
			"bucket name can only contain lowercase letters, numbers, dots, and hyphens",
		},
		{
			"contains_underscore",
			"my_bucket",
			true,
			"bucket name can only contain lowercase letters, numbers, dots, and hyphens",
		},
		{
			"contains_space",
			"my bucket",
			true,
			"bucket name can only contain lowercase letters, numbers, dots, and hyphens",
		},
		{"starts_with_number", "1bucket", true, "bucket name cannot start with a number"},
		{"ip_address", "192.168.1.1", true, "bucket name cannot be formatted as an IP address"},
		{"localhost", "localhost", true, "bucket name cannot be a reserved word"},
		{
			"double_dots",
			"my..bucket",
			true,
			"bucket name cannot contain two adjacent periods or hyphens",
		},
		{
			"double_hyphens",
			"my--bucket",
			true,
			"bucket name cannot contain two adjacent periods or hyphens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBucketName(tt.bucket)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateBucketName(%q) expected error, got nil", tt.bucket)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateBucketName(%q) error = %q, want to contain %q", tt.bucket, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateBucketName(%q) expected no error, got %q", tt.bucket, err)
				}
			}
		})
	}
}

func TestValidateObjectKey(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantError bool
		errMsg    string
	}{
		// Valid object keys
		{"valid_simple", "my-file.txt", false, ""},
		{"valid_with_path", "folder/subfolder/file.txt", false, ""},
		{"valid_unicode", "файл.txt", false, ""},
		{"valid_numbers", "file123.txt", false, ""},
		{"valid_special_chars", "file_with-dashes.and.dots.txt", false, ""},
		{"valid_spaces", "file with spaces.txt", false, ""},

		// Invalid object keys
		{"empty", "", true, "object key cannot be empty"},
		{"too_long", strings.Repeat("a", 1025), true, "object key cannot exceed 1024 characters"},
		{
			"path_traversal_dot_dot",
			"../secret.txt",
			true,
			"object key cannot contain path traversal sequences",
		},
		{
			"path_traversal_dot_dot_path",
			"folder/../../../secret.txt",
			true,
			"object key cannot contain path traversal sequences",
		},
		{
			"path_traversal_absolute",
			"/etc/passwd",
			true,
			"object key cannot contain path traversal sequences",
		},
		{
			"path_traversal_windows",
			"C:\\Windows\\System32\\config\\system",
			true,
			"object key cannot contain path traversal sequences",
		},
		{
			"control_characters",
			"file\x00with\x01null.txt",
			true,
			"object key cannot contain control characters",
		},
		{
			"newline",
			"file\nwith\nnewlines.txt",
			true,
			"object key cannot contain control characters",
		},
		{"tab", "file\twith\ttabs.txt", true, "object key cannot contain control characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateObjectKey(tt.key)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateObjectKey(%q) expected error, got nil", tt.key)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateObjectKey(%q) error = %q, want to contain %q", tt.key, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateObjectKey(%q) expected no error, got %q", tt.key, err)
				}
			}
		})
	}
}

func TestSanitizeMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "nil_metadata",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty_metadata",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "remove_control_chars",
			input: map[string]string{
				"key1": "value\x00with\x01null",
				"key2": "value\nwith\nnewlines",
				"key3": "value\twith\ttabs",
			},
			expected: map[string]string{
				"key1": "valuewithnull",
				"key2": "value\nwith\nnewlines",
				"key3": "value\twith\ttabs",
			},
		},
		{
			name: "sanitize_keys_and_values",
			input: map[string]string{
				"Key\x01With\x02Control": "Value\x03With\x04Control",
				"Normal-Key":             "Normal Value",
			},
			expected: map[string]string{
				"KeyWithControl": "ValueWithControl",
				"Normal-Key":     "Normal Value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeMetadata(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf(
					"SanitizeMetadata() length mismatch: got %d, want %d",
					len(result),
					len(tt.expected),
				)
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("SanitizeMetadata()[%q] = %q, want %q", k, result[k], v)
				}
			}
		})
	}
}

func TestValidateMetadata(t *testing.T) {
	tests := []struct {
		name      string
		metadata  map[string]string
		wantError bool
		errMsg    string
	}{
		{
			name:      "nil_metadata",
			metadata:  nil,
			wantError: false,
		},
		{
			name:      "empty_metadata",
			metadata:  map[string]string{},
			wantError: false,
		},
		{
			name: "valid_metadata",
			metadata: map[string]string{
				"content-type":    "application/json",
				"cache-control":   "max-age=3600",
				"custom-metadata": "some-value",
			},
			wantError: false,
		},
		{
			name: "too_long_value",
			metadata: map[string]string{
				"long-value": strings.Repeat("a", 2049),
			},
			wantError: true,
			errMsg:    "metadata value cannot exceed 2048 characters",
		},
		{
			name: "aws_reserved_prefix",
			metadata: map[string]string{
				"aws:some-key": "value",
			},
			wantError: true,
			errMsg:    "metadata key cannot start with reserved prefix",
		},
		{
			name: "x_amz_reserved_prefix",
			metadata: map[string]string{
				"x-amz-meta-custom": "value",
			},
			wantError: true,
			errMsg:    "metadata key cannot start with reserved prefix",
		},
		{
			name: "control_characters_in_value",
			metadata: map[string]string{
				"key": "value\x00with\x01null",
			},
			wantError: true,
			errMsg:    "metadata value can only contain printable characters",
		},
		{
			name: "empty_key",
			metadata: map[string]string{
				"": "value",
			},
			wantError: true,
			errMsg:    "metadata key cannot be empty",
		},
		{
			name: "too_long_key",
			metadata: map[string]string{
				strings.Repeat("a", 129): "value",
			},
			wantError: true,
			errMsg:    "metadata key cannot exceed 128 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMetadata(tt.metadata)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateMetadata() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateMetadata() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateMetadata() expected no error, got %q", err)
				}
			}
		})
	}
}

func TestValidateContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantError   bool
		errMsg      string
	}{
		{"empty", "", false, ""},
		{"valid_mime", "application/json", false, ""},
		{"valid_with_params", "text/plain; charset=utf-8", false, ""},
		{"invalid_mime", "invalid/mime/type/extra", true, "content type must be a valid MIME type"},
		{
			"dangerous_flash",
			"application/x-shockwave-flash",
			true,
			"content type is not allowed for security reasons",
		},
		{
			"dangerous_jar",
			"application/java-archive",
			true,
			"content type is not allowed for security reasons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContentType(tt.contentType)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateContentType(%q) expected error, got nil", tt.contentType)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateContentType(%q) error = %q, want to contain %q", tt.contentType, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateContentType(%q) expected no error, got %q", tt.contentType, err)
				}
			}
		})
	}
}

func TestValidateACL(t *testing.T) {
	tests := []struct {
		name      string
		acl       string
		wantError bool
		errMsg    string
	}{
		{"empty", "", false, ""},
		{"private", "private", false, ""},
		{"public_read", "public-read", false, ""},
		{"public_read_write", "public-read-write", false, ""},
		{"authenticated_read", "authenticated-read", false, ""},
		{"aws_exec_read", "aws-exec-read", false, ""},
		{"bucket_owner_read", "bucket-owner-read", false, ""},
		{"bucket_owner_full_control", "bucket-owner-full-control", false, ""},
		{"invalid_acl", "invalid-acl", true, "ACL must be one of:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateACL(tt.acl)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateACL(%q) expected error, got nil", tt.acl)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateACL(%q) error = %q, want to contain %q", tt.acl, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateACL(%q) expected no error, got %q", tt.acl, err)
				}
			}
		})
	}
}

// Test edge cases and security scenarios
func TestSecurityValidation(t *testing.T) {
	t.Run("path_traversal_variations", func(t *testing.T) {
		traversalKeys := []string{
			"..",
			"../",
			"/..",
			"folder/..",
			"folder/../",
			"../../../etc/passwd",
			"..\\..\\..\\windows\\system32\\config\\system",
			"C:\\Windows\\System32",
			"/etc/passwd",
			"/absolute/path",
		}

		for _, key := range traversalKeys {
			err := ValidateObjectKey(key)
			if err == nil {
				t.Errorf("ValidateObjectKey(%q) should reject path traversal attempt", key)
			} else if !strings.Contains(err.Error(), "path traversal") {
				t.Errorf("ValidateObjectKey(%q) error should mention path traversal, got: %s", key, err.Error())
			}
		}
	})

	t.Run("control_character_detection", func(t *testing.T) {
		// Test all control characters except newline and tab (which are allowed in metadata)
		for i := 0; i < 32; i++ {
			if i == '\n' || i == '\t' {
				continue
			}
			key := "file" + string(rune(i)) + "test.txt"
			err := ValidateObjectKey(key)
			if err == nil {
				t.Errorf("ValidateObjectKey(%q) should reject control character %d", key, i)
			}
		}

		// Test DEL character
		err := ValidateObjectKey("file\x7fdel.txt")
		if err == nil {
			t.Errorf("ValidateObjectKey should reject DEL character")
		}
	})

	t.Run("metadata_injection_prevention", func(t *testing.T) {
		dangerousMetadata := map[string]string{
			"x-amz-meta-injection": "<script>alert('xss')</script>",
			"aws:reserved":         "injected-value",
			"":                     "empty-key",
		}

		err := ValidateMetadata(dangerousMetadata)
		if err == nil {
			t.Errorf("ValidateMetadata should reject dangerous metadata")
		}
	})
}

// Benchmark tests for performance
func BenchmarkValidateBucketName(b *testing.B) {
	validBuckets := []string{
		"my-bucket",
		"test-bucket-123",
		"my.bucket.name",
		"valid-bucket-name-with-dashes",
	}

	for _, bucket := range validBuckets {
		b.Run("valid_"+bucket, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = ValidateBucketName(bucket) // Error ignored for benchmark performance
			}
		})
	}
}

func BenchmarkValidateObjectKey(b *testing.B) {
	validKeys := []string{
		"simple-file.txt",
		"folder/subfolder/deep/nested/file.txt",
		"file-with-dashes-and.dots.txt",
		"unicode-文件名.txt",
	}

	for _, key := range validKeys {
		b.Run("valid_"+key, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = ValidateObjectKey(key) // Error ignored for benchmark performance
			}
		})
	}
}

func BenchmarkSanitizeMetadata(b *testing.B) {
	metadata := map[string]string{
		"content-type":   "application/json",
		"cache-control":  "max-age=3600",
		"custom-header":  "value-with-control\x00chars",
		"another-header": "another-value",
		"long-value":     strings.Repeat("a", 100),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SanitizeMetadata(metadata)
	}
}
