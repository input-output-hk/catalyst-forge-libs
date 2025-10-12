package config

import (
	"context"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cuepkg "github.com/input-output-hk/catalyst-forge-libs/cue"
	"github.com/input-output-hk/catalyst-forge-libs/cue/attributes"
	"github.com/input-output-hk/catalyst-forge-libs/fs/billy"
)

// TestIntegration_EndToEndFlow tests the complete workflow from loading configurations
// to querying data and validating references.
func TestIntegration_EndToEndFlow(t *testing.T) {
	ctx := context.Background()

	// Setup: Create memory filesystem with test fixtures
	fs := setupTestFS(t, "valid-repo.cue", "valid-project.cue")

	// Step 1: Load repository configuration
	repo, err := LoadRepoConfig(ctx, fs, "valid-repo.cue")
	if err != nil {
		t.Fatalf("Failed to load repository config: %v", err)
	}

	// Verify repo loaded correctly
	if repo.ForgeVersion != "0.1.0" {
		t.Errorf("Expected ForgeVersion='0.1.0', got %q", repo.ForgeVersion)
	}

	// Step 2: Load project configuration
	project, err := LoadProjectConfig(ctx, fs, "valid-project.cue")
	if err != nil {
		t.Fatalf("Failed to load project config: %v", err)
	}

	// Verify project loaded correctly
	if project.Name != "test-project" {
		t.Errorf("Expected Name='test-project', got %q", project.Name)
	}

	// Step 3: Validate project against repository context
	if err := project.Validate(repo); err != nil {
		t.Errorf("Project validation failed: %v", err)
	}

	// Step 4: Query configuration data using helper methods
	t.Run("Query repository data", func(t *testing.T) {
		// Check tagging strategy
		if !repo.IsMonorepo() {
			t.Error("Expected IsMonorepo() to be true")
		}

		if repo.IsTagAll() {
			t.Error("Expected IsTagAll() to be false")
		}

		// List and verify phases
		phases := repo.ListPhases()
		expectedPhases := []string{"build", "deploy", "test"} // sorted
		if len(phases) != len(expectedPhases) {
			t.Errorf("Expected %d phases, got %d", len(expectedPhases), len(phases))
		}
		for i, expected := range expectedPhases {
			if i >= len(phases) || phases[i] != expected {
				t.Errorf("Expected phase[%d]=%q, got %q", i, expected, phases[i])
			}
		}

		// Get specific phase
		buildPhase, ok := repo.GetPhase("build")
		if !ok {
			t.Fatal("Expected build phase to exist")
		}
		if buildPhase.Group != 1 {
			t.Errorf("Expected build phase group=1, got %d", buildPhase.Group)
		}
		if buildPhase.Description != "Build artifacts" {
			t.Errorf("Expected build phase description='Build artifacts', got %q", buildPhase.Description)
		}
		if buildPhase.Timeout != "30m" {
			t.Errorf("Expected build phase timeout='30m', got %q", buildPhase.Timeout)
		}

		// Check phase existence
		if !repo.HasPhase("test") {
			t.Error("Expected test phase to exist")
		}
		if repo.HasPhase("nonexistent") {
			t.Error("Expected nonexistent phase to not exist")
		}

		// List and verify publishers
		publishers := repo.ListPublishers()
		expectedPublishers := []string{"docker", "github"} // sorted
		if len(publishers) != len(expectedPublishers) {
			t.Errorf("Expected %d publishers, got %d", len(expectedPublishers), len(publishers))
		}
		for i, expected := range expectedPublishers {
			if i >= len(publishers) || publishers[i] != expected {
				t.Errorf("Expected publisher[%d]=%q, got %q", i, expected, publishers[i])
			}
		}

		// Get specific publisher
		dockerPub, ok := repo.GetPublisher("docker")
		if !ok {
			t.Fatal("Expected docker publisher to exist")
		}
		if dockerPub["type"] != "docker" {
			t.Errorf("Expected docker publisher type='docker', got %v", dockerPub["type"])
		}

		// Check publisher existence
		if !repo.HasPublisher("github") {
			t.Error("Expected github publisher to exist")
		}
		if repo.HasPublisher("nonexistent") {
			t.Error("Expected nonexistent publisher to not exist")
		}
	})

	t.Run("Query project data", func(t *testing.T) {
		// Check phase participation
		if !project.ParticipatesIn("build") {
			t.Error("Expected project to participate in build phase")
		}
		if !project.ParticipatesIn("test") {
			t.Error("Expected project to participate in test phase")
		}
		if project.ParticipatesIn("deploy") {
			t.Error("Expected project to not participate in deploy phase")
		}

		// Get phase steps
		buildSteps, ok := project.GetPhaseSteps("build")
		if !ok {
			t.Fatal("Expected to get build phase steps")
		}
		if len(buildSteps) != 1 {
			t.Errorf("Expected 1 build step, got %d", len(buildSteps))
		}
		if len(buildSteps) > 0 && buildSteps[0].Name != "compile" {
			t.Errorf("Expected first build step name='compile', got %q", buildSteps[0].Name)
		}

		// List participating phases
		participatingPhases := project.ListParticipatingPhases()
		expectedParticipating := []string{"build", "test"} // sorted
		if len(participatingPhases) != len(expectedParticipating) {
			t.Errorf("Expected %d participating phases, got %d", len(expectedParticipating), len(participatingPhases))
		}

		// Check artifacts
		artifacts := project.ListArtifacts()
		if len(artifacts) != 2 {
			t.Errorf("Expected 2 artifacts, got %d", len(artifacts))
		}

		// Get specific artifact
		apiArtifact, ok := project.GetArtifact("api-server")
		if !ok {
			t.Fatal("Expected api-server artifact to exist")
		}
		if apiArtifact["type"] != "container" {
			t.Errorf("Expected api-server type='container', got %v", apiArtifact["type"])
		}

		// Check artifact existence
		if !project.HasArtifact("binary") {
			t.Error("Expected binary artifact to exist")
		}
		if project.HasArtifact("nonexistent") {
			t.Error("Expected nonexistent artifact to not exist")
		}

		// Get artifact publishers
		apiPublishers, err := project.GetArtifactPublishers("api-server")
		if err != nil {
			t.Fatalf("Failed to get artifact publishers: %v", err)
		}
		if len(apiPublishers) != 1 || apiPublishers[0] != "docker" {
			t.Errorf("Expected api-server publishers=['docker'], got %v", apiPublishers)
		}

		// Check release configuration
		if !project.HasRelease() {
			t.Error("Expected project to have release configuration")
		}

		releaseCfg, ok := project.GetReleaseTriggers()
		if !ok {
			t.Fatal("Expected to get release triggers")
		}
		if len(releaseCfg.On) != 2 {
			t.Errorf("Expected 2 release triggers, got %d", len(releaseCfg.On))
		}

		// Check deployment configuration (should not exist in this fixture)
		if project.HasDeployment() {
			t.Error("Expected project to not have deployment configuration")
		}

		_, ok = project.GetDeploymentConfig()
		if ok {
			t.Error("Expected GetDeploymentConfig to return false")
		}
	})
}

