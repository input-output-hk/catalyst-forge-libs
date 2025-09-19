// Package validation provides centralized input validation logic.
// This includes bucket name validation, object key validation, and security checks.
//
// All user inputs are validated before being sent to AWS to prevent
// injection attacks and ensure compliance with S3 requirements.
package validation

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
)

// ValidateBucketName validates that a bucket name is DNS-compliant according to AWS S3 rules.
// Returns ErrInvalidBucketName if the bucket name is invalid.
func ValidateBucketName(bucket string) error {
	if err := validateBucketNameBasics(bucket); err != nil {
		return err
	}

	if err := validateBucketNameCharacters(bucket); err != nil {
		return err
	}

	if err := validateBucketNameStructure(bucket); err != nil {
		return err
	}

	return nil
}

// ValidateObjectKey validates that an object key is valid according to AWS S3 rules.
// This includes preventing path traversal attacks and ensuring valid characters.
func ValidateObjectKey(key string) error {
	if key == "" {
		return errors.NewError("validateObjectKey", errors.ErrInvalidObjectKey).
			WithKey(key).
			WithMessage("object key cannot be empty")
	}

	// Check for path traversal attempts
	if hasPathTraversal(key) {
		return errors.NewError("validateObjectKey", errors.ErrInvalidObjectKey).
			WithKey(key).
			WithMessage("object key cannot contain path traversal sequences")
	}

	// Validate key length (S3 supports up to 1024 bytes)
	if len(key) > 1024 {
		return errors.NewError("validateObjectKey", errors.ErrInvalidObjectKey).
			WithKey(key).
			WithMessage("object key cannot exceed 1024 characters")
	}

	// Validate characters - S3 keys can contain any UTF-8 character
	// but we should prevent control characters
	if hasControlCharacters(key) {
		return errors.NewError("validateObjectKey", errors.ErrInvalidObjectKey).
			WithKey(key).
			WithMessage("object key cannot contain control characters")
	}

	return nil
}

// SanitizeMetadata sanitizes metadata values to prevent injection attacks.
// This removes or escapes potentially dangerous characters.
func SanitizeMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}

	sanitized := make(map[string]string, len(metadata))
	for key, value := range metadata {
		sanitized[sanitizeMetadataKey(key)] = sanitizeMetadataValue(value)
	}

	return sanitized
}

// ValidateMetadata validates metadata keys and values according to S3 rules.
func ValidateMetadata(metadata map[string]string) error {
	if metadata == nil {
		return nil
	}

	for key, value := range metadata {
		if err := validateMetadataKey(key); err != nil {
			return err
		}
		if err := validateMetadataValue(value); err != nil {
			return err
		}
	}

	return nil
}

// validateBucketNameBasics validates basic bucket name requirements
func validateBucketNameBasics(bucket string) error {
	if bucket == "" {
		return errors.NewError("validateBucketName", errors.ErrInvalidBucketName).
			WithBucket(bucket).
			WithMessage("bucket name cannot be empty")
	}

	// Bucket names must be between 3 and 63 characters long
	if len(bucket) < 3 || len(bucket) > 63 {
		return errors.NewError("validateBucketName", errors.ErrInvalidBucketName).
			WithBucket(bucket).
			WithMessage("bucket name must be between 3 and 63 characters long")
	}

	return nil
}

// validateBucketNameCharacters validates allowed characters in bucket names
func validateBucketNameCharacters(bucket string) error {
	// Bucket names can consist only of lowercase letters, numbers, dots (.), and hyphens (-)
	for _, char := range bucket {
		if !isValidBucketChar(char) {
			return errors.NewError("validateBucketName", errors.ErrInvalidBucketName).
				WithBucket(bucket).
				WithMessage("bucket name can only contain lowercase letters, numbers, dots, and hyphens")
		}
	}

	return nil
}

// validateBucketNameStructure validates bucket name structural requirements
func validateBucketNameStructure(bucket string) error {
	// Bucket names must not start or end with a hyphen or dot
	if bucket[0] == '-' || bucket[0] == '.' || bucket[len(bucket)-1] == '-' || bucket[len(bucket)-1] == '.' {
		return errors.NewError("validateBucketName", errors.ErrInvalidBucketName).
			WithBucket(bucket).
			WithMessage("bucket name cannot start or end with a hyphen or dot")
	}

	// Bucket names cannot be formatted as an IP address (check before number check)
	if isIPAddress(bucket) {
		return errors.NewError("validateBucketName", errors.ErrInvalidBucketName).
			WithBucket(bucket).
			WithMessage("bucket name cannot be formatted as an IP address")
	}

	// Bucket names cannot start with a number
	if bucket[0] >= '0' && bucket[0] <= '9' {
		return errors.NewError("validateBucketName", errors.ErrInvalidBucketName).
			WithBucket(bucket).
			WithMessage("bucket name cannot start with a number")
	}

	// Bucket names cannot contain two adjacent periods or hyphens
	if hasAdjacentSpecialChars(bucket) {
		return errors.NewError("validateBucketName", errors.ErrInvalidBucketName).
			WithBucket(bucket).
			WithMessage("bucket name cannot contain two adjacent periods or hyphens")
	}

	// Bucket names cannot be reserved words
	if isReservedWord(bucket) {
		return errors.NewError("validateBucketName", errors.ErrInvalidBucketName).
			WithBucket(bucket).
			WithMessage("bucket name cannot be a reserved word")
	}

	return nil
}

// isValidBucketChar checks if a character is valid in a bucket name
func isValidBucketChar(char rune) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'z') || char == '.' || char == '-'
}

