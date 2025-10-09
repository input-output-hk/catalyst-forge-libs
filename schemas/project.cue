package schema

import (
	"github.com/input-output-hk/catalyst-forge-libs/schema/artifacts"
)

// Hidden field to satisfy CUE's import usage checker
_useArtifacts: artifacts.#ArtifactSpec

// ProjectConfig defines project-level configuration for Catalyst Forge.
// This is the configuration for an individual project within a repository.
#ProjectConfig: {
	name: string // Project name
	phases: [string]:    #PhaseParticipation          // Map of phase names to phase participation
	artifacts: [string]: artifacts.#ArtifactSpec // Map of artifact names to artifact specifications
	release?: #ReleaseConfig    // Optional release configuration
	deploy?:  #DeploymentConfig // Optional deployment configuration
}

// PhaseParticipation defines how a project participates in a specific phase.
// Contains the steps to execute during that phase.
#PhaseParticipation: {
	steps: [...#Step] // List of steps to execute in this phase
}

// EarthlyStep defines a step that executes an Earthly target.
// Discriminated by action!: "earthly".
#EarthlyStep: {
	name:     string    // Step name
	action!:  "earthly" // Required literal tag for discriminated union
	target:   string    // Earthly target to execute (e.g., "+test")
	timeout?: string    // Optional timeout (e.g., "30m", "1h")
}

// Step is a discriminated union of all step types.
// MVP: earthly only
// Future: can extend with | #DockerStep | #ShellStep, etc.
#Step: #EarthlyStep

// ReleaseConfig defines when and how releases should be triggered.
#ReleaseConfig: {
	on: [...#ReleaseTrigger] // List of conditions that trigger a release
}

// ReleaseTrigger defines a condition that triggers a release.
// At least one field should be specified.
#ReleaseTrigger: {
	branch?: string // Branch name or regex pattern that triggers release
	tag?:    bool   // Trigger on any git tag
	manual?: bool   // Allow manual trigger only
}

// DeploymentConfig defines Kubernetes resources to deploy.
#DeploymentConfig: {
	resources: [...#K8sResource] // List of Kubernetes resources to deploy
}

// K8sResource defines a Kubernetes resource manifest.
// Uses flexible map structure for metadata and spec to support any K8s resource type.
#K8sResource: {
	apiVersion: string // Kubernetes API version (e.g., "apps/v1")
	kind:       string // Kubernetes resource kind (e.g., "Deployment", "Service")
	metadata: {...} // Kubernetes metadata (flexible map)
	spec: {...} // Kubernetes spec (flexible map)
}