// TestIntegration_ValidationFlow tests the validation workflow with valid and invalid references.
func TestIntegration_ValidationFlow(t *testing.T) {
	ctx := context.Background()

	// Load valid configurations
	fs := setupTestFS(t, "valid-repo.cue", "valid-project.cue")
	repo, err := LoadRepoConfig(ctx, fs, "valid-repo.cue")
	if err != nil {
		t.Fatalf("Failed to load repository config: %v", err)
	}

	project, err := LoadProjectConfig(ctx, fs, "valid-project.cue")
	if err != nil {
		t.Fatalf("Failed to load project config: %v", err)
	}

	t.Run("Valid project validates successfully", func(t *testing.T) {
		err := project.Validate(repo)
		if err != nil {
			t.Errorf("Expected validation to succeed, got error: %v", err)
		}
	})

	t.Run("Invalid phase reference fails validation", func(t *testing.T) {
		// Create project with invalid phase reference
		invalidFS := billy.NewMemory()
		invalidProjectCue := `
name: "invalid-project"

phases: {
	nonexistent: {
		steps: [
			{
				name: "step1"
				action: "earthly"
				target: "+build"
			},
		]
	}
}

artifacts: {}
`
		if err := invalidFS.WriteFile("invalid-project.cue", []byte(invalidProjectCue), 0644); err != nil {
			t.Fatalf("Failed to write invalid project: %v", err)
		}

		invalidProject, err := LoadProjectConfig(ctx, invalidFS, "invalid-project.cue")
		if err != nil {
			t.Fatalf("Failed to load invalid project: %v", err)
		}

		err = invalidProject.Validate(repo)
		if err == nil {
			t.Error("Expected validation to fail for invalid phase reference")
		} else if !contains(err.Error(), "unknown phase") {
			t.Errorf("Expected error about unknown phase, got: %v", err)
		}
	})

	t.Run("Invalid publisher reference fails validation", func(t *testing.T) {
		// Create project with invalid publisher reference
		invalidFS := billy.NewMemory()
		invalidProjectCue := `
name: "invalid-project"

phases: {
	build: {
		steps: [
			{
				name: "step1"
				action: "earthly"
				target: "+build"
			},
		]
	}
}

artifacts: {
	"my-artifact": {
		type: "container"
		ref: "my-artifact:latest"
		producer: {
			type: "earthly"
			target: "+docker"
		}
		publishers: ["nonexistent-publisher"]
	}
}
`
		if err := invalidFS.WriteFile("invalid-publisher-project.cue", []byte(invalidProjectCue), 0644); err != nil {
			t.Fatalf("Failed to write invalid project: %v", err)
		}

		invalidProject, err := LoadProjectConfig(ctx, invalidFS, "invalid-publisher-project.cue")
		if err != nil {
			t.Fatalf("Failed to load invalid project: %v", err)
		}

		err = invalidProject.Validate(repo)
		if err == nil {
			t.Error("Expected validation to fail for invalid publisher reference")
		} else if !contains(err.Error(), "unknown publisher") {
			t.Errorf("Expected error about unknown publisher, got: %v", err)
		}
	})
}

