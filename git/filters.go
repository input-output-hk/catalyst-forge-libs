package git

import (
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// Common ChangeFilter functions for filtering diffs

// PathFilter creates a filter that includes changes matching the given path pattern.
// The pattern can include wildcards (* and ?) and is matched against both
// the old and new file names (to handle renames).
func PathFilter(pattern string) ChangeFilter {
	return func(change *object.Change) bool {
		// Check both From and To names to handle renames
		if change.From.Name != "" {
			if matched, _ := filepath.Match(pattern, change.From.Name); matched {
				return true
			}
		}
		if change.To.Name != "" {
			if matched, _ := filepath.Match(pattern, change.To.Name); matched {
				return true
			}
		}
		return false
	}
}

// PathPrefixFilter creates a filter that includes changes with paths starting with the given prefix.
// This is useful for filtering by directory.
func PathPrefixFilter(prefix string) ChangeFilter {
	return func(change *object.Change) bool {
		// Check both From and To names to handle renames
		return strings.HasPrefix(change.From.Name, prefix) ||
			strings.HasPrefix(change.To.Name, prefix)
	}
}

// ExtensionFilter creates a filter that includes changes for files with the given extensions.
// Extensions should include the dot (e.g., ".go", ".js").
func ExtensionFilter(extensions ...string) ChangeFilter {
	// Build a set for O(1) lookup
	extSet := make(map[string]bool)
	for _, ext := range extensions {
		extSet[strings.ToLower(ext)] = true
	}

	return func(change *object.Change) bool {
		// Check both From and To names
		if change.From.Name != "" {
			ext := strings.ToLower(filepath.Ext(change.From.Name))
			if extSet[ext] {
				return true
			}
		}
		if change.To.Name != "" {
			ext := strings.ToLower(filepath.Ext(change.To.Name))
			if extSet[ext] {
				return true
			}
		}
		return false
	}
}

// NonBinaryFilter creates a filter that excludes binary files.
// It uses file extension heuristics to identify binary files.
func NonBinaryFilter() ChangeFilter {
	return func(change *object.Change) bool {
		// Check if either file is likely binary
		if isBinaryPath(change.From.Name) || isBinaryPath(change.To.Name) {
			return false
		}
		return true
	}
}

// MaxSizeFilter creates a filter that excludes files larger than the specified size in bytes.
// Note: This requires fetching the blob objects to check their size, which may impact performance.
func MaxSizeFilter(maxBytes int64) ChangeFilter {
	return func(change *object.Change) bool {
		// For now, we can't easily access file sizes from Change objects
		// This would require accessing the blob objects which adds overhead
		// Return true to not filter based on size for now
		// TODO: Implement size checking if needed by fetching blobs
		return true
	}
}

// AddedFilter creates a filter that only includes newly added files.
func AddedFilter() ChangeFilter {
	return func(change *object.Change) bool {
		return change.From.Name == "" && change.To.Name != ""
	}
}

// DeletedFilter creates a filter that only includes deleted files.
func DeletedFilter() ChangeFilter {
	return func(change *object.Change) bool {
		return change.From.Name != "" && change.To.Name == ""
	}
}

// ModifiedFilter creates a filter that only includes modified files (not added or deleted).
func ModifiedFilter() ChangeFilter {
	return func(change *object.Change) bool {
		return change.From.Name != "" && change.To.Name != "" &&
			change.From.Name == change.To.Name
	}
}

// RenamedFilter creates a filter that only includes renamed/moved files.
func RenamedFilter() ChangeFilter {
	return func(change *object.Change) bool {
		return change.From.Name != "" && change.To.Name != "" &&
			change.From.Name != change.To.Name
	}
}

// AndFilter combines multiple filters with AND logic - all must pass.
func AndFilter(filters ...ChangeFilter) ChangeFilter {
	return func(change *object.Change) bool {
		for _, filter := range filters {
			if filter != nil && !filter(change) {
				return false
			}
		}
		return true
	}
}

// OrFilter combines multiple filters with OR logic - at least one must pass.
func OrFilter(filters ...ChangeFilter) ChangeFilter {
	return func(change *object.Change) bool {
		for _, filter := range filters {
			if filter != nil && filter(change) {
				return true
			}
		}
		return false
	}
}

// NotFilter creates a filter that inverts the result of another filter.
func NotFilter(filter ChangeFilter) ChangeFilter {
	return func(change *object.Change) bool {
		return filter == nil || !filter(change)
	}
}

// CustomFilter allows creating a filter with a custom predicate function.
// This is useful for one-off filtering logic.
func CustomFilter(predicate func(*object.Change) bool) ChangeFilter {
	return ChangeFilter(predicate)
}
