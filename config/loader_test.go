package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/errors"
	"github.com/input-output-hk/catalyst-forge-libs/fs/billy"
)

// setupTestFS creates a memory filesystem and loads test fixtures.
// Accepts a list of fixture filenames to load from testdata directory.
func setupTestFS(t *testing.T, fixtures ...string) *billy.MemoryFS {
	t.Helper()
	fs := billy.NewMemory()
	
	for _, fixture := range fixtures {
		data, err := os.ReadFile(filepath.Join("testdata", fixture))
		if err != nil {
			t.Fatalf("Failed to read test fixture %s: %v", fixture, err)
		}
		if err := fs.WriteFile(fixture, data, 0644); err != nil {
			t.Fatalf("Failed to write fixture %s to memory fs: %v", fixture, err)
		}
	}
	
	return fs
}

// TestLoadRepoConfig_Valid tests loading a valid repository configuration.
func TestLoadRepoConfig_Valid(t *testing.T) {
	ctx := context.Background()
	fs := setupTestFS(t, "valid-repo.cue")

	// Skip validation since validator is not yet implemented
	opts := LoadOptions{SkipValidation: true}
	repo, err := LoadRepoConfigWithOptions(ctx, fs, "valid-repo.cue", opts)
	if err != nil {
		t.Fatalf("LoadRepoConfig failed: %v", err)
	}

	// Verify basic fields
	if repo.ForgeVersion != "0.1.0" {
		t.Errorf("Expected ForgeVersion='0.1.0', got %q", repo.ForgeVersion)
	}

	if repo.Tagging.Strategy != "monorepo" {
		t.Errorf("Expected Tagging.Strategy='monorepo', got %q", repo.Tagging.Strategy)
	}

	// Verify phases are loaded
	phases := repo.ListPhases()
	if len(phases) != 3 {
		t.Errorf("Expected 3 phases, got %d", len(phases))
	}

	// Verify specific phase exists
	buildPhase, ok := repo.GetPhase("build")
	if !ok {
		t.Error("Expected build phase to exist")
	}
	if buildPhase != nil && buildPhase.Group != 1 {
		t.Errorf("Expected build phase group=1, got %d", buildPhase.Group)
	}
	if buildPhase != nil && buildPhase.Description != "Build artifacts" {
		t.Errorf("Expected build phase description='Build artifacts', got %q", buildPhase.Description)
	}

	// Verify publishers are loaded
	publishers := repo.ListPublishers()
	if len(publishers) != 2 {
		t.Errorf("Expected 2 publishers, got %d", len(publishers))
	}

	// Verify specific publisher exists
	dockerPub, ok := repo.GetPublisher("docker")
	if !ok {
		t.Error("Expected docker publisher to exist")
	}
	if dockerPub != nil {
		typeVal, ok := dockerPub["type"].(string)
		if !ok || typeVal != "docker" {
			t.Errorf("Expected docker publisher type='docker', got %v", dockerPub["type"])
		}
	}
}

// TestLoadRepoConfig_WithOptions tests loading with skip validation option.
func TestLoadRepoConfig_WithOptions(t *testing.T) {
	ctx := context.Background()
	fs := setupTestFS(t, "valid-repo.cue")

	opts := LoadOptions{SkipValidation: true}
	repo, err := LoadRepoConfigWithOptions(ctx, fs, "valid-repo.cue", opts)
	if err != nil {
		t.Fatalf("LoadRepoConfigWithOptions failed: %v", err)
	}

	if repo.ForgeVersion != "0.1.0" {
		t.Errorf("Expected ForgeVersion='0.1.0', got %q", repo.ForgeVersion)
	}
}

// TestLoadRepoConfig_InvalidSyntax tests loading a file with invalid CUE syntax.
func TestLoadRepoConfig_InvalidSyntax(t *testing.T) {
	ctx := context.Background()
	fs := setupTestFS(t, "invalid-syntax.cue")

	_, err := LoadRepoConfig(ctx, fs, "invalid-syntax.cue")
	if err == nil {
		t.Fatal("Expected error for invalid CUE syntax, got nil")
	}

	// Verify it's a CUE load error
	var platformErr errors.PlatformError
	if !errors.As(err, &platformErr) {
		t.Error("Expected error to be a PlatformError")
	}

	// Verify error message mentions CUE loading/building
	errMsg := err.Error()
	if !strings.Contains(errMsg, "CUE") && !strings.Contains(errMsg, "cue") {
		t.Errorf("Expected error to mention CUE, got: %v", errMsg)
	}
}

// TestLoadRepoConfig_MissingFile tests loading a non-existent file.
func TestLoadRepoConfig_MissingFile(t *testing.T) {
	ctx := context.Background()
	fs := setupTestFS(t) // Empty filesystem

	_, err := LoadRepoConfig(ctx, fs, "does-not-exist.cue")
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}

	// Verify it's a config load error
	var platformErr errors.PlatformError
	if !errors.As(err, &platformErr) {
		t.Error("Expected error to be a PlatformError")
	}
}

