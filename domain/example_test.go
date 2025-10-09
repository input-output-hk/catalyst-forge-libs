package domain_test

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/input-output-hk/catalyst-forge-libs/domain"
)

// Example_pipelineRun demonstrates creating and marshaling pipeline execution entities.
// This shows the typical flow of creating a pipeline run with its execution hierarchy.
func Example_pipelineRun() {
	// Create a fixed timestamp for deterministic output
	timestamp := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)

	// Create a pipeline run with execution entities
	run := domain.PipelineRun{
		ID:          "run-123",
		Repository:  "catalyst-forge",
		Branch:      "main",
		CommitSHA:   "abc123def456",
		TriggeredBy: "github-webhook",
		TriggerType: "push",
		Status:      domain.PipelineStatusRunning,
		StartedAt:   &timestamp,
		CreatedAt:   timestamp,
	}

	// Create a phase execution
	phase := domain.PhaseExecution{
		ID:            "phase-456",
		PipelineRunID: run.ID,
		PhaseName:     "test",
		GroupNumber:   1,
		Status:        domain.PipelineStatusRunning,
		StartedAt:     &timestamp,
	}

	// Create a task execution
	task := domain.TaskExecution{
		ID:               "task-789",
		PhaseExecutionID: phase.ID,
		ProjectName:      "platform-api",
		ProjectPath:      "services/api",
		Status:           domain.PipelineStatusRunning,
		StartedAt:        &timestamp,
	}

	// Create a step execution
	exitCode := 0
	step := domain.StepExecution{
		ID:              "step-012",
		TaskExecutionID: task.ID,
		StepName:        "test",
		Action:          "earthly",
		Target:          "+test",
		Status:          domain.PipelineStatusSuccess,
		StartedAt:       &timestamp,
		CompletedAt:     &timestamp,
		ExitCode:        &exitCode,
		LogsS3Key:       "logs/step-012.log",
	}

	// Marshal to JSON to show serialization
	runData, _ := json.Marshal(run)
	phaseData, _ := json.Marshal(phase)
	taskData, _ := json.Marshal(task)
	stepData, _ := json.Marshal(step)

	fmt.Println("PipelineRun:", string(runData))
	fmt.Println("PhaseExecution:", string(phaseData))
	fmt.Println("TaskExecution:", string(taskData))
	fmt.Println("StepExecution:", string(stepData))

	// Output:
	// PipelineRun: {"id":"run-123","repository":"catalyst-forge","branch":"main","commit_sha":"abc123def456","triggered_by":"github-webhook","trigger_type":"push","status":"RUNNING","started_at":"2025-10-08T12:00:00Z","created_at":"2025-10-08T12:00:00Z"}
	// PhaseExecution: {"id":"phase-456","pipeline_run_id":"run-123","phase_name":"test","group_number":1,"status":"RUNNING","started_at":"2025-10-08T12:00:00Z"}
	// TaskExecution: {"id":"task-789","phase_execution_id":"phase-456","project_name":"platform-api","project_path":"services/api","status":"RUNNING","started_at":"2025-10-08T12:00:00Z"}
	// StepExecution: {"id":"step-012","task_execution_id":"task-789","step_name":"test","action":"earthly","target":"+test","status":"SUCCESS","started_at":"2025-10-08T12:00:00Z","completed_at":"2025-10-08T12:00:00Z","exit_code":0,"logs_s3_key":"logs/step-012.log"}
}

