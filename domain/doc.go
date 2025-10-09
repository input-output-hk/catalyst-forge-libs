// Package domain provides canonical type definitions for all Catalyst Forge platform entities.
//
// This package serves as Layer 0 in the foundation libraries architecture - a zero-dependency
// library containing pure data structures with comprehensive struct tags for JSON, SQL, and
// YAML serialization. It defines the domain model shared across all platform services.
//
// # Design Principles
//
//   - Zero dependencies (standard library only)
//   - Pure data structures (no business logic)
//   - No constructors or validation functions
//   - Comprehensive struct tags for multiple serialization contexts
//   - Type-safe enumerations for domain concepts
//   - Flat structure with no sub-packages
//
// # Architecture Overview
//
// The domain package is Layer 0 - the foundation. It has no dependencies on other platform
// libraries and is imported by all other components. This ensures a consistent domain model
// across REST APIs, database layers, message queues, and configuration files.
//
// All entities use struct tags to support multiple serialization contexts:
//
//   - json: JSON serialization for REST APIs and events
//   - db: Database column mapping for SQL persistence
//   - validate: Validation rules (enforced by consuming libraries, not this package)
//
// # Domain Model
//
// The package organizes types into four domains:
//
//   - Pipeline Execution: PipelineRun, PhaseExecution, TaskExecution, StepExecution
//   - Release Management: Release, ReleaseAlias, ReleaseTrigger, ReleaseArtifact, ReleaseApproval
//   - Deployment: Deployment
//   - Reference Data: Project, Repository
//
// # Quick Start
//
// Creating entities:
//
//	import (
//	    "time"
//	    "github.com/input-output-hk/catalyst-forge-libs/domain"
//	)
//
//	// Create a pipeline run entity
//	run := domain.PipelineRun{
//	    ID:         "run-123",
//	    Repository: "catalyst-forge",
//	    CommitSHA:  "abc123",
//	    Status:     domain.PipelineStatusRunning,
//	    CreatedAt:  time.Now(),
//	}
//
// JSON serialization for APIs:
//
//	import "encoding/json"
//
//	// Serialize to JSON
//	data, err := json.Marshal(run)
//	// {"id":"run-123","repository":"catalyst-forge",...}
//
//	// Deserialize from JSON
//	var run domain.PipelineRun
//	err := json.Unmarshal(data, &run)
//
// Database persistence (using sqlx or similar):
//
//	// The db struct tags map fields to database columns
//	// Example with sqlx (not included in this package):
//	// db.NamedExec("INSERT INTO pipeline_runs (...) VALUES (...)", run)
//	// db.Get(&run, "SELECT * FROM pipeline_runs WHERE id = $1", id)
//
// Event publishing:
//
//	// Create and publish events
//	event := domain.PipelineEvent{
//	    EventID:       "evt-456",
//	    Timestamp:     time.Now(),
//	    PipelineRunID: run.ID,
//	    Status:        run.Status,
//	}
//
//	eventData, _ := json.Marshal(event)
//	// Publish eventData to NATS (message queue logic not included)
//
// # Pipeline Execution Domain
//
// Pipeline execution follows a four-level hierarchy:
//
//   - PipelineRun: Top-level execution for a repository commit
//   - PhaseExecution: Groups of related tasks (e.g., "test", "build", "publish")
//   - TaskExecution: Execution of tasks for a single project/monorepo subdirectory
//   - StepExecution: Individual actions within a task (e.g., running earthly targets)
//
// Example hierarchy:
//
//	run := domain.PipelineRun{
//	    ID:         "run-123",
//	    Repository: "catalyst-forge",
//	    CommitSHA:  "abc123",
//	    Status:     domain.PipelineStatusRunning,
//	}
//
//	phase := domain.PhaseExecution{
//	    ID:            "phase-456",
//	    PipelineRunID: run.ID,
//	    PhaseName:     "test",
//	    Status:        domain.PipelineStatusRunning,
//	}
//
//	task := domain.TaskExecution{
//	    ID:               "task-789",
//	    PhaseExecutionID: phase.ID,
//	    ProjectName:      "platform-api",
//	    Status:           domain.PipelineStatusRunning,
//	}
//
//	step := domain.StepExecution{
//	    ID:              "step-012",
//	    TaskExecutionID: task.ID,
//	    StepName:        "test",
//	    Action:          "earthly",
//	    Target:          "+test",
//	    Status:          domain.PipelineStatusSuccess,
//	}
//
// # Release Management Domain
//
// Releases are immutable, versioned artifacts tracked with OCI images:
//
//	// Create a release entity
//	release := domain.Release{
//	    ID:            "rel-123",
//	    Repository:    "catalyst-forge",
//	    Project:       "platform-api",
//	    CommitSHA:     "abc123",
//	    ReleaseNumber: 42,
//	    OciURI:        "ghcr.io/org/project",
//	    OciDigest:     "sha256:...",
//	    CreatedAt:     time.Now(),
//	    CreatedBy:     "pipeline-run-123",
//	}
//
//	// Add aliases/tags
//	alias := domain.ReleaseAlias{
//	    ID:        "alias-456",
//	    ReleaseID: release.ID,
//	    Alias:     "v1.2.3",
//	    AliasType: "tag",
//	}
//
//	// Track artifacts
//	artifact := domain.ReleaseArtifact{
//	    ID:           "art-789",
//	    ReleaseID:    release.ID,
//	    ArtifactName: "api-server",
//	    ArtifactType: domain.ArtifactTypeContainer,
//	}
//
// # Enumerations
//
// Type-safe enumerations provide compile-time safety for domain concepts:
//
//	// Pipeline status (used across execution hierarchy)
//	status := domain.PipelineStatusRunning
//	fmt.Println(status.String()) // "RUNNING"
//
//	// Artifact types
//	artifactType := domain.ArtifactTypeContainer
//	fmt.Println(artifactType.String()) // "CONTAINER"
//
// Available enumerations:
//
//   - PipelineStatus: PENDING, RUNNING, SUCCESS, FAILED, CANCELLED
//   - ArtifactType: CONTAINER, BINARY, ARCHIVE, PACKAGE
//
// Other status fields (trigger_type, alias_type, deployment status) use plain strings
// as defined in the domain model specification.
//
// # Events
//
// Event types for asynchronous messaging via NATS:
//
//	// Pipeline lifecycle events
//	pipelineEvent := domain.PipelineEvent{
//	    EventID:       "evt-123",
//	    Timestamp:     time.Now(),
//	    PipelineRunID: "run-123",
//	    Status:        domain.PipelineStatusSuccess,
//	    Metadata: map[string]string{
//	        "duration": "5m30s",
//	        "branch":   "main",
//	    },
//	}
//
//	// Release events
//	releaseEvent := domain.ReleaseEvent{
//	    EventID:    "evt-456",
//	    Timestamp:  time.Now(),
//	    ReleaseID:  "rel-123",
//	    Repository: "catalyst-forge",
//	    Project:    "platform-api",
//	    Version:    "v1.2.3",
//	}
//
//	// Deployment events
//	deploymentEvent := domain.DeploymentEvent{
//	    EventID:      "evt-789",
//	    Timestamp:    time.Now(),
//	    DeploymentID: "dep-123",
//	    Environment:  "production",
//	    Status:       "active",
//	}
//
//	// Artifact events
//	artifactEvent := domain.ArtifactEvent{
//	    EventID:    "evt-012",
//	    Timestamp:  time.Now(),
//	    ArtifactID: "art-123",
//	    Type:       domain.ArtifactTypeContainer,
//	    URI:        "ghcr.io/org/project:v1.2.3",
//	}
//
// # Struct Tag Contexts
//
// All entity structs use comprehensive tags for different serialization contexts:
//
// JSON tags (for REST APIs and events):
//
//	Repository string `json:"repository"`            // Required field
//	StartedAt  *time.Time `json:"started_at,omitempty"` // Optional field (omit if nil)
//
// Database tags (for SQL persistence):
//
//	Repository string `db:"repository"`              // Maps to 'repository' column
//	StartedAt  *time.Time `db:"started_at"`            // Nullable column (NULL if nil)
//
// Validation tags (enforced by consuming libraries):
//
//	Repository string `validate:"required"`          // Required field validation
//	URL        string `validate:"required,url"`      // URL format validation
//
// Complete example:
//
//	type Release struct {
//	    ID         string    `json:"id" db:"id"`
//	    Repository string    `json:"repository" db:"repository" validate:"required"`
//	    CommitSHA  string    `json:"commit_sha" db:"commit_sha" validate:"required"`
//	    CreatedAt  time.Time `json:"created_at" db:"created_at"`
//	}
//
// # Optional vs Required Fields
//
// The package uses pointer types to distinguish between optional and required fields:
//
//   - Pointer types (*time.Time, *string, *int): Optional fields that may be NULL in database
//   - Value types (time.Time, string, int): Required fields with default zero values
//
// Examples:
//
//	StartedAt   *time.Time // Optional: nil = not started yet
//	CompletedAt *time.Time // Optional: nil = still running
//	CreatedAt   time.Time  // Required: always set
//
//	PipelineRunID *string  // Optional: nil for manual releases
//	ReleaseID     string   // Required: always references a release
//
// # What This Package Does NOT Include
//
// This is a pure data structure library. It explicitly does NOT include:
//
//   - Business logic or validation functions
//   - Constructors or builder patterns
//   - Database persistence logic
//   - API handlers or serialization utilities
//   - Message queue publishing/consuming logic
//   - Configuration management
//
// These concerns are handled by consuming libraries that import domain types:
//
//   - errors package: Platform error handling
//   - Database libraries: Persistence and query logic
//   - API libraries: HTTP handlers and validation
//   - Message queue libraries: Event publishing and consuming
//
// # Layer 0 Library
//
// This is a Layer 0 library in the Catalyst Forge architecture, meaning it has
// no dependencies on other platform libraries. All other platform components
// import this library to use canonical domain types.
//
// Import path:
//
//	import "github.com/input-output-hk/catalyst-forge-libs/domain"
package domain
