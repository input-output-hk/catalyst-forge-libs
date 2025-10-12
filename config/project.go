package config

import (
	"fmt"
	"sort"

	"github.com/input-output-hk/catalyst-forge-libs/schema"
	"github.com/input-output-hk/catalyst-forge-libs/schema/artifacts"
)

// ProjectConfig wraps schema.ProjectConfig with helper methods for convenient access
// to project configuration data. All methods are read-only; configurations
// are immutable after loading.
type ProjectConfig struct {
	*schema.ProjectConfig // Embedded for direct access to all schema fields
}

// Phase participation helper methods

// ParticipatesIn checks if the project participates in the specified phase.
// Returns true if the project has steps defined for the phase, false otherwise.
func (p *ProjectConfig) ParticipatesIn(phase string) bool {
	if p.ProjectConfig == nil || p.Phases == nil {
		return false
	}
	_, ok := p.Phases[phase]
	return ok
}

// GetPhaseSteps retrieves the steps for a specific phase.
// Returns the steps and true if found, or nil and false if the phase is not found.
func (p *ProjectConfig) GetPhaseSteps(phase string) ([]schema.Step, bool) {
	if p.ProjectConfig == nil || p.Phases == nil {
		return nil, false
	}
	participation, ok := p.Phases[phase]
	if !ok {
		return nil, false
	}
	return participation.Steps, true
}

// ListParticipatingPhases returns a sorted list of all phase names the project participates in.
// The list is sorted alphabetically for deterministic output.
func (p *ProjectConfig) ListParticipatingPhases() []string {
	if p.ProjectConfig == nil || p.Phases == nil {
		return []string{}
	}
	names := make([]string, 0, len(p.Phases))
	for name := range p.Phases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Artifact helper methods

// GetArtifact retrieves an artifact specification by name.
// Returns the artifact specification and true if found, or nil and false if not found.
func (p *ProjectConfig) GetArtifact(name string) (artifacts.ArtifactSpec, bool) {
	if p.Artifacts == nil {
		return nil, false
	}
	artifact, ok := p.Artifacts[name]
	return artifact, ok
}

// ListArtifacts returns a sorted list of all artifact names defined in the project.
// The list is sorted alphabetically for deterministic output.
func (p *ProjectConfig) ListArtifacts() []string {
	if p.Artifacts == nil {
		return []string{}
	}
	names := make([]string, 0, len(p.Artifacts))
	for name := range p.Artifacts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HasArtifact checks if an artifact with the given name exists.
func (p *ProjectConfig) HasArtifact(name string) bool {
	if p.Artifacts == nil {
		return false
	}
	_, ok := p.Artifacts[name]
	return ok
}

// GetArtifactPublishers extracts the list of publisher names for a specific artifact.
// Returns the publisher names and nil error if successful.
// Returns an error if the artifact is not found or if the artifact structure is invalid.
func (p *ProjectConfig) GetArtifactPublishers(artifactName string) ([]string, error) {
	artifact, ok := p.GetArtifact(artifactName)
	if !ok {
		return nil, fmt.Errorf("artifact %q not found", artifactName)
	}

	// Extract publishers field from the artifact map
	publishersVal, ok := artifact["publishers"]
	if !ok {
		// No publishers defined is valid (empty list)
		return []string{}, nil
	}

	// Convert to slice of strings
	publishersSlice, ok := publishersVal.([]interface{})
	if !ok {
		return nil, fmt.Errorf("artifact %q has invalid publishers field type", artifactName)
	}

	publishers := make([]string, 0, len(publishersSlice))
	for i, pub := range publishersSlice {
		pubStr, ok := pub.(string)
		if !ok {
			return nil, fmt.Errorf("artifact %q publishers[%d] is not a string", artifactName, i)
		}
		publishers = append(publishers, pubStr)
	}

	return publishers, nil
}

// Release configuration helper methods

// HasRelease checks if the project has release configuration defined.
func (p *ProjectConfig) HasRelease() bool {
	if p.ProjectConfig == nil {
		return false
	}
	return p.Release != nil
}

// GetReleaseTriggers retrieves the release configuration if defined.
// Returns the release configuration and true if defined, or nil and false if not defined.
func (p *ProjectConfig) GetReleaseTriggers() (*schema.ReleaseConfig, bool) {
	if p.ProjectConfig == nil || p.Release == nil {
		return nil, false
	}
	return p.Release, true
}

// Deployment configuration helper methods

// HasDeployment checks if the project has deployment configuration defined.
func (p *ProjectConfig) HasDeployment() bool {
	if p.ProjectConfig == nil {
		return false
	}
	return p.Deploy != nil
}

// GetDeploymentConfig retrieves the deployment configuration if defined.
// Returns the deployment configuration and true if defined, or nil and false if not defined.
func (p *ProjectConfig) GetDeploymentConfig() (*schema.DeploymentConfig, bool) {
	if p.ProjectConfig == nil || p.Deploy == nil {
		return nil, false
	}
	return p.Deploy, true
}

// Validate runs comprehensive validation checks on the project configuration.
// This requires repository context to validate phase and publisher references.
// Returns an error if validation fails, or nil if all checks pass.
func (p *ProjectConfig) Validate(repo *RepoConfig) error {
	return validateProjectConfig(p, repo)
}
