// Package config provides parsing, validation, and convenient access
// to Catalyst Forge repository and project configurations.
package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/input-output-hk/catalyst-forge-libs/schema/phases"
	"github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

// filterPhasesByGroup returns phases matching a specific group number.
// The result is a slice of phase names.
func filterPhasesByGroup(phases map[string]phases.PhaseDefinition, group int) []string {
	result := make([]string, 0)

	for name, phase := range phases {
		if phase.Group == int64(group) {
			result = append(result, name)
		}
	}

	// Sort for deterministic output
	slices.Sort(result)

	return result
}

// sortPhasesByGroup returns phases organized by group number.
// Each inner slice contains phase names for that group, sorted alphabetically.
// The outer slice is ordered by group number (ascending).
func sortPhasesByGroup(phases map[string]phases.PhaseDefinition) [][]string {
	// Find all unique group numbers
	groupSet := make(map[int64]bool)
	for _, phase := range phases {
		groupSet[phase.Group] = true
	}

	// Sort group numbers
	groups := make([]int64, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	slices.Sort(groups)

	// Build result with phases for each group
	result := make([][]string, len(groups))
	for i, group := range groups {
		result[i] = filterPhasesByGroup(phases, int(group))
	}

	return result
}

// phaseDefinitionToString formats a PhaseDefinition for debugging.
// It returns a human-readable string representation.
func phaseDefinitionToString(phase phases.PhaseDefinition) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("group=%d", phase.Group))

	if phase.Description != "" {
		parts = append(parts, fmt.Sprintf("description=%q", phase.Description))
	}

	if phase.Timeout != "" {
		parts = append(parts, fmt.Sprintf("timeout=%s", phase.Timeout))
	}

	if phase.Required {
		parts = append(parts, "required=true")
	}

	return fmt.Sprintf("PhaseDefinition{%s}", strings.Join(parts, ", "))
}

// publisherConfigToString formats a PublisherConfig for debugging.
// It returns a human-readable string representation.
func publisherConfigToString(pub publishers.PublisherConfig) string {
	// Get the type discriminator
	typeVal, ok := pub["type"]
	if !ok {
		return "PublisherConfig{invalid: missing type field}"
	}

	pubType, ok := typeVal.(string)
	if !ok {
		return "PublisherConfig{invalid: type field is not a string}"
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("type=%s", pubType))

	// Add type-specific fields
	switch pubType {
	case "docker":
		if registry, ok := pub["registry"].(string); ok {
			parts = append(parts, fmt.Sprintf("registry=%s", registry))
		}
		if namespace, ok := pub["namespace"].(string); ok {
			parts = append(parts, fmt.Sprintf("namespace=%s", namespace))
		}
	case "github":
		if repo, ok := pub["repository"].(string); ok {
			parts = append(parts, fmt.Sprintf("repository=%s", repo))
		}
	case "s3":
		if bucket, ok := pub["bucket"].(string); ok {
			parts = append(parts, fmt.Sprintf("bucket=%s", bucket))
		}
		if region, ok := pub["region"].(string); ok && region != "" {
			parts = append(parts, fmt.Sprintf("region=%s", region))
		}
	}

	// Check for credentials
	if _, hasCredentials := pub["credentials"]; hasCredentials {
		parts = append(parts, "credentials=<present>")
	}

	return fmt.Sprintf("PublisherConfig{%s}", strings.Join(parts, ", "))
}
