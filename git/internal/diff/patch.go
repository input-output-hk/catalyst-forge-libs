// Package diff provides utilities for analyzing diff output.
// These utilities help detect binary files and count changed files in patch text.
package diff

import "strings"

// ContainsBinaryFiles checks if a patch contains any binary file changes.
// Binary files are detected by checking the patch string for binary markers.
func ContainsBinaryFiles(patchText string) bool {
	if patchText == "" {
		return false
	}

	// Look for common binary file indicators in git diff output
	return strings.Contains(patchText, "Binary files differ") ||
		strings.Contains(patchText, "GIT binary patch")
}

// CountChangedFiles returns the number of files that have changes in the patch.
// This counts the number of file diff headers in the patch text.
func CountChangedFiles(patchText string) int {
	if patchText == "" {
		return 0
	}

	// Count occurrences of "diff --git" which indicates a file change
	return strings.Count(patchText, "diff --git")
}
