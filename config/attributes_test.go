package config

import (
	"context"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/input-output-hk/catalyst-forge-libs/cue/attributes"
	"github.com/input-output-hk/catalyst-forge-libs/schema"
	"github.com/input-output-hk/catalyst-forge-libs/schema/phases"
	"github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

// TestRepoAttributeProcessor tests the RepoAttributeProcessor.
func TestRepoAttributeProcessor(t *testing.T) {
	ctx := context.Background()
	cueCtx := cuecontext.New()

	// Create a test repository configuration
	repo := &RepoConfig{
		RepoConfig: &schema.RepoConfig{
			ForgeVersion: "0.1.0",
			Tagging: schema.TaggingStrategy{
				Strategy: "monorepo",
			},
			Phases: map[string]phases.PhaseDefinition{
				"test": {
					Group: 1,
				},
			},
			Publishers: map[string]publishers.PublisherConfig{
				"ghcr": {
					"type": "docker",
					"registry": "ghcr.io",
				},
			},
		},
	}

	processor := NewRepoAttributeProcessor(repo, cueCtx)

	t.Run("Name returns repo", func(t *testing.T) {
		if processor.Name() != "repo" {
			t.Errorf("expected name 'repo', got %q", processor.Name())
		}
	})

	t.Run("Process resolves tagging strategy", func(t *testing.T) {
		attr := attributes.Attribute{
			Name: "repo",
			Args: map[string]string{
				"path": "tagging.strategy",
			},
		}

		result, err := processor.Process(ctx, attr)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		var strategy string
		if err := result.Decode(&strategy); err != nil {
			t.Fatalf("failed to decode result: %v", err)
		}

		if strategy != "monorepo" {
			t.Errorf("expected strategy 'monorepo', got %q", strategy)
		}
	})

	t.Run("Process resolves forge version", func(t *testing.T) {
		attr := attributes.Attribute{
			Name: "repo",
			Args: map[string]string{
				"path": "forgeVersion",
			},
		}

		result, err := processor.Process(ctx, attr)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		var version string
		if err := result.Decode(&version); err != nil {
			t.Fatalf("failed to decode result: %v", err)
		}

		if version != "0.1.0" {
			t.Errorf("expected version '0.1.0', got %q", version)
		}
	})

	t.Run("Process fails without path argument", func(t *testing.T) {
		attr := attributes.Attribute{
			Name: "repo",
			Args: map[string]string{},
		}

		_, err := processor.Process(ctx, attr)
		if err == nil {
			t.Fatal("expected error for missing path argument, got nil")
		}

		if err.Error() != "@repo() attribute missing required 'path' argument" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Process fails for invalid path", func(t *testing.T) {
		attr := attributes.Attribute{
			Name: "repo",
			Args: map[string]string{
				"path": "nonexistent.field",
			},
		}

		_, err := processor.Process(ctx, attr)
		if err == nil {
			t.Fatal("expected error for invalid path, got nil")
		}
	})
}

// TestArtifactAttributeProcessor tests the ArtifactAttributeProcessor.
func TestArtifactAttributeProcessor(t *testing.T) {
	ctx := context.Background()
	cueCtx := cuecontext.New()

	t.Run("Name returns artifact", func(t *testing.T) {
		processor := NewArtifactAttributeProcessor(nil, cueCtx)
		if processor.Name() != "artifact" {
			t.Errorf("expected name 'artifact', got %q", processor.Name())
		}
	})

	t.Run("Process resolves artifact URI from provided data", func(t *testing.T) {
		artifacts := map[string]interface{}{
			"api-server": map[string]interface{}{
				"uri":    "ghcr.io/example/api-server:v1.0.0",
				"digest": "sha256:abc123",
			},
		}

		processor := NewArtifactAttributeProcessor(artifacts, cueCtx)

		attr := attributes.Attribute{
			Name: "artifact",
			Args: map[string]string{
				"name":  "api-server",
				"field": "uri",
			},
		}

		result, err := processor.Process(ctx, attr)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		var uri string
		if err := result.Decode(&uri); err != nil {
			t.Fatalf("failed to decode result: %v", err)
		}

		if uri != "ghcr.io/example/api-server:v1.0.0" {
			t.Errorf("expected uri 'ghcr.io/example/api-server:v1.0.0', got %q", uri)
		}
	})

	t.Run("Process resolves artifact digest from provided data", func(t *testing.T) {
		artifacts := map[string]interface{}{
			"worker": map[string]interface{}{
				"uri":    "ghcr.io/example/worker:v2.0.0",
				"digest": "sha256:def456",
			},
		}

		processor := NewArtifactAttributeProcessor(artifacts, cueCtx)

		attr := attributes.Attribute{
			Name: "artifact",
			Args: map[string]string{
				"name":  "worker",
				"field": "digest",
			},
		}

		result, err := processor.Process(ctx, attr)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		var digest string
		if err := result.Decode(&digest); err != nil {
			t.Fatalf("failed to decode result: %v", err)
		}

		if digest != "sha256:def456" {
			t.Errorf("expected digest 'sha256:def456', got %q", digest)
		}
	})

	t.Run("Process falls back to default when artifact not found", func(t *testing.T) {
		// Empty artifacts map
		artifacts := map[string]interface{}{}

		processor := NewArtifactAttributeProcessor(artifacts, cueCtx)

		attr := attributes.Attribute{
			Name: "artifact",
			Args: map[string]string{
				"name":  "missing-artifact",
				"field": "uri",
			},
		}

		result, err := processor.Process(ctx, attr)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		var uri string
		if err := result.Decode(&uri); err != nil {
			t.Fatalf("failed to decode result: %v", err)
		}

		expected := "ARTIFACT_URI_missing-artifact"
		if uri != expected {
			t.Errorf("expected default %q, got %q", expected, uri)
		}
	})

	t.Run("Process falls back to default when field not found", func(t *testing.T) {
		artifacts := map[string]interface{}{
			"api-server": map[string]interface{}{
				"uri": "ghcr.io/example/api-server:v1.0.0",
				// Missing "digest" field
			},
		}

		processor := NewArtifactAttributeProcessor(artifacts, cueCtx)

		attr := attributes.Attribute{
			Name: "artifact",
			Args: map[string]string{
				"name":  "api-server",
				"field": "digest",
			},
		}

		result, err := processor.Process(ctx, attr)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		var digest string
		if err := result.Decode(&digest); err != nil {
			t.Fatalf("failed to decode result: %v", err)
		}

		expected := "ARTIFACT_DIGEST_api-server"
		if digest != expected {
			t.Errorf("expected default %q, got %q", expected, digest)
		}
	})

	t.Run("Process uses default when artifacts is nil", func(t *testing.T) {
		// No artifacts provided (validation/discovery phase)
		processor := NewArtifactAttributeProcessor(nil, cueCtx)

		attr := attributes.Attribute{
			Name: "artifact",
			Args: map[string]string{
				"name":  "validation-artifact",
				"field": "uri",
			},
		}

		result, err := processor.Process(ctx, attr)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}

		var uri string
		if err := result.Decode(&uri); err != nil {
			t.Fatalf("failed to decode result: %v", err)
		}

		expected := "ARTIFACT_URI_validation-artifact"
		if uri != expected {
			t.Errorf("expected default %q, got %q", expected, uri)
		}
	})

	t.Run("Process fails without name argument", func(t *testing.T) {
		processor := NewArtifactAttributeProcessor(nil, cueCtx)

		attr := attributes.Attribute{
			Name: "artifact",
			Args: map[string]string{
				"field": "uri",
			},
		}

		_, err := processor.Process(ctx, attr)
		if err == nil {
			t.Fatal("expected error for missing name argument, got nil")
		}

		if err.Error() != "@artifact() attribute missing required 'name' argument" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Process fails without field argument", func(t *testing.T) {
		processor := NewArtifactAttributeProcessor(nil, cueCtx)

		attr := attributes.Attribute{
			Name: "artifact",
			Args: map[string]string{
				"name": "api-server",
			},
		}

		_, err := processor.Process(ctx, attr)
		if err == nil {
			t.Fatal("expected error for missing field argument, got nil")
		}

		if err.Error() != "@artifact() attribute missing required 'field' argument" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Process fails when artifact data is not a map", func(t *testing.T) {
		// Invalid artifact data structure
		artifacts := map[string]interface{}{
			"bad-artifact": "not-a-map",
		}

		processor := NewArtifactAttributeProcessor(artifacts, cueCtx)

		attr := attributes.Attribute{
			Name: "artifact",
			Args: map[string]string{
				"name":  "bad-artifact",
				"field": "uri",
			},
		}

		_, err := processor.Process(ctx, attr)
		if err == nil {
			t.Fatal("expected error for invalid artifact data, got nil")
		}
	})
}

