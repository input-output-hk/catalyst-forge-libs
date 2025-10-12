package schema

import (
	art "github.com/input-output-hk/catalyst-forge-libs/schema/artifacts"
)

// Hidden field to satisfy CUE's import usage checker
_useArtifacts: art.#ArtifactSpec

// ProjectConfig defines project-level configuration for Catalyst Forge.
// This is the configuration for an individual project within a repository.
#ProjectConfig: {
	// Project name
	name: string
	// Map of phase names to phase participation
	phases: [string]: #PhaseParticipation
	// Map of artifact names to artifact specifications
	artifacts: [string]: art.#ArtifactSpec
	// Optional release configuration
	release?: #ReleaseConfig
	// Optional deployment configuration
	deploy?: #DeploymentConfig
}

// PhaseParticipation defines how a project participates in a specific phase.
// Contains the steps to execute during that phase.
#PhaseParticipation: {
	// List of steps to execute in this phase
	steps: [...#Step]
}

// EarthlyStep defines a step that executes an Earthly target.
// Discriminated by action!: "earthly".
#EarthlyStep: {
	// Step name
	name: string
	// Required literal tag for discriminated union
	action!: "earthly"
	// Earthly target to execute (e.g., "+test")
	target: string
	// Optional timeout (e.g., "30m", "1h")
	timeout?: string
}

// Step is a discriminated union of all step types.
// MVP: earthly only
// Future: can extend with | #DockerStep | #ShellStep, etc.
#Step: #EarthlyStep

// ReleaseConfig defines when and how releases should be triggered.
#ReleaseConfig: {
	// List of conditions that trigger a release
	on: [...#ReleaseTrigger]
}

// ReleaseTrigger defines a condition that triggers a release.
// At least one field should be specified.
#ReleaseTrigger: {
	// Branch name or regex pattern that triggers release
	branch?: string
	// Trigger on any git tag
	tag?: bool
	// Allow manual trigger only
	manual?: bool
}

// DeploymentConfig defines Kubernetes resources to deploy.
#DeploymentConfig: {
	// List of Kubernetes resources to deploy
	resources: [...#K8sResource]
}

// K8sResource defines a Kubernetes resource manifest.
// Uses flexible map structure for metadata and spec to support any K8s resource type.
#K8sResource: {
	// Kubernetes API version (e.g., "apps/v1")
	apiVersion: string
	// Kubernetes resource kind (e.g., "Deployment", "Service")
	kind: string
	// Kubernetes metadata (flexible map)
	metadata: {...}
	// Kubernetes spec (flexible map)
	spec: {...}
}
