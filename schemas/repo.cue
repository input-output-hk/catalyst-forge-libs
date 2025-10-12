package schema

import (
	ph "github.com/input-output-hk/catalyst-forge-libs/schema/phases"
	pub "github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

// Hidden fields to satisfy CUE's import usage checker
_usePhases:     ph.#PhaseDefinition
_usePublishers: pub.#PublisherConfig

// RepoConfig defines repository-level configuration for Catalyst Forge.
// This is the root configuration for an entire repository.
#RepoConfig: {
	// Schema version for compatibility checks (e.g., "0.1.0")
	forgeVersion: string
	// Git tagging strategy
	tagging: #TaggingStrategy
	// Map of phase names to phase definitions
	phases: [string]: ph.#PhaseDefinition
	// Map of publisher names to publisher configurations
	publishers: [string]: pub.#PublisherConfig
}

// TaggingStrategy defines how git tags should be applied in the repository.
#TaggingStrategy: {
	// "monorepo": tags per project, "tag-all": single tag for repo
	strategy: "monorepo" | "tag-all"
}