// hasAdjacentSpecialChars checks for adjacent special characters
func hasAdjacentSpecialChars(bucket string) bool {
	for i := 0; i < len(bucket)-1; i++ {
		if (bucket[i] == '.' && bucket[i+1] == '.') || (bucket[i] == '-' && bucket[i+1] == '-') {
			return true
		}
	}
	return false
}

// isReservedWord checks if bucket name is a reserved word
func isReservedWord(bucket string) bool {
	reservedWords := []string{"localhost"}
	for _, word := range reservedWords {
		if bucket == word {
			return true
		}
	}
	return false
}

// isIPAddress checks if a string is formatted as an IP address
func isIPAddress(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if len(part) == 0 {
			return true // Empty part indicates IP-like format (e.g., "192.168..1")
		}
		// Check if each part is a valid number 0-255
		num := 0
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
			num = num*10 + int(char-'0')
		}
		if num > 255 {
			return false
		}
	}

	return true
}

// hasPathTraversal checks for path traversal attempts in object keys
func hasPathTraversal(key string) bool {
	// Check for obvious traversal patterns
	if strings.Contains(key, "..") {
		return true
	}

	// Use filepath.Clean to normalize the path and check for traversal
	cleaned := filepath.Clean(key)

	// If the cleaned path starts with "..", it's a traversal attempt
	if strings.HasPrefix(cleaned, "..") {
		return true
	}

	// Check for absolute path attempts
	if strings.HasPrefix(cleaned, "/") {
		return true
	}

	// Check for Windows-style absolute paths
	if len(cleaned) >= 3 && cleaned[1] == ':' && (cleaned[2] == '\\' || cleaned[2] == '/') {
		return true
	}

	return false
}

// hasControlCharacters checks for control characters in the key
func hasControlCharacters(key string) bool {
	for _, char := range key {
		if unicode.IsControl(char) {
			return true
		}
	}
	return false
}

// sanitizeMetadataKey sanitizes metadata keys
func sanitizeMetadataKey(key string) string {
	// Remove any non-printable characters
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, key)
}

// sanitizeMetadataValue sanitizes metadata values
func sanitizeMetadataValue(value string) string {
	// Remove any control characters but keep newlines and tabs for multi-line values
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, value)
}

// validateMetadataKey validates a metadata key according to S3 rules
func validateMetadataKey(key string) error {
	if key == "" {
		return errors.NewError("validateMetadata", errors.ErrInvalidInput).
			WithMessage("metadata key cannot be empty")
	}

	// S3 metadata keys have restrictions
	if len(key) > 128 {
		return errors.NewError("validateMetadata", errors.ErrInvalidInput).
			WithMessage("metadata key cannot exceed 128 characters")
	}

	// Keys cannot start with certain prefixes that are reserved by AWS
	reservedPrefixes := []string{"aws:", "x-amz-", "x-amz:"}
	for _, prefix := range reservedPrefixes {
		if strings.HasPrefix(strings.ToLower(key), prefix) {
			return errors.NewError("validateMetadata", errors.ErrInvalidInput).
				WithMessage(fmt.Sprintf("metadata key cannot start with reserved prefix: %s", prefix))
		}
	}

	// Keys can only contain printable ASCII characters except for spaces
	for _, char := range key {
		if char < 32 || char > 126 {
			return errors.NewError("validateMetadata", errors.ErrInvalidInput).
				WithMessage("metadata key can only contain printable ASCII characters")
		}
	}

	return nil
}

// validateMetadataValue validates a metadata value according to S3 rules
func validateMetadataValue(value string) error {
	// S3 metadata values can be up to 2KB
	if len(value) > 2048 {
		return errors.NewError("validateMetadata", errors.ErrInvalidInput).
			WithMessage("metadata value cannot exceed 2048 characters")
	}

	// Values can contain any printable characters
	for _, char := range value {
		if !unicode.IsPrint(char) && char != '\n' && char != '\t' {
			return errors.NewError("validateMetadata", errors.ErrInvalidInput).
				WithMessage("metadata value can only contain printable characters")
		}
	}

	return nil
}

// ValidateContentType validates that a content type is safe and valid
func ValidateContentType(contentType string) error {
	if contentType == "" {
		return nil // Empty content type is allowed
	}

	// Basic MIME type pattern validation
	mimePattern := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-+]*\/[a-zA-Z0-9][a-zA-Z0-9\-+]*(\s*;.*)?$`)
	if !mimePattern.MatchString(contentType) {
		return errors.NewError("validateContentType", errors.ErrInvalidInput).
			WithMessage("content type must be a valid MIME type")
	}

	// Prevent potentially dangerous content types
	dangerousTypes := []string{
		"application/x-shockwave-flash",
		"application/java-archive",
		"application/x-java-archive",
	}

	contentTypeLower := strings.ToLower(strings.Split(contentType, ";")[0])
	for _, dangerous := range dangerousTypes {
		if contentTypeLower == dangerous {
			return errors.NewError("validateContentType", errors.ErrInvalidInput).
				WithMessage("content type is not allowed for security reasons")
		}
	}

	return nil
}

// ValidateACL validates that an ACL value is valid
func ValidateACL(acl string) error {
	if acl == "" {
		return nil // Empty ACL defaults to private
	}

	validACLs := map[string]bool{
		"private":                   true,
		"public-read":               true,
		"public-read-write":         true,
		"authenticated-read":        true,
		"aws-exec-read":             true,
		"bucket-owner-read":         true,
		"bucket-owner-full-control": true,
	}

	if !validACLs[acl] {
		return errors.NewError("validateACL", errors.ErrInvalidInput).
			WithMessage("ACL must be one of: private, public-read, public-read-write, authenticated-read, aws-exec-read, bucket-owner-read, bucket-owner-full-control")
	}

	return nil
}
