// Package domain provides canonical type definitions for Catalyst Forge platform entities.
package domain

import "time"

// PipelineEvent represents an event emitted during pipeline execution lifecycle.
// These events are published to NATS for asynchronous processing by monitoring
// and notification services.
type PipelineEvent struct {
	// EventID is a unique identifier for this specific event instance.
	EventID string `json:"event_id"`

	// Timestamp is when this event was generated.
	Timestamp time.Time `json:"timestamp"`

	// PipelineRunID references the pipeline run that triggered this event.
	PipelineRunID string `json:"pipeline_run_id"`

	// Status indicates the current status of the pipeline run.
	Status PipelineStatus `json:"status"`

	// Metadata contains additional event-specific information as key-value pairs.
	// This field is optional and omitted from JSON when empty.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ReleaseEvent represents an event emitted when a release is created or updated.
// These events enable downstream systems to react to new releases, trigger
// deployments, or update release tracking dashboards.
type ReleaseEvent struct {
	// EventID is a unique identifier for this specific event instance.
	EventID string `json:"event_id"`

	// Timestamp is when this event was generated.
	Timestamp time.Time `json:"timestamp"`

	// ReleaseID references the release that triggered this event.
	ReleaseID string `json:"release_id"`

	// Repository is the name of the repository containing the released project.
	Repository string `json:"repository"`

	// Project is the name of the project being released.
	Project string `json:"project"`

	// Version is the semantic version or version identifier for this release.
	Version string `json:"version"`
}

// DeploymentEvent represents an event emitted during deployment lifecycle.
// These events track deployment status changes and enable integration with
// monitoring, notification, and GitOps reconciliation systems.
type DeploymentEvent struct {
	// EventID is a unique identifier for this specific event instance.
	EventID string `json:"event_id"`

	// Timestamp is when this event was generated.
	Timestamp time.Time `json:"timestamp"`

	// DeploymentID references the deployment that triggered this event.
	DeploymentID string `json:"deployment_id"`

	// Environment is the target environment for this deployment (e.g., "staging", "production").
	// Note: Using string type for consistency with entity definitions.
	Environment string `json:"environment"`

	// Status indicates the current status of the deployment (e.g., "pending", "active", "superseded").
	Status string `json:"status"`
}

// ArtifactEvent represents an event emitted when an artifact is published.
// These events enable artifact tracking, security scanning triggers, and
// integration with artifact repositories.
type ArtifactEvent struct {
	// EventID is a unique identifier for this specific event instance.
	EventID string `json:"event_id"`

	// Timestamp is when this event was generated.
	Timestamp time.Time `json:"timestamp"`

	// ArtifactID references the artifact that triggered this event.
	ArtifactID string `json:"artifact_id"`

	// Type indicates the type of artifact (e.g., container, binary, archive, package).
	Type ArtifactType `json:"type"`

	// URI is the location where the artifact can be accessed.
	URI string `json:"uri"`
}