// TestGenerateDefaultArtifactValue tests the default value generation.
func TestGenerateDefaultArtifactValue(t *testing.T) {
	tests := []struct {
		name         string
		artifactName string
		field        string
		expected     string
	}{
		{
			name:         "uri field",
			artifactName: "api-server",
			field:        "uri",
			expected:     "ARTIFACT_URI_api-server",
		},
		{
			name:         "image field",
			artifactName: "worker",
			field:        "image",
			expected:     "ARTIFACT_URI_worker",
		},
		{
			name:         "url field",
			artifactName: "frontend",
			field:        "url",
			expected:     "ARTIFACT_URI_frontend",
		},
		{
			name:         "digest field",
			artifactName: "api-server",
			field:        "digest",
			expected:     "ARTIFACT_DIGEST_api-server",
		},
		{
			name:         "sha256 field",
			artifactName: "worker",
			field:        "sha256",
			expected:     "ARTIFACT_DIGEST_worker",
		},
		{
			name:         "hash field",
			artifactName: "frontend",
			field:        "hash",
			expected:     "ARTIFACT_DIGEST_frontend",
		},
		{
			name:         "tag field",
			artifactName: "api-server",
			field:        "tag",
			expected:     "ARTIFACT_TAG_api-server",
		},
		{
			name:         "version field",
			artifactName: "worker",
			field:        "version",
			expected:     "ARTIFACT_TAG_worker",
		},
		{
			name:         "path field",
			artifactName: "frontend",
			field:        "path",
			expected:     "ARTIFACT_PATH_frontend",
		},
		{
			name:         "location field",
			artifactName: "api-server",
			field:        "location",
			expected:     "ARTIFACT_PATH_api-server",
		},
		{
			name:         "unknown field",
			artifactName: "custom",
			field:        "custom-field",
			expected:     "ARTIFACT_custom_custom-field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateDefaultArtifactValue(tt.artifactName, tt.field)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestNewAttributeRegistry tests the registry creation helper.
func TestNewAttributeRegistry(t *testing.T) {
	ctx := context.Background()
	cueCtx := cuecontext.New()

	// Create a test repository configuration
	repo := &RepoConfig{
		RepoConfig: &schema.RepoConfig{
			ForgeVersion: "0.1.0",
			Tagging: schema.TaggingStrategy{
				Strategy: "monorepo",
			},
			Phases:     map[string]phases.PhaseDefinition{},
			Publishers: map[string]publishers.PublisherConfig{},
		},
	}

	t.Run("creates registry with both processors", func(t *testing.T) {
		artifacts := map[string]interface{}{
			"test-artifact": map[string]interface{}{
				"uri": "test-uri",
			},
		}

		registry, err := NewAttributeRegistry(repo, artifacts, cueCtx)
		if err != nil {
			t.Fatalf("NewAttributeRegistry failed: %v", err)
		}

		// Verify repo processor is registered
		repoProc, ok := registry.Get("repo")
		if !ok {
			t.Error("expected 'repo' processor to be registered")
		}
		if repoProc != nil && repoProc.Name() != "repo" {
			t.Errorf("expected processor name 'repo', got %q", repoProc.Name())
		}

		// Verify artifact processor is registered
		artifactProc, ok := registry.Get("artifact")
		if !ok {
			t.Error("expected 'artifact' processor to be registered")
		}
		if artifactProc != nil && artifactProc.Name() != "artifact" {
			t.Errorf("expected processor name 'artifact', got %q", artifactProc.Name())
		}
	})

	t.Run("works with nil artifacts", func(t *testing.T) {
		registry, err := NewAttributeRegistry(repo, nil, cueCtx)
		if err != nil {
			t.Fatalf("NewAttributeRegistry failed with nil artifacts: %v", err)
		}

		// Both processors should still be registered
		if _, ok := registry.Get("repo"); !ok {
			t.Error("expected 'repo' processor to be registered")
		}
		if _, ok := registry.Get("artifact"); !ok {
			t.Error("expected 'artifact' processor to be registered")
		}
	})

	t.Run("registry can process attributes", func(t *testing.T) {
		artifacts := map[string]interface{}{
			"api-server": map[string]interface{}{
				"uri":    "ghcr.io/example/api-server:v1.0.0",
				"digest": "sha256:abc123",
			},
		}

		registry, err := NewAttributeRegistry(repo, artifacts, cueCtx)
		if err != nil {
			t.Fatalf("NewAttributeRegistry failed: %v", err)
		}

		// Test repo processor
		repoProc, _ := registry.Get("repo")
		repoAttr := attributes.Attribute{
			Name: "repo",
			Args: map[string]string{
				"path": "forgeVersion",
			},
		}
		repoResult, err := repoProc.Process(ctx, repoAttr)
		if err != nil {
			t.Fatalf("repo processor failed: %v", err)
		}
		var version string
		if err := repoResult.Decode(&version); err != nil {
			t.Fatalf("failed to decode repo result: %v", err)
		}
		if version != "0.1.0" {
			t.Errorf("expected version '0.1.0', got %q", version)
		}

		// Test artifact processor
		artifactProc, _ := registry.Get("artifact")
		artifactAttr := attributes.Attribute{
			Name: "artifact",
			Args: map[string]string{
				"name":  "api-server",
				"field": "uri",
			},
		}
		artifactResult, err := artifactProc.Process(ctx, artifactAttr)
		if err != nil {
			t.Fatalf("artifact processor failed: %v", err)
		}
		var uri string
		if err := artifactResult.Decode(&uri); err != nil {
			t.Fatalf("failed to decode artifact result: %v", err)
		}
		if uri != "ghcr.io/example/api-server:v1.0.0" {
			t.Errorf("expected uri 'ghcr.io/example/api-server:v1.0.0', got %q", uri)
		}
	})
}

// TestAttributeProcessorIntegration tests end-to-end attribute processing with CUE walker.
func TestAttributeProcessorIntegration(t *testing.T) {
	ctx := context.Background()
	cueCtx := cuecontext.New()

	// Create a test repository configuration
	repo := &RepoConfig{
		RepoConfig: &schema.RepoConfig{
			ForgeVersion: "0.1.0",
			Tagging: schema.TaggingStrategy{
				Strategy: "monorepo",
			},
			Phases:     map[string]phases.PhaseDefinition{},
			Publishers: map[string]publishers.PublisherConfig{},
		},
	}

	t.Run("process deployment with artifact attributes", func(t *testing.T) {
		// Artifact data
		artifacts := map[string]interface{}{
			"api-server": map[string]interface{}{
				"uri":    "ghcr.io/example/api-server:v1.0.0",
				"digest": "sha256:abc123def456",
			},
		}

		// Create registry
		registry, err := NewAttributeRegistry(repo, artifacts, cueCtx)
		if err != nil {
			t.Fatalf("NewAttributeRegistry failed: %v", err)
		}

		// Create a CUE value with attributes
		cueSource := `{
			deployment: {
				name: "api-server"
				image: "placeholder" @artifact(name="api-server", field="uri")
				digest: "placeholder" @artifact(name="api-server", field="digest")
			}
		}`

		value := cueCtx.CompileString(cueSource)
		if value.Err() != nil {
			t.Fatalf("failed to compile CUE: %v", value.Err())
		}

		deploymentVal := value.LookupPath(cue.ParsePath("deployment"))
		if deploymentVal.Err() != nil {
			t.Fatalf("failed to lookup deployment: %v", deploymentVal.Err())
		}

		// Walk and process attributes
		walker := attributes.NewWalker(registry, cueCtx)
		result, err := walker.Walk(ctx, deploymentVal)
		if err != nil {
			t.Fatalf("Walk failed: %v", err)
		}

		// Verify the result
		imageVal := result.LookupPath(cue.ParsePath("image"))
		var image string
		if err := imageVal.Decode(&image); err != nil {
			t.Fatalf("failed to decode image: %v", err)
		}
		if image != "ghcr.io/example/api-server:v1.0.0" {
			t.Errorf("expected image to be resolved, got %q", image)
		}

		digestVal := result.LookupPath(cue.ParsePath("digest"))
		var digest string
		if err := digestVal.Decode(&digest); err != nil {
			t.Fatalf("failed to decode digest: %v", err)
		}
		if digest != "sha256:abc123def456" {
			t.Errorf("expected digest to be resolved, got %q", digest)
		}
	})

	t.Run("process with default values when no artifact data", func(t *testing.T) {
		// No artifact data (validation phase)
		registry, err := NewAttributeRegistry(repo, nil, cueCtx)
		if err != nil {
			t.Fatalf("NewAttributeRegistry failed: %v", err)
		}

		// Create a CUE value with attributes
		cueSource := `{
			deployment: {
				image: "placeholder" @artifact(name="test-artifact", field="uri")
			}
		}`

		value := cueCtx.CompileString(cueSource)
		if value.Err() != nil {
			t.Fatalf("failed to compile CUE: %v", value.Err())
		}

		deploymentVal := value.LookupPath(cue.ParsePath("deployment"))

		// Walk and process attributes
		walker := attributes.NewWalker(registry, cueCtx)
		result, err := walker.Walk(ctx, deploymentVal)
		if err != nil {
			t.Fatalf("Walk failed: %v", err)
		}

		// Verify default value was used
		imageVal := result.LookupPath(cue.ParsePath("image"))
		var image string
		if err := imageVal.Decode(&image); err != nil {
			t.Fatalf("failed to decode image: %v", err)
		}
		expected := "ARTIFACT_URI_test-artifact"
		if image != expected {
			t.Errorf("expected default value %q, got %q", expected, image)
		}
	})
}
