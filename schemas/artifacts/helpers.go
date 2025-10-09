// Package artifacts provides helper methods for working with artifact discriminated unions.
package artifacts

import (
	"encoding/json"
	"fmt"
)

// Type returns the discriminator value for this ArtifactSpec.
// Returns an empty string if the type field is missing or not a string.
func (as ArtifactSpec) Type() string {
	if t, ok := as["type"].(string); ok {
		return t
	}
	return ""
}

// AsContainer attempts to convert the ArtifactSpec to a ContainerArtifact.
// Returns the artifact and true if successful, nil and false otherwise.
func (as ArtifactSpec) AsContainer() (*ContainerArtifact, bool) {
	if as.Type() != "container" {
		return nil, false
	}

	// Re-marshal and unmarshal to convert map[string]any to ContainerArtifact
	data, err := json.Marshal(as)
	if err != nil {
		return nil, false
	}

	var container ContainerArtifact
	if err := json.Unmarshal(data, &container); err != nil {
		return nil, false
	}

	return &container, true
}

// AsBinary attempts to convert the ArtifactSpec to a BinaryArtifact.
// Returns the artifact and true if successful, nil and false otherwise.
func (as ArtifactSpec) AsBinary() (*BinaryArtifact, bool) {
	if as.Type() != "binary" {
		return nil, false
	}

	data, err := json.Marshal(as)
	if err != nil {
		return nil, false
	}

	var binary BinaryArtifact
	if err := json.Unmarshal(data, &binary); err != nil {
		return nil, false
	}

	return &binary, true
}

// AsArchive attempts to convert the ArtifactSpec to an ArchiveArtifact.
// Returns the artifact and true if successful, nil and false otherwise.
func (as ArtifactSpec) AsArchive() (*ArchiveArtifact, bool) {
	if as.Type() != "archive" {
		return nil, false
	}

	data, err := json.Marshal(as)
	if err != nil {
		return nil, false
	}

	var archive ArchiveArtifact
	if err := json.Unmarshal(data, &archive); err != nil {
		return nil, false
	}

	return &archive, true
}

// Validate checks if the ArtifactSpec has a valid type discriminator.
// Returns an error if the type is missing or unknown.
func (as ArtifactSpec) Validate() error {
	typ := as.Type()
	if typ == "" {
		return fmt.Errorf("artifact spec missing 'type' field")
	}

	switch typ {
	case "container", "binary", "archive":
		return nil
	default:
		return fmt.Errorf("unknown artifact type: %q", typ)
	}
}
