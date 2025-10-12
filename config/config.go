// Package config provides parsing, validation, and convenient access to Catalyst Forge
// repository and project configurations defined in CUE format.
//
// The package wraps the generated types from the schemas package with helper methods
// and provides straightforward loading and validation capabilities.
//
// # Basic Usage
//
// Load a repository configuration:
//
//	import (
//	    "context"
//	    "github.com/input-output-hk/catalyst-forge-libs/config"
//	    "github.com/input-output-hk/catalyst-forge-libs/fs/billy"
//	)
//
//	func main() {
//	    ctx := context.Background()
//	    fs := billy.NewReadOnlyFS("/path/to/repo")
//
//	    // Load repository configuration (validates by default)
//	    repo, err := config.LoadRepoConfig(ctx, fs, "repo.cue")
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Access configuration using helpers
//	    if repo.IsMonorepo() {
//	        fmt.Println("Repository uses monorepo tagging")
//	    }
//
//	    // List all phases
//	    for _, phaseName := range repo.ListPhases() {
//	        phase, _ := repo.GetPhase(phaseName)
//	        fmt.Printf("Phase: %s, Group: %d\n", phaseName, phase.Group)
//	    }
//	}
//
// Load a project configuration:
//
//	// Load project configuration
//	project, err := config.LoadProjectConfig(ctx, fs, ".forge/api/project.cue")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Validate project against repository context
//	if err := project.Validate(repo); err != nil {
//	    log.Fatalf("Project validation failed: %v", err)
//	}
//
//	// Check phase participation
//	if project.ParticipatesIn("test") {
//	    steps, _ := project.GetPhaseSteps("test")
//	    fmt.Printf("Test phase has %d steps\n", len(steps))
//	}
//
// # Advanced Usage
//
// Skip validation during loading:
//
//	opts := config.LoadOptions{SkipValidation: true}
//	repo, err := config.LoadRepoConfigWithOptions(ctx, fs, "repo.cue", opts)
//
// Access embedded schema types directly:
//
//	fmt.Printf("Project name: %s\n", project.Name)
//	fmt.Printf("Schema version: %s\n", repo.ForgeVersion)
package config

import (
	"context"

	"github.com/input-output-hk/catalyst-forge-libs/fs/core"
)

// SupportedSchemaVersion defines the schema version this package supports.
// Configurations with incompatible versions will fail validation.
const SupportedSchemaVersion = "0.1.0"

// LoadOptions configures the behavior of configuration loading operations.
type LoadOptions struct {
	// SkipValidation disables automatic validation after loading.
	// When true, configurations are loaded and parsed but not validated.
	// Useful for scenarios where validation will be performed separately
	// or when loading partially complete configurations.
	SkipValidation bool
}

// LoadRepoConfig loads and validates a repository configuration from the specified path.
// The configuration is automatically validated unless SkipValidation is set in options.
//
// Parameters:
//   - ctx: Context for cancellation and deadlines
//   - filesystem: Filesystem abstraction to read configuration from
//   - path: Path to the repository configuration file (e.g., "repo.cue")
//
// Returns the loaded and validated RepoConfig, or an error if loading or validation fails.
func LoadRepoConfig(ctx context.Context, filesystem core.ReadFS, path string) (*RepoConfig, error) {
	return loadRepoConfig(ctx, filesystem, path, LoadOptions{})
}

// LoadRepoConfigWithOptions loads a repository configuration with custom options.
// Provides control over validation and other loading behaviors.
//
// Parameters:
//   - ctx: Context for cancellation and deadlines
//   - filesystem: Filesystem abstraction to read configuration from
//   - path: Path to the repository configuration file (e.g., "repo.cue")
//   - opts: Loading options to customize behavior
//
// Returns the loaded RepoConfig, or an error if loading fails.
func LoadRepoConfigWithOptions(ctx context.Context, filesystem core.ReadFS, path string, opts LoadOptions) (*RepoConfig, error) {
	return loadRepoConfig(ctx, filesystem, path, opts)
}

// LoadProjectConfig loads and validates a project configuration from the specified path.
// The configuration is automatically validated unless SkipValidation is set in options.
// Note: Full validation requires repository context via the Validate method.
//
// Parameters:
//   - ctx: Context for cancellation and deadlines
//   - filesystem: Filesystem abstraction to read configuration from
//   - path: Path to the project configuration file (e.g., ".forge/project.cue")
//
// Returns the loaded and validated ProjectConfig, or an error if loading or validation fails.
func LoadProjectConfig(ctx context.Context, filesystem core.ReadFS, path string) (*ProjectConfig, error) {
	return loadProjectConfig(ctx, filesystem, path, LoadOptions{})
}

// LoadProjectConfigWithOptions loads a project configuration with custom options.
// Provides control over validation and other loading behaviors.
//
// Parameters:
//   - ctx: Context for cancellation and deadlines
//   - filesystem: Filesystem abstraction to read configuration from
//   - path: Path to the project configuration file (e.g., ".forge/project.cue")
//   - opts: Loading options to customize behavior
//
// Returns the loaded ProjectConfig, or an error if loading fails.
func LoadProjectConfigWithOptions(ctx context.Context, filesystem core.ReadFS, path string, opts LoadOptions) (*ProjectConfig, error) {
	return loadProjectConfig(ctx, filesystem, path, opts)
}