// Example_release demonstrates creating and marshaling release entities.
// This shows creating a release with its associated metadata entities.
func Example_release() {
	// Create a fixed timestamp for deterministic output
	timestamp := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)

	pipelineRunID := "run-123"

	// Create a release
	release := domain.Release{
		ID:            "rel-456",
		Repository:    "catalyst-forge",
		Project:       "platform-api",
		CommitSHA:     "abc123def456",
		ReleaseNumber: 42,
		PipelineRunID: &pipelineRunID,
		OciURI:        "ghcr.io/catalyst-forge/platform-api",
		OciDigest:     "sha256:abcdef123456",
		CreatedAt:     timestamp,
		CreatedBy:     "pipeline-bot",
	}

	// Create a release alias
	alias := domain.ReleaseAlias{
		ID:        "alias-789",
		ReleaseID: release.ID,
		Alias:     "v1.2.3",
		AliasType: "tag",
	}

	// Create a release trigger
	branch := "main"
	trigger := domain.ReleaseTrigger{
		ReleaseID:   release.ID,
		TriggerType: "branch_push",
		Branch:      &branch,
		TriggeredBy: "github-webhook",
		TriggeredAt: timestamp,
	}

	// Create a release artifact
	artifact := domain.ReleaseArtifact{
		ID:                  "artifact-012",
		ReleaseID:           release.ID,
		ArtifactName:        "platform-api-container",
		ArtifactType:        domain.ArtifactTypeContainer,
		TrackingOciURI:      "ghcr.io/catalyst-forge/platform-api",
		TrackingOciDigest:   "sha256:abcdef123456",
		PrimaryPublishedURI: "ghcr.io/catalyst-forge/platform-api:v1.2.3",
		CreatedAt:           timestamp,
	}

	// Create a release approval
	expiresAt := timestamp.Add(30 * 24 * time.Hour)
	approval := domain.ReleaseApproval{
		ID:            "approval-345",
		ReleaseID:     release.ID,
		Environment:   "production",
		Approver:      "release-manager",
		ApprovedAt:    timestamp,
		ExpiresAt:     &expiresAt,
		Justification: "Release v1.2.3 approved for production deployment",
		Status:        "active",
	}

	// Marshal to JSON to show serialization
	releaseData, _ := json.Marshal(release)
	aliasData, _ := json.Marshal(alias)
	triggerData, _ := json.Marshal(trigger)
	artifactData, _ := json.Marshal(artifact)
	approvalData, _ := json.Marshal(approval)

	fmt.Println("Release:", string(releaseData))
	fmt.Println("ReleaseAlias:", string(aliasData))
	fmt.Println("ReleaseTrigger:", string(triggerData))
	fmt.Println("ReleaseArtifact:", string(artifactData))
	fmt.Println("ReleaseApproval:", string(approvalData))

	// Output:
	// Release: {"id":"rel-456","repository":"catalyst-forge","project":"platform-api","commit_sha":"abc123def456","release_number":42,"pipeline_run_id":"run-123","oci_uri":"ghcr.io/catalyst-forge/platform-api","oci_digest":"sha256:abcdef123456","created_at":"2025-10-08T12:00:00Z","created_by":"pipeline-bot"}
	// ReleaseAlias: {"id":"alias-789","release_id":"rel-456","alias":"v1.2.3","alias_type":"tag"}
	// ReleaseTrigger: {"release_id":"rel-456","trigger_type":"branch_push","branch":"main","triggered_by":"github-webhook","triggered_at":"2025-10-08T12:00:00Z"}
	// ReleaseArtifact: {"id":"artifact-012","release_id":"rel-456","artifact_name":"platform-api-container","artifact_type":"CONTAINER","tracking_oci_uri":"ghcr.io/catalyst-forge/platform-api","tracking_oci_digest":"sha256:abcdef123456","primary_published_uri":"ghcr.io/catalyst-forge/platform-api:v1.2.3","created_at":"2025-10-08T12:00:00Z"}
	// ReleaseApproval: {"id":"approval-345","release_id":"rel-456","environment":"production","approver":"release-manager","approved_at":"2025-10-08T12:00:00Z","expires_at":"2025-11-07T12:00:00Z","justification":"Release v1.2.3 approved for production deployment","status":"active"}
}

// Example_events demonstrates creating and marshaling event entities.
// This shows the event types used for asynchronous messaging via NATS.
func Example_events() {
	// Create a fixed timestamp for deterministic output
	timestamp := time.Date(2025, 10, 8, 12, 0, 0, 0, time.UTC)

	// Create a pipeline event
	pipelineEvent := domain.PipelineEvent{
		EventID:       "evt-123",
		Timestamp:     timestamp,
		PipelineRunID: "run-123",
		Status:        domain.PipelineStatusSuccess,
		Metadata: map[string]string{
			"duration_seconds": "120",
			"steps_completed":  "5",
		},
	}

	// Create a release event
	releaseEvent := domain.ReleaseEvent{
		EventID:    "evt-456",
		Timestamp:  timestamp,
		ReleaseID:  "rel-456",
		Repository: "catalyst-forge",
		Project:    "platform-api",
		Version:    "v1.2.3",
	}

	// Create a deployment event
	deploymentEvent := domain.DeploymentEvent{
		EventID:      "evt-789",
		Timestamp:    timestamp,
		DeploymentID: "deploy-012",
		Environment:  "production",
		Status:       "active",
	}

	// Create an artifact event
	artifactEvent := domain.ArtifactEvent{
		EventID:    "evt-012",
		Timestamp:  timestamp,
		ArtifactID: "artifact-345",
		Type:       domain.ArtifactTypeContainer,
		URI:        "ghcr.io/catalyst-forge/platform-api:v1.2.3",
	}

	// Marshal to JSON to show serialization
	pipelineEventData, _ := json.Marshal(pipelineEvent)
	releaseEventData, _ := json.Marshal(releaseEvent)
	deploymentEventData, _ := json.Marshal(deploymentEvent)
	artifactEventData, _ := json.Marshal(artifactEvent)

	fmt.Println("PipelineEvent:", string(pipelineEventData))
	fmt.Println("ReleaseEvent:", string(releaseEventData))
	fmt.Println("DeploymentEvent:", string(deploymentEventData))
	fmt.Println("ArtifactEvent:", string(artifactEventData))

	// Output:
	// PipelineEvent: {"event_id":"evt-123","timestamp":"2025-10-08T12:00:00Z","pipeline_run_id":"run-123","status":"SUCCESS","metadata":{"duration_seconds":"120","steps_completed":"5"}}
	// ReleaseEvent: {"event_id":"evt-456","timestamp":"2025-10-08T12:00:00Z","release_id":"rel-456","repository":"catalyst-forge","project":"platform-api","version":"v1.2.3"}
	// DeploymentEvent: {"event_id":"evt-789","timestamp":"2025-10-08T12:00:00Z","deployment_id":"deploy-012","environment":"production","status":"active"}
	// ArtifactEvent: {"event_id":"evt-012","timestamp":"2025-10-08T12:00:00Z","artifact_id":"artifact-345","type":"CONTAINER","uri":"ghcr.io/catalyst-forge/platform-api:v1.2.3"}
}