// TestIntegration_AttributeProcessing tests CUE attribute processing with @repo() and @artifact().
func TestIntegration_AttributeProcessing(t *testing.T) {
	ctx := context.Background()

	// Load repository configuration
	fs := setupTestFS(t, "valid-repo.cue")
	repo, err := LoadRepoConfig(ctx, fs, "valid-repo.cue")
	if err != nil {
		t.Fatalf("Failed to load repository config: %v", err)
	}

	// Create CUE context
	cueCtx := cuecontext.New()

	t.Run("Process @repo() attributes", func(t *testing.T) {
		// Create a CUE value with @repo() attribute
		cueValue := cueCtx.CompileString(`
deployment: {
	strategy: string @repo(path="tagging.strategy")
	version: string @repo(path="forgeVersion")
}
`)
		if cueValue.Err() != nil {
			t.Fatalf("Failed to compile CUE: %v", cueValue.Err())
		}

		// Create attribute registry with repo processor
		registry, err := NewAttributeRegistry(repo, nil, cueCtx)
		if err != nil {
			t.Fatalf("Failed to create attribute registry: %v", err)
		}

		// Process attributes
		walker := attributes.NewWalker(registry, cueCtx)
		result, err := walker.Walk(ctx, cueValue)
		if err != nil {
			t.Fatalf("Failed to walk CUE value: %v", err)
		}

		// Verify results
		strategyPath := cue.ParsePath("deployment.strategy")
		strategyValue := result.LookupPath(strategyPath)
		if strategyValue.Err() != nil {
			t.Fatalf("Failed to lookup strategy: %v", strategyValue.Err())
		}

		var strategy string
		if err := strategyValue.Decode(&strategy); err != nil {
			t.Fatalf("Failed to decode strategy: %v", err)
		}
		if strategy != "monorepo" {
			t.Errorf("Expected strategy='monorepo', got %q", strategy)
		}

		versionPath := cue.ParsePath("deployment.version")
		versionValue := result.LookupPath(versionPath)
		if versionValue.Err() != nil {
			t.Fatalf("Failed to lookup version: %v", versionValue.Err())
		}

		var version string
		if err := versionValue.Decode(&version); err != nil {
			t.Fatalf("Failed to decode version: %v", err)
		}
		if version != "0.1.0" {
			t.Errorf("Expected version='0.1.0', got %q", version)
		}
	})

	t.Run("Process @artifact() attributes with data", func(t *testing.T) {
		// Provide artifact data
		artifactData := map[string]interface{}{
			"api-server": map[string]interface{}{
				"image":  "ghcr.io/test-org/api-server:v1.0.0",
				"digest": "sha256:abc123def456",
			},
		}

		// Create a CUE value with @artifact() attribute
		cueValue := cueCtx.CompileString(`
deployment: {
	image: string @artifact(name="api-server", field="image")
	digest: string @artifact(name="api-server", field="digest")
}
`)
		if cueValue.Err() != nil {
			t.Fatalf("Failed to compile CUE: %v", cueValue.Err())
		}

		// Create attribute registry with artifact processor
		registry, err := NewAttributeRegistry(repo, artifactData, cueCtx)
		if err != nil {
			t.Fatalf("Failed to create attribute registry: %v", err)
		}

		// Process attributes
		walker := attributes.NewWalker(registry, cueCtx)
		result, err := walker.Walk(ctx, cueValue)
		if err != nil {
			t.Fatalf("Failed to walk CUE value: %v", err)
		}

		// Verify results
		imagePath := cue.ParsePath("deployment.image")
		imageValue := result.LookupPath(imagePath)
		if imageValue.Err() != nil {
			t.Fatalf("Failed to lookup image: %v", imageValue.Err())
		}

		var image string
		if err := imageValue.Decode(&image); err != nil {
			t.Fatalf("Failed to decode image: %v", err)
		}
		if image != "ghcr.io/test-org/api-server:v1.0.0" {
			t.Errorf("Expected image='ghcr.io/test-org/api-server:v1.0.0', got %q", image)
		}

		digestPath := cue.ParsePath("deployment.digest")
		digestValue := result.LookupPath(digestPath)
		if digestValue.Err() != nil {
			t.Fatalf("Failed to lookup digest: %v", digestValue.Err())
		}

		var digest string
		if err := digestValue.Decode(&digest); err != nil {
			t.Fatalf("Failed to decode digest: %v", err)
		}
		if digest != "sha256:abc123def456" {
			t.Errorf("Expected digest='sha256:abc123def456', got %q", digest)
		}
	})

	t.Run("Process @artifact() attributes with defaults", func(t *testing.T) {
		// No artifact data provided - should use defaults
		cueValue := cueCtx.CompileString(`
deployment: {
	image: string @artifact(name="api-server", field="image")
	digest: string @artifact(name="api-server", field="digest")
}
`)
		if cueValue.Err() != nil {
			t.Fatalf("Failed to compile CUE: %v", cueValue.Err())
		}

		// Create attribute registry without artifact data
		registry, err := NewAttributeRegistry(repo, nil, cueCtx)
		if err != nil {
			t.Fatalf("Failed to create attribute registry: %v", err)
		}

		// Process attributes
		walker := attributes.NewWalker(registry, cueCtx)
		result, err := walker.Walk(ctx, cueValue)
		if err != nil {
			t.Fatalf("Failed to walk CUE value: %v", err)
		}

		// Verify results are default placeholders
		imagePath := cue.ParsePath("deployment.image")
		imageValue := result.LookupPath(imagePath)
		if imageValue.Err() != nil {
			t.Fatalf("Failed to lookup image: %v", imageValue.Err())
		}

		var image string
		if err := imageValue.Decode(&image); err != nil {
			t.Fatalf("Failed to decode image: %v", err)
		}
		if image != "ARTIFACT_URI_api-server" {
			t.Errorf("Expected default image placeholder, got %q", image)
		}

		digestPath := cue.ParsePath("deployment.digest")
		digestValue := result.LookupPath(digestPath)
		if digestValue.Err() != nil {
			t.Fatalf("Failed to lookup digest: %v", digestValue.Err())
		}

		var digest string
		if err := digestValue.Decode(&digest); err != nil {
			t.Fatalf("Failed to decode digest: %v", err)
		}
		if digest != "ARTIFACT_DIGEST_api-server" {
			t.Errorf("Expected default digest placeholder, got %q", digest)
		}
	})
}

