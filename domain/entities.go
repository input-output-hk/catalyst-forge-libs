// Package domain provides canonical type definitions for Catalyst Forge platform entities.
package domain

import "time"

// PipelineRun represents a complete execution of a CI/CD pipeline for a repository.
// It coordinates multiple phases and tracks overall pipeline status and metadata.
type PipelineRun struct {
	// ID is the unique identifier for this pipeline run (UUID).
	ID string `json:"id" db:"id"`

	// Repository is the name of the repository being processed.
	Repository string `json:"repository" db:"repository" validate:"required"`

	// Branch is the git branch that triggered this run.
	Branch string `json:"branch" db:"branch"`

	// CommitSHA is the git commit hash being processed.
	CommitSHA string `json:"commit_sha" db:"commit_sha" validate:"required"`

	// TriggeredBy identifies who or what initiated this pipeline run.
	TriggeredBy string `json:"triggered_by" db:"triggered_by"`

	// TriggerType indicates how the pipeline was triggered (push, pull_request, manual).
	TriggerType string `json:"trigger_type" db:"trigger_type"`

	// Status represents the current execution status of the pipeline.
	Status PipelineStatus `json:"status" db:"status"`

	// StartedAt is when the pipeline execution began. Nil if not yet started.
	StartedAt *time.Time `json:"started_at,omitempty" db:"started_at"`

	// CompletedAt is when the pipeline execution finished. Nil if still running.
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// ArgoWorkflowName is the name of the Argo Workflow executing this pipeline.
	ArgoWorkflowName string `json:"argo_workflow_name,omitempty" db:"argo_workflow_name"`

	// DiscoveryOutput contains the JSON output from the discovery phase (stored as JSONB).
	DiscoveryOutput []byte `json:"discovery_output,omitempty" db:"discovery_output"`

	// CreatedAt is when this pipeline run record was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// PhaseExecution represents the execution of a single phase within a pipeline run.
// Phases group related tasks (e.g., "test", "build", "publish") and execute in sequence.
type PhaseExecution struct {
	// ID is the unique identifier for this phase execution (UUID).
	ID string `json:"id" db:"id"`

	// PipelineRunID is the foreign key reference to the parent PipelineRun.
	PipelineRunID string `json:"pipeline_run_id" db:"pipeline_run_id" validate:"required"`

	// PhaseName is the name of this phase (e.g., "test", "build", "publish").
	PhaseName string `json:"phase_name" db:"phase_name" validate:"required"`

	// GroupNumber indicates the execution group for parallel task scheduling.
	GroupNumber int `json:"group_number" db:"group_number"`

	// Status represents the current execution status of the phase.
	Status PipelineStatus `json:"status" db:"status"`

	// StartedAt is when the phase execution began. Nil if not yet started.
	StartedAt *time.Time `json:"started_at,omitempty" db:"started_at"`

	// CompletedAt is when the phase execution finished. Nil if still running.
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
}

// TaskExecution represents the execution of tasks for a single project within a phase.
// Each task processes one project (monorepo subdirectory) through multiple steps.
type TaskExecution struct {
	// ID is the unique identifier for this task execution (UUID).
	ID string `json:"id" db:"id"`

	// PhaseExecutionID is the foreign key reference to the parent PhaseExecution.
	PhaseExecutionID string `json:"phase_execution_id" db:"phase_execution_id" validate:"required"`

	// ProjectName is the name of the project being processed.
	ProjectName string `json:"project_name" db:"project_name" validate:"required"`

	// ProjectPath is the relative path to the project within the repository.
	ProjectPath string `json:"project_path" db:"project_path"`

	// Status represents the current execution status of the task.
	Status PipelineStatus `json:"status" db:"status"`

	// StartedAt is when the task execution began. Nil if not yet started.
	StartedAt *time.Time `json:"started_at,omitempty" db:"started_at"`

	// CompletedAt is when the task execution finished. Nil if still running.
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
}

// StepExecution represents the execution of a single step within a task.
// Steps are individual actions (e.g., running earthly targets) that produce logs and exit codes.
type StepExecution struct {
	// ID is the unique identifier for this step execution (UUID).
	ID string `json:"id" db:"id"`

	// TaskExecutionID is the foreign key reference to the parent TaskExecution.
	TaskExecutionID string `json:"task_execution_id" db:"task_execution_id" validate:"required"`

	// StepName is the name of this step (e.g., "lint", "test", "build").
	StepName string `json:"step_name" db:"step_name" validate:"required"`

	// Action is the action type being executed (e.g., "earthly").
	Action string `json:"action" db:"action"`

	// Target is the specific target for the action (e.g., "+test").
	Target string `json:"target" db:"target"`

	// Status represents the current execution status of the step.
	Status PipelineStatus `json:"status" db:"status"`

	// StartedAt is when the step execution began. Nil if not yet started.
	StartedAt *time.Time `json:"started_at,omitempty" db:"started_at"`

	// CompletedAt is when the step execution finished. Nil if still running.
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// ExitCode is the process exit code from the step execution. Nil if not yet completed.
	ExitCode *int `json:"exit_code,omitempty" db:"exit_code"`

	// LogsS3Key is the S3 key where the step execution logs are stored.
	LogsS3Key string `json:"logs_s3_key,omitempty" db:"logs_s3_key"`
}

// Release represents a versioned release of a project artifact.
// Each release is immutable and corresponds to a specific commit, with artifacts
// tracked in OCI registries and identified by unique release numbers.
type Release struct {
	// ID is the unique identifier for this release (UUID).
	ID string `json:"id" db:"id"`

	// Repository is the name of the repository containing this project.
	Repository string `json:"repository" db:"repository" validate:"required"`

	// Project is the name of the project within the repository.
	Project string `json:"project" db:"project" validate:"required"`

	// CommitSHA is the git commit hash that this release was built from.
	CommitSHA string `json:"commit_sha" db:"commit_sha" validate:"required"`

	// ReleaseNumber is the sequential release number for this project.
	ReleaseNumber int `json:"release_number" db:"release_number"`

	// PipelineRunID references the pipeline run that produced this release.
	// Nil for manual releases created outside the pipeline.
	PipelineRunID *string `json:"pipeline_run_id,omitempty" db:"pipeline_run_id"`

	// OciURI is the OCI registry URI for the release tracking image.
	OciURI string `json:"oci_uri" db:"oci_uri"`

	// OciDigest is the content-addressable digest of the OCI tracking image.
	OciDigest string `json:"oci_digest" db:"oci_digest"`

	// CreatedAt is when this release was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// CreatedBy identifies who or what created this release.
	CreatedBy string `json:"created_by" db:"created_by"`
}

// ReleaseAlias represents an alias (tag) pointing to a specific release.
// Aliases provide human-readable names for releases (e.g., "v1.2.3", "latest", "main").
type ReleaseAlias struct {
	// ID is the unique identifier for this alias (UUID).
	ID string `json:"id" db:"id"`

	// ReleaseID is the foreign key reference to the Release this alias points to.
	ReleaseID string `json:"release_id" db:"release_id" validate:"required"`

	// Alias is the alias name/tag (e.g., "v1.2.3", "latest", "main").
	Alias string `json:"alias" db:"alias" validate:"required"`

	// AliasType categorizes the alias (numeric, tag, branch, custom).
	AliasType string `json:"alias_type" db:"alias_type"`
}

// ReleaseTrigger represents the event that triggered the creation of a release.
// Each release has exactly one trigger indicating how and why it was created.
type ReleaseTrigger struct {
	// ReleaseID is both the primary key and foreign key to the Release.
	ReleaseID string `json:"release_id" db:"release_id"`

	// TriggerType indicates how the release was triggered (branch_push, tag, manual).
	TriggerType string `json:"trigger_type" db:"trigger_type"`

	// Branch is the git branch name for branch_push triggers. Nil for other trigger types.
	Branch *string `json:"branch,omitempty" db:"branch"`

	// Tag is the git tag name for tag triggers. Nil for other trigger types.
	Tag *string `json:"tag,omitempty" db:"tag"`

	// TriggeredBy identifies who or what triggered the release.
	TriggeredBy string `json:"triggered_by" db:"triggered_by"`

	// TriggeredAt is when the release was triggered.
	TriggeredAt time.Time `json:"triggered_at" db:"triggered_at"`
}

// ReleaseArtifact represents a build artifact associated with a release.
// Artifacts are tracked via OCI images and may be published to multiple registries.
type ReleaseArtifact struct {
	// ID is the unique identifier for this artifact (UUID).
	ID string `json:"id" db:"id"`

	// ReleaseID is the foreign key reference to the parent Release.
	ReleaseID string `json:"release_id" db:"release_id" validate:"required"`

	// ArtifactName is the name of this artifact (e.g., "platform-api", "web-ui").
	ArtifactName string `json:"artifact_name" db:"artifact_name" validate:"required"`

	// ArtifactType indicates the type of artifact (container, binary, archive, package).
	ArtifactType ArtifactType `json:"artifact_type" db:"artifact_type"`

	// TrackingOciURI is the OCI registry URI for the artifact tracking image.
	TrackingOciURI string `json:"tracking_oci_uri" db:"tracking_oci_uri"`

	// TrackingOciDigest is the content-addressable digest of the tracking image.
	TrackingOciDigest string `json:"tracking_oci_digest" db:"tracking_oci_digest"`

	// PrimaryPublishedURI is the primary location where the artifact is published.
	PrimaryPublishedURI string `json:"primary_published_uri" db:"primary_published_uri"`

	// CreatedAt is when this artifact record was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ReleaseApproval represents an approval to deploy a release to an environment.
// Approvals may have expiration times and can be revoked.
type ReleaseApproval struct {
	// ID is the unique identifier for this approval (UUID).
	ID string `json:"id" db:"id"`

	// ReleaseID is the foreign key reference to the approved Release.
	ReleaseID string `json:"release_id" db:"release_id" validate:"required"`

	// Environment is the target environment for this approval (e.g., "staging", "production").
	Environment string `json:"environment" db:"environment" validate:"required"`

	// Approver identifies who granted this approval.
	Approver string `json:"approver" db:"approver" validate:"required"`

	// ApprovedAt is when this approval was granted.
	ApprovedAt time.Time `json:"approved_at" db:"approved_at"`

	// ExpiresAt is when this approval expires. Nil if the approval doesn't expire.
	ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`

	// Justification is the reason or context for granting this approval.
	Justification string `json:"justification" db:"justification"`

	// Status indicates the current state of the approval (active, expired, revoked).
	Status string `json:"status" db:"status"`
}

// Deployment represents a deployment of a release to a specific environment.
// Each deployment tracks the release version, target environment, and GitOps sync status.
type Deployment struct {
	// ID is the unique identifier for this deployment (UUID).
	ID string `json:"id" db:"id"`

	// ReleaseID is the foreign key reference to the Release being deployed.
	ReleaseID string `json:"release_id" db:"release_id" validate:"required"`

	// Environment is the target environment for this deployment (e.g., "staging", "production").
	Environment string `json:"environment" db:"environment" validate:"required"`

	// Status indicates the current state of the deployment (pending, active, superseded).
	Status string `json:"status" db:"status"`

	// DeployedAt is when this deployment was executed.
	DeployedAt time.Time `json:"deployed_at" db:"deployed_at"`

	// DeployedBy identifies who or what initiated this deployment.
	DeployedBy string `json:"deployed_by" db:"deployed_by"`

	// GitOpsCommitSHA is the commit hash in the GitOps repository reflecting this deployment.
	GitOpsCommitSHA string `json:"gitops_commit_sha" db:"gitops_commit_sha"`
}

// Project represents a project (monorepo subdirectory) within a repository.
// Projects are the atomic units of build, test, and release within the platform.
type Project struct {
	// Repository is the name of the repository containing this project.
	Repository string `json:"repository" db:"repository" validate:"required"`

	// Path is the relative path to this project within the repository.
	Path string `json:"path" db:"path" validate:"required"`

	// Name is the human-readable name of the project.
	Name string `json:"name" db:"name"`
}

// Repository represents a source code repository tracked by the platform.
// Repositories contain one or more projects and are the root of the CI/CD pipeline.
type Repository struct {
	// Name is the unique name/identifier of this repository.
	Name string `json:"name" db:"name" validate:"required"`

	// URL is the git URL for cloning this repository.
	URL string `json:"url" db:"url" validate:"required,url"`

	// DefaultBranch is the primary branch for this repository (e.g., "main", "master").
	DefaultBranch string `json:"default_branch" db:"default_branch"`
}
