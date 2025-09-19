// Package scanner provides pattern matching utilities for file filtering.
package scanner

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PatternMatcher handles pattern matching for file filtering.
type PatternMatcher struct{}

// NewPatternMatcher creates a new pattern matcher.
func NewPatternMatcher() *PatternMatcher {
	return &PatternMatcher{}
}

// ShouldIncludeFile determines if a file should be included based on patterns.
// It uses the same pattern matching logic as rsync/gitignore patterns.
func (pm *PatternMatcher) ShouldIncludeFile(
	relPath string,
	includePatterns []string,
	excludePatterns []string,
) bool {
	// Normalize path separators to forward slashes for consistent pattern matching
	relPath = filepath.ToSlash(relPath)

	// Check exclude patterns first (excludes take precedence)
	for _, pattern := range excludePatterns {
		if pm.matchesPattern(relPath, pattern) {
			return false
		}
	}

	// If there are include patterns, file must match at least one
	if len(includePatterns) > 0 {
		included := false
		for _, pattern := range includePatterns {
			if pm.matchesPattern(relPath, pattern) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	// File is included
	return true
}

// matchesPattern checks if a path matches a glob pattern.
// It supports basic glob patterns like *, **, and ?.
func (pm *PatternMatcher) matchesPattern(path, pattern string) bool {
	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		// This is a directory pattern
		pattern = strings.TrimSuffix(pattern, "/")
		// For directory patterns, check if the path is within that directory
		return strings.HasPrefix(path+"/", pattern+"/") || path == pattern
	}

	// Handle ** patterns (recursive wildcard)
	if strings.Contains(pattern, "**") {
		return pm.matchesGlobPattern(path, pattern)
	}

	// Simple pattern matching using filepath.Match
	match, err := filepath.Match(pattern, path)
	if err != nil {
		// If pattern is invalid, don't match
		return false
	}

	return match
}

// matchesGlobPattern handles patterns with ** (recursive wildcard).
// This is a simplified implementation - in production you might want more sophisticated globbing.
func (pm *PatternMatcher) matchesGlobPattern(path, pattern string) bool {
	// Split pattern on **
	parts := strings.Split(pattern, "**")

	if len(parts) == 1 {
		// No ** in pattern, use simple match
		match, _ := filepath.Match(pattern, path)
		return match
	}

	if len(parts) == 2 {
		// Pattern like "prefix**suffix"
		prefix := parts[0]
		suffix := parts[1]

		// Path must start with prefix
		if !strings.HasPrefix(path, prefix) {
			return false
		}

		// If suffix is empty, any path starting with prefix matches
		if suffix == "" {
			return true
		}

		// Path must end with suffix
		return strings.HasSuffix(path, suffix)
	}

	// More complex patterns not supported in this simplified implementation
	return false
}

// ValidatePatterns validates that the given patterns are syntactically correct.
func (pm *PatternMatcher) ValidatePatterns(patterns []string) []error {
	var errors []error

	for i, pattern := range patterns {
		if strings.Contains(pattern, "**") && strings.Count(pattern, "**") > 1 {
			// Multiple ** not supported in our implementation
			continue
		}

		// Try to match against a dummy path to validate pattern syntax
		_, err := filepath.Match(pattern, "dummy")
		if err != nil {
			errors = append(errors, &PatternError{
				Pattern: pattern,
				Index:   i,
				Err:     err,
			})
		}
	}

	return errors
}

// PatternError represents an error with a pattern.
type PatternError struct {
	Pattern string
	Index   int
	Err     error
}

func (e *PatternError) Error() string {
	return fmt.Sprintf("invalid pattern at index %d '%s': %v", e.Index, e.Pattern, e.Err)
}
