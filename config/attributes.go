package config

import (
	"context"
	"fmt"

	"cuelang.org/go/cue"
	"github.com/input-output-hk/catalyst-forge-libs/cue/attributes"
)

// RepoAttributeProcessor resolves @repo() attributes using repository configuration data.
// It allows CUE templates to reference repository-level configuration values.
type RepoAttributeProcessor struct {
	repo   *RepoConfig
	cueCtx *cue.Context
}

// NewRepoAttributeProcessor creates a new repository attribute processor.
// The processor resolves @repo() attributes by looking up values in the repository configuration.
func NewRepoAttributeProcessor(repo *RepoConfig, cueCtx *cue.Context) *RepoAttributeProcessor {
	return &RepoAttributeProcessor{
		repo:   repo,
		cueCtx: cueCtx,
	}
}

// Name returns the attribute name this processor handles.
func (p *RepoAttributeProcessor) Name() string {
	return "repo"
}

// Process resolves a @repo() attribute and returns the replacement CUE value.
// The attribute should specify a path to a field in the repository configuration.
// Example: @repo(path="tagging.strategy") resolves to the tagging strategy value.
func (p *RepoAttributeProcessor) Process(_ context.Context, attr attributes.Attribute) (cue.Value, error) {
	// Get the path argument
	path, ok := attr.Args["path"]
	if !ok {
		return cue.Value{}, fmt.Errorf("@repo() attribute missing required 'path' argument")
	}

	// Convert the RepoConfig to a CUE value
	repoValue := p.cueCtx.Encode(p.repo.RepoConfig)
	if repoValue.Err() != nil {
		return cue.Value{}, fmt.Errorf("failed to encode repo config: %w", repoValue.Err())
	}

	// Lookup the path in the repository configuration
	result := repoValue.LookupPath(cue.ParsePath(path))
	if result.Err() != nil {
		return cue.Value{}, fmt.Errorf("failed to lookup path %q in repo config: %w", path, result.Err())
	}

	return result, nil
}

// ArtifactAttributeProcessor resolves @artifact() attributes using caller-provided artifact data.
// It falls back to default/placeholder values when artifact data is not available.
// This is useful during validation/discovery phases when actual artifacts don't exist yet.
type ArtifactAttributeProcessor struct {
	artifacts map[string]interface{} // Caller-provided artifact data
	defaults  map[string]interface{} // Default/placeholder values
	cueCtx    *cue.Context
}

// NewArtifactAttributeProcessor creates a new artifact attribute processor.
// The artifacts parameter provides actual artifact data (may be nil for validation/discovery).
// When artifact data is not available, the processor generates default placeholder values.
func NewArtifactAttributeProcessor(artifacts map[string]interface{}, cueCtx *cue.Context) *ArtifactAttributeProcessor {
	return &ArtifactAttributeProcessor{
		artifacts: artifacts,
		defaults:  make(map[string]interface{}),
		cueCtx:    cueCtx,
	}
}

// Name returns the attribute name this processor handles.
func (p *ArtifactAttributeProcessor) Name() string {
	return "artifact"
}

// Process resolves an @artifact() attribute and returns the replacement CUE value.
// The attribute must specify 'name' (artifact name) and 'field' (which field to retrieve).
// Example: @artifact(name="api-server", field="uri") resolves to the artifact's URI.
// If artifact data is not available, returns a type-appropriate placeholder value.
func (p *ArtifactAttributeProcessor) Process(_ context.Context, attr attributes.Attribute) (cue.Value, error) {
	// Get required arguments
	name, ok := attr.Args["name"]
	if !ok {
		return cue.Value{}, fmt.Errorf("@artifact() attribute missing required 'name' argument")
	}

	field, ok := attr.Args["field"]
	if !ok {
		return cue.Value{}, fmt.Errorf("@artifact() attribute missing required 'field' argument")
	}

	// Try to get the value from provided artifacts
	if value, err := p.tryGetArtifactValue(name, field); err == nil {
		return value, nil
	} else if err.Error() != "not found" {
		// Return actual errors (not just "not found")
		return cue.Value{}, err
	}

	// Fall back to default/placeholder value
	defaultValue := GenerateDefaultArtifactValue(name, field)
	result := p.cueCtx.Encode(defaultValue)
	if result.Err() != nil {
		return cue.Value{}, fmt.Errorf("failed to encode default artifact value: %w", result.Err())
	}

	return result, nil
}

// tryGetArtifactValue attempts to retrieve a value from the artifacts map.
// Returns the value and nil error if found.
// Returns zero value and "not found" error if not found.
// Returns zero value and other error if data structure is invalid.
func (p *ArtifactAttributeProcessor) tryGetArtifactValue(name, field string) (cue.Value, error) {
	if p.artifacts == nil {
		return cue.Value{}, fmt.Errorf("not found")
	}

	artifactData, ok := p.artifacts[name]
	if !ok {
		return cue.Value{}, fmt.Errorf("not found")
	}

	// Artifact data should be a map
	artifactMap, ok := artifactData.(map[string]interface{})
	if !ok {
		return cue.Value{}, fmt.Errorf("artifact %q data is not a map", name)
	}

	// Get the requested field
	value, ok := artifactMap[field]
	if !ok {
		return cue.Value{}, fmt.Errorf("not found")
	}

	// Encode the value to CUE
	result := p.cueCtx.Encode(value)
	if result.Err() != nil {
		return cue.Value{}, fmt.Errorf("failed to encode artifact field value: %w", result.Err())
	}

	return result, nil
}

// NewAttributeRegistry creates a registry with both repository and artifact processors registered.
// This is a convenience function for setting up the common case of processing both attribute types.
// Pass nil for artifacts during validation/discovery phases when artifacts don't exist yet.
func NewAttributeRegistry(repo *RepoConfig, artifacts map[string]interface{}, cueCtx *cue.Context) (*attributes.Registry, error) {
	registry := attributes.NewRegistry()

	// Register repository processor
	repoProcessor := NewRepoAttributeProcessor(repo, cueCtx)
	if err := registry.Register(repoProcessor); err != nil {
		return nil, fmt.Errorf("failed to register repo processor: %w", err)
	}

	// Register artifact processor
	artifactProcessor := NewArtifactAttributeProcessor(artifacts, cueCtx)
	if err := registry.Register(artifactProcessor); err != nil {
		return nil, fmt.Errorf("failed to register artifact processor: %w", err)
	}

	return registry, nil
}

// GenerateDefaultArtifactValue creates a type-appropriate placeholder value for an artifact field.
// This is used during validation/discovery when actual artifact data is not available.
// The placeholders are recognizable strings that indicate the field is not yet resolved.
func GenerateDefaultArtifactValue(artifactName string, field string) interface{} {
	// Generate field-specific placeholders
	switch field {
	case "uri", "image", "url":
		return fmt.Sprintf("ARTIFACT_URI_%s", artifactName)
	case "digest", "sha256", "hash":
		return fmt.Sprintf("ARTIFACT_DIGEST_%s", artifactName)
	case "tag", "version":
		return fmt.Sprintf("ARTIFACT_TAG_%s", artifactName)
	case "path", "location":
		return fmt.Sprintf("ARTIFACT_PATH_%s", artifactName)
	default:
		// Generic placeholder for unknown fields
		return fmt.Sprintf("ARTIFACT_%s_%s", artifactName, field)
	}
}