// TestIntegration_MemoryFilesystem tests that the package works correctly with memory filesystem.
func TestIntegration_MemoryFilesystem(t *testing.T) {
	ctx := context.Background()

	// Create memory filesystem
	memFS := billy.NewMemory()

	// Write repository configuration
	repoCue := `
forgeVersion: "0.1.0"

tagging: {
	strategy: "tag-all"
}

phases: {
	test: {
		group: 1
		description: "Test phase"
		timeout: "10m"
		required: true
	}
}

publishers: {
	s3: {
		type: "s3"
		bucket: "test-bucket"
		region: "us-east-1"
	}
}
`
	if err := memFS.WriteFile("repo.cue", []byte(repoCue), 0644); err != nil {
		t.Fatalf("Failed to write repo.cue: %v", err)
	}

	// Write project configuration
	projectCue := `
name: "memory-test-project"

phases: {
	test: {
		steps: [
			{
				name: "run-tests"
				action: "earthly"
				target: "+test"
			},
		]
	}
}

artifacts: {
	"test-artifact": {
		type: "generic"
		name: "test.tar.gz"
		producer: {
			type: "earthly"
			target: "+package"
		}
		publishers: ["s3"]
	}
}
`
	if err := memFS.WriteFile("project.cue", []byte(projectCue), 0644); err != nil {
		t.Fatalf("Failed to write project.cue: %v", err)
	}

	// Load and validate configurations
	repo, err := LoadRepoConfig(ctx, memFS, "repo.cue")
	if err != nil {
		t.Fatalf("Failed to load repo config from memory: %v", err)
	}

	if !repo.IsTagAll() {
		t.Error("Expected IsTagAll() to be true")
	}

	project, err := LoadProjectConfig(ctx, memFS, "project.cue")
	if err != nil {
		t.Fatalf("Failed to load project config from memory: %v", err)
	}

	if project.Name != "memory-test-project" {
		t.Errorf("Expected name='memory-test-project', got %q", project.Name)
	}

	// Validate project against repo
	if err := project.Validate(repo); err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	// Verify data is accessible
	if !project.HasArtifact("test-artifact") {
		t.Error("Expected test-artifact to exist")
	}

	publishers, err := project.GetArtifactPublishers("test-artifact")
	if err != nil {
		t.Fatalf("Failed to get artifact publishers: %v", err)
	}
	if len(publishers) != 1 || publishers[0] != "s3" {
		t.Errorf("Expected publishers=['s3'], got %v", publishers)
	}
}

