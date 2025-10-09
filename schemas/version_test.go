package schema

import (
	"testing"
)

func TestIsCompatible_ValidVersions(t *testing.T) {
	tests := []struct {
		name        string
		userVersion string
		want        bool
	}{
		// Compatible versions
		{"exact match", "0.1.0", true},
		{"patch version higher", "0.1.5", true},
		{"build metadata same version", "0.1.0+build", true},
		{"build metadata higher patch", "0.1.5+20231201", true},

		// Incompatible - minor version changes
		{"minor version higher", "0.2.0", false},
		{"minor and patch version higher", "0.2.3", false},
		{"build metadata higher minor", "0.2.0+build.123", false},

		// Incompatible - major version changes
		{"major version higher", "1.0.0", false},
		{"major version higher with patch", "1.0.1", false},
		{"major version much higher", "2.0.0", false},
		{"short format major only", "1", false},
		{"short format major.minor", "1.2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsCompatible(tt.userVersion)
			if err != nil {
				t.Errorf("IsCompatible() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("IsCompatible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCompatible_PreReleaseVersions(t *testing.T) {
	tests := []struct {
		name        string
		userVersion string
		want        bool
	}{
		{"pre-release same base version", "0.1.0-alpha", false},
		{"pre-release higher patch", "0.1.5-beta", false},
		{"pre-release higher minor", "0.2.0-rc.1", false},
		{"pre-release with build metadata", "0.1.0-alpha+build", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsCompatible(tt.userVersion)
			if err != nil {
				t.Errorf("IsCompatible() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("IsCompatible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCompatible_InvalidVersions(t *testing.T) {
	tests := []struct {
		name        string
		userVersion string
	}{
		{"invalid format - letters", "abc"},
		{"invalid format - too many segments", "1.2.3.4"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsCompatible(tt.userVersion)
			if err == nil {
				t.Errorf("IsCompatible() expected error, got nil (result: %v)", got)
				return
			}
			if got {
				t.Errorf("IsCompatible() = %v, want false on error", got)
			}
		})
	}
}

func TestSchemaVersion(t *testing.T) {
	// Verify that SchemaVersion is a valid semantic version
	if SchemaVersion != "0.1.0" {
		t.Errorf("SchemaVersion = %q, want %q", SchemaVersion, "0.1.0")
	}

	// Verify that SchemaVersion can be used to create a valid constraint
	compatible, err := IsCompatible(SchemaVersion)
	if err != nil {
		t.Errorf("SchemaVersion %q should be valid, got error: %v", SchemaVersion, err)
	}
	if !compatible {
		t.Errorf("SchemaVersion should be compatible with itself")
	}
}
