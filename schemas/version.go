package schema

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// SchemaVersion is the current schema version.
// User configurations declare their forgeVersion to indicate compatibility.
const SchemaVersion = "0.1.0"

// IsCompatible checks if a user's forgeVersion is compatible with SchemaVersion.
// Uses caret constraint (^) for semantic version compatibility.
//
// For version 0.x.y, caret constraint allows only patch version changes:
//   - Same major and minor version with different patch (0.1.0, 0.1.1, 0.1.5, etc.)
//   - Does NOT allow minor version changes (0.2.0, 0.3.0 are incompatible)
//   - Does NOT allow major version changes (1.0.0 is incompatible)
//
// This is because in 0.x versions, minor version changes are considered
// potentially breaking according to semantic versioning.
//
// Returns true if the versions are compatible according to semantic versioning rules.
// Returns false (with no error) if versions are incompatible.
// Returns an error if either version string is invalid.
func IsCompatible(userVersion string) (bool, error) {
	// Create constraint: ^SchemaVersion (compatible with same major version)
	constraint, err := semver.NewConstraint("^" + SchemaVersion)
	if err != nil {
		return false, fmt.Errorf("invalid schema version: %w", err)
	}

	// Parse user version
	v, err := semver.NewVersion(userVersion)
	if err != nil {
		return false, fmt.Errorf("invalid user version %q: %w", userVersion, err)
	}

	// Check compatibility
	return constraint.Check(v), nil
}
