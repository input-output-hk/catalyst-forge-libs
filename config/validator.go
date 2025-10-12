package config

import (
	"fmt"
	"strings"

	"github.com/input-output-hk/catalyst-forge-libs/errors"
)

// validateProjectConfig validates a project configuration against repository context.
// This performs referential integrity validation that CUE cannot express.
//
// Specifically, it validates:
//   - Phase references: All project phases must exist in repo.Phases
//   - Publisher references: All artifact publishers must exist in repo.Publishers
//
// Note: All schema-level validation (types, formats, required fields, etc.) is handled by CUE.
// This function only validates cross-config references.
func validateProjectConfig(project *ProjectConfig, repo *RepoConfig) error {
	if project == nil {
		return errors.New(errors.CodeInvalidInput, "project configuration is nil")
	}
	if repo == nil {
		return errors.New(errors.CodeInvalidInput, "repository configuration is nil")
	}

	// Collect all validation errors
	var validationErrors []string

	// Validate phase references
	if err := validateProjectPhaseReferences(project, repo); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}

	// Validate artifact publisher references
	if err := validateArtifactPublisherReferences(project, repo); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}

	// If we have validation errors, combine them into a single error
	if len(validationErrors) > 0 {
		return errors.New(
			errors.CodeInvalidConfig,
			fmt.Sprintf("project configuration validation failed: %s", strings.Join(validationErrors, "; ")),
		)
	}

	return nil
}

// validateProjectPhaseReferences ensures all project phases reference phases that exist in the repository configuration.
// Returns a detailed error with field paths if validation fails.
func validateProjectPhaseReferences(project *ProjectConfig, repo *RepoConfig) error {
	if project.ProjectConfig == nil || project.Phases == nil {
		// No phases to validate
		return nil
	}

	var invalidPhases []string

	// Check each phase the project participates in
	for phaseName := range project.Phases {
		if !repo.HasPhase(phaseName) {
			invalidPhases = append(invalidPhases, phaseName)
		}
	}

	if len(invalidPhases) > 0 {
		return errors.New(
			errors.CodeInvalidConfig,
			fmt.Sprintf("project references unknown phase(s): %s (available phases: %s)",
				strings.Join(invalidPhases, ", "),
				strings.Join(repo.ListPhases(), ", "),
			),
		)
	}

	return nil
}

// validateArtifactPublisherReferences ensures all artifact publishers reference publishers that exist in the repository configuration.
// Returns a detailed error with field paths if validation fails.
func validateArtifactPublisherReferences(project *ProjectConfig, repo *RepoConfig) error {
	if project.Artifacts == nil {
		// No artifacts to validate
		return nil
	}

	var validationErrors []string

	// Check each artifact's publishers
	for artifactName := range project.Artifacts {
		publishers, err := project.GetArtifactPublishers(artifactName)
		if err != nil {
			// Skip artifacts with invalid structure - CUE should catch this
			continue
		}

		// Validate each publisher reference
		for _, publisherName := range publishers {
			if !repo.HasPublisher(publisherName) {
				validationErrors = append(validationErrors,
					fmt.Sprintf("artifact %q references unknown publisher %q (available publishers: %s)",
						artifactName,
						publisherName,
						strings.Join(repo.ListPublishers(), ", "),
					),
				)
			}
		}
	}

	if len(validationErrors) > 0 {
		return errors.New(
			errors.CodeInvalidConfig,
			strings.Join(validationErrors, "; "),
		)
	}

	return nil
}
