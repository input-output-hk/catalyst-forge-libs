// Package domain provides canonical type definitions for Catalyst Forge platform entities.
package domain

// PipelineStatus represents the execution status of pipeline components.
// This status is used across the execution hierarchy: PipelineRun, PhaseExecution,
// TaskExecution, and StepExecution.
type PipelineStatus string

const (
	// PipelineStatusPending indicates execution is queued but not yet started.
	PipelineStatusPending PipelineStatus = "PENDING"

	// PipelineStatusRunning indicates execution is currently in progress.
	PipelineStatusRunning PipelineStatus = "RUNNING"

	// PipelineStatusSuccess indicates execution completed successfully.
	PipelineStatusSuccess PipelineStatus = "SUCCESS"

	// PipelineStatusFailed indicates execution completed with errors.
	PipelineStatusFailed PipelineStatus = "FAILED"

	// PipelineStatusCancelled indicates execution was cancelled before completion.
	PipelineStatusCancelled PipelineStatus = "CANCELLED"
)

// String returns the string representation of the PipelineStatus.
func (s PipelineStatus) String() string {
	return string(s)
}

// ArtifactType represents the type of build artifact.
// Artifacts are tracked in ReleaseArtifact entities with metadata stored
// in OCI tracking images.
type ArtifactType string

const (
	// ArtifactTypeContainer represents container/Docker images.
	ArtifactTypeContainer ArtifactType = "CONTAINER"

	// ArtifactTypeBinary represents compiled executable binaries.
	ArtifactTypeBinary ArtifactType = "BINARY"

	// ArtifactTypeArchive represents compressed archives (tar.gz, zip, etc).
	ArtifactTypeArchive ArtifactType = "ARCHIVE"

	// ArtifactTypePackage represents language-specific packages (npm, pip, etc).
	ArtifactTypePackage ArtifactType = "PACKAGE"
)

// String returns the string representation of the ArtifactType.
func (t ArtifactType) String() string {
	return string(t)
}
