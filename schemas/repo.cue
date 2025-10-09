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
	forgeVersion: string           // Schema version for compatibility checks (e.g., "0.1.0")
	tagging:      #TaggingStrategy // Git tagging strategy
	phases: [string]:     phases.#PhaseDefinition       // Map of phase names to phase definitions
	publishers: [string]: publishers.#PublisherConfig // Map of publisher names to publisher configurations
}

// TaggingStrategy defines how git tags should be applied in the repository.
#TaggingStrategy: {
	strategy: "monorepo" | "tag-all" // "monorepo": tags per project, "tag-all": single tag for repo
}