// TestIntegration_CompleteWorkflow tests a complete realistic workflow.
func TestIntegration_CompleteWorkflow(t *testing.T) {
	ctx := context.Background()

	// Simulate a complete workflow:
	// 1. Load repository configuration
	// 2. Load multiple project configurations
	// 3. Validate all projects
	// 4. Query configuration data
	// 5. Process CUE attributes for deployment

	fs := setupTestFS(t, "valid-repo.cue", "valid-project.cue")

	// Step 1: Load repository
	repo, err := LoadRepoConfig(ctx, fs, "valid-repo.cue")
	if err != nil {
		t.Fatalf("Step 1 failed - load repo: %v", err)
	}

	// Step 2: Load projects
	projects := make([]*ProjectConfig, 0)
	project1, err := LoadProjectConfig(ctx, fs, "valid-project.cue")
	if err != nil {
		t.Fatalf("Step 2 failed - load project: %v", err)
	}
	projects = append(projects, project1)

	// Step 3: Validate all projects
	for i, proj := range projects {
		if err := proj.Validate(repo); err != nil {
			t.Errorf("Step 3 failed - validate project %d: %v", i, err)
		}
	}

	// Step 4: Query configuration data
	// Check which phases are defined
	repoPhases := repo.ListPhases()
	if len(repoPhases) == 0 {
		t.Error("Step 4 failed - no phases found")
	}

	// Find projects participating in specific phase
	buildPhaseProjects := 0
	for _, proj := range projects {
		if proj.ParticipatesIn("build") {
			buildPhaseProjects++
		}
	}
	if buildPhaseProjects == 0 {
		t.Error("Step 4 failed - no projects participate in build phase")
	}

	// Collect all artifacts across projects
	allArtifacts := make(map[string]*ProjectConfig)
	for _, proj := range projects {
		for _, artifactName := range proj.ListArtifacts() {
			allArtifacts[artifactName] = proj
		}
	}
	if len(allArtifacts) == 0 {
		t.Error("Step 4 failed - no artifacts found")
	}

	// Step 5: Process CUE attributes for deployment
	cueCtx := cuecontext.New()

	// Simulate artifact data from a build
	artifactData := map[string]interface{}{
		"api-server": map[string]interface{}{
			"image":  "ghcr.io/test-org/api-server:v1.2.3",
			"digest": "sha256:fedcba987654321",
		},
	}

	// Create deployment template with attributes
	deploymentCue := `
deployment: {
	apiVersion: "apps/v1"
	kind: "Deployment"
	spec: {
		image: string @artifact(name="api-server", field="image")
		imageDigest: string @artifact(name="api-server", field="digest")
		forgeVersion: string @repo(path="forgeVersion")
	}
}
`
	deploymentValue := cueCtx.CompileString(deploymentCue)
	if deploymentValue.Err() != nil {
		t.Fatalf("Step 5 failed - compile deployment: %v", deploymentValue.Err())
	}

	// Process attributes
	registry, err := NewAttributeRegistry(repo, artifactData, cueCtx)
	if err != nil {
		t.Fatalf("Step 5 failed - create registry: %v", err)
	}

	walker := attributes.NewWalker(registry, cueCtx)
	result, err := walker.Walk(ctx, deploymentValue)
	if err != nil {
		t.Fatalf("Step 5 failed - walk CUE: %v", err)
	}

	// Encode to YAML for deployment
	yaml, err := cuepkg.EncodeYAML(ctx, result)
	if err != nil {
		t.Fatalf("Step 5 failed - encode YAML: %v", err)
	}

	// Verify YAML contains expected values
	yamlStr := string(yaml)
	if !contains(yamlStr, "ghcr.io/test-org/api-server:v1.2.3") {
		t.Error("Step 5 failed - YAML missing artifact image")
	}
	if !contains(yamlStr, "sha256:fedcba987654321") {
		t.Error("Step 5 failed - YAML missing artifact digest")
	}
	if !contains(yamlStr, "0.1.0") {
		t.Error("Step 5 failed - YAML missing forge version")
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOf(s, substr) >= 0)
}

// indexOf returns the index of substr in s, or -1 if not found.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