// TestLoadProjectConfig_Valid tests loading a valid project configuration.
func TestLoadProjectConfig_Valid(t *testing.T) {
	ctx := context.Background()
	fs := setupTestFS(t, "valid-project.cue")

	// Skip validation since validator is not yet implemented
	opts := LoadOptions{SkipValidation: true}
	project, err := LoadProjectConfigWithOptions(ctx, fs, "valid-project.cue", opts)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	// Verify basic fields
	if project.Name != "test-project" {
		t.Errorf("Expected Name='test-project', got %q", project.Name)
	}

	// Verify phases are loaded
	if !project.ParticipatesIn("build") {
		t.Error("Expected project to participate in build phase")
	}
	if !project.ParticipatesIn("test") {
		t.Error("Expected project to participate in test phase")
	}

	// Verify phase steps
	buildSteps, ok := project.GetPhaseSteps("build")
	if !ok {
		t.Error("Expected to get build phase steps")
	}
	if len(buildSteps) != 1 {
		t.Errorf("Expected 1 build step, got %d", len(buildSteps))
	}
}

// TestLoadProjectConfig_Artifacts tests artifact loading.
func TestLoadProjectConfig_Artifacts(t *testing.T) {
	ctx := context.Background()
	fs := setupTestFS(t, "valid-project.cue")

	opts := LoadOptions{SkipValidation: true}
	project, err := LoadProjectConfigWithOptions(ctx, fs, "valid-project.cue", opts)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	// Verify artifacts are loaded
	artifacts := project.ListArtifacts()
	if len(artifacts) != 2 {
		t.Errorf("Expected 2 artifacts, got %d", len(artifacts))
	}

	// Verify specific artifact exists
	if !project.HasArtifact("api-server") {
		t.Error("Expected api-server artifact to exist")
	}
	if !project.HasArtifact("binary") {
		t.Error("Expected binary artifact to exist")
	}

	// Verify artifact details
	apiArtifact, ok := project.GetArtifact("api-server")
	if !ok {
		t.Error("Expected to get api-server artifact")
	}
	if apiArtifact != nil {
		typeVal, ok := apiArtifact["type"].(string)
		if !ok || typeVal != "container" {
			t.Errorf("Expected artifact type='container', got %v", apiArtifact["type"])
		}
	}
}

// TestLoadProjectConfig_Release tests release configuration loading.
func TestLoadProjectConfig_Release(t *testing.T) {
	ctx := context.Background()
	fs := setupTestFS(t, "valid-project.cue")

	opts := LoadOptions{SkipValidation: true}
	project, err := LoadProjectConfigWithOptions(ctx, fs, "valid-project.cue", opts)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	// Verify release configuration
	if !project.HasRelease() {
		t.Error("Expected project to have release configuration")
	}

	releaseConfig, ok := project.GetReleaseTriggers()
	if !ok {
		t.Error("Expected to get release triggers")
	}
	if releaseConfig != nil && len(releaseConfig.On) != 2 {
		t.Errorf("Expected 2 release triggers, got %d", len(releaseConfig.On))
	}
}

// TestLoadProjectConfig_WithOptions tests loading with options.
func TestLoadProjectConfig_WithOptions(t *testing.T) {
	ctx := context.Background()
	fs := setupTestFS(t, "valid-project.cue")

	opts := LoadOptions{SkipValidation: true}
	project, err := LoadProjectConfigWithOptions(ctx, fs, "valid-project.cue", opts)
	if err != nil {
		t.Fatalf("LoadProjectConfigWithOptions failed: %v", err)
	}

	if project.Name != "test-project" {
		t.Errorf("Expected Name='test-project', got %q", project.Name)
	}
}

// TestLoadProjectConfig_MissingFile tests loading a non-existent file.
func TestLoadProjectConfig_MissingFile(t *testing.T) {
	ctx := context.Background()
	fs := setupTestFS(t) // Empty filesystem

	_, err := LoadProjectConfig(ctx, fs, "does-not-exist.cue")
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}

	// Verify it's a config load error
	var platformErr errors.PlatformError
	if !errors.As(err, &platformErr) {
		t.Error("Expected error to be a PlatformError")
	}
}

// TestNewCueLoader tests the newCueLoader helper function.
func TestNewCueLoader(t *testing.T) {
	fs := setupTestFS(t) // Empty filesystem is fine for this test
	loader := newCueLoader(fs)

	if loader == nil {
		t.Fatal("Expected newCueLoader to return non-nil loader")
	}

	// Verify the loader has a context
	ctx := loader.Context()
	if ctx == nil {
		t.Error("Expected loader to have non-nil context")
	}
}

// TestLoadRepoConfig_ContextCancellation tests context cancellation during loading.
func TestLoadRepoConfig_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	fs := setupTestFS(t, "valid-repo.cue")

	_, err := LoadRepoConfig(ctx, fs, "valid-repo.cue")
	if err == nil {
		t.Fatal("Expected error due to context cancellation, got nil")
	}

	// The error should mention context
	errMsg := err.Error()
	if !strings.Contains(errMsg, "context") && !strings.Contains(errMsg, "cancel") {
		t.Logf("Warning: error message doesn't mention context cancellation: %v", errMsg)
	}
}

// TestLoadProjectConfig_ContextCancellation tests context cancellation during loading.
func TestLoadProjectConfig_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	fs := setupTestFS(t, "valid-project.cue")

	_, err := LoadProjectConfig(ctx, fs, "valid-project.cue")
	if err == nil {
		t.Fatal("Expected error due to context cancellation, got nil")
	}

	// The error should mention context
	errMsg := err.Error()
	if !strings.Contains(errMsg, "context") && !strings.Contains(errMsg, "cancel") {
		t.Logf("Warning: error message doesn't mention context cancellation: %v", errMsg)
	}
}
