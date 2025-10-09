package schema

import (
	"github.com/input-output-hk/catalyst-forge-libs/schema/phases"
	"github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

// Hidden fields to satisfy CUE's import usage checker
_usePhases:     phases.#PhaseDefinition
_usePublishers: publishers.#PublisherConfig

// RepoConfig defines repository-level configuration for Catalyst Forge.
// This is the root configuration for an entire repository.
#RepoConfig: {
	// Schema version for compatibility checks (e.g., "0.1.0")
	forgeVersion: string
	// Git tagging strategy
	tagging: #TaggingStrategy
	// Map of phase names to phase definitions
	phases: [string]: phases.#PhaseDefinition
	// Map of publisher names to publisher configurations
	publishers: [string]: publishers.#PublisherConfig
}

// TaggingStrategy defines how git tags should be applied in the repository.
#TaggingStrategy: {
	// "monorepo": tags per project, "tag-all": single tag for repo
	strategy: "monorepo" | "tag-all"
}
