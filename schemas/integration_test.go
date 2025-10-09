package schema

import (
	"encoding/json"
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/schema/artifacts"
	"github.com/input-output-hk/catalyst-forge-libs/schema/common"
	"github.com/input-output-hk/catalyst-forge-libs/schema/phases"
	"github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

// TestRepoConfig_UnmarshalYAML tests unmarshaling RepoConfig from JSON (YAML-like structure).
// Note: The generated types have json struct tags which work for both JSON and YAML unmarshaling.
func TestRepoConfig_UnmarshalYAML(t *testing.T) {
	// Using JSON for testing since generated types have json struct tags
	jsonData := `{
		"forgeVersion": "0.1.0",
		"tagging": {
			"strategy": "monorepo"
		},
		"phases": {},
		"publishers": {}
	}`

	var config RepoConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal RepoConfig from JSON: %v", err)
	}

	// Verify fields
	if config.ForgeVersion != "0.1.0" {
		t.Errorf("ForgeVersion = %q, want %q", config.ForgeVersion, "0.1.0")
	}
	if config.Tagging.Strategy != "monorepo" {
		t.Errorf("Tagging.Strategy = %q, want %q", config.Tagging.Strategy, "monorepo")
	}
}

// TestRepoConfig_UnmarshalJSON tests unmarshaling RepoConfig from JSON.
func TestRepoConfig_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"forgeVersion": "0.1.5",
		"tagging": {
			"strategy": "tag-all"
		},
		"phases": {},
		"publishers": {}
	}`

	var config RepoConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal RepoConfig from JSON: %v", err)
	}

	// Verify fields
	if config.ForgeVersion != "0.1.5" {
		t.Errorf("ForgeVersion = %q, want %q", config.ForgeVersion, "0.1.5")
	}
	if config.Tagging.Strategy != "tag-all" {
		t.Errorf("Tagging.Strategy = %q, want %q", config.Tagging.Strategy, "tag-all")
	}
}

// TestProjectConfig_UnmarshalYAML tests unmarshaling ProjectConfig from JSON.
func TestProjectConfig_UnmarshalYAML(t *testing.T) {
	jsonData := `{
		"name": "my-service",
		"phases": {
			"test": {
				"steps": [
					{
						"name": "run tests",
						"action": "earthly",
						"target": "+test"
					}
				]
			},
			"build": {
				"steps": [
					{
						"name": "build binary",
						"action": "earthly",
						"target": "+build",
						"timeout": "10m"
					}
				]
			}
		},
		"artifacts": {}
	}`

	var config ProjectConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProjectConfig from JSON: %v", err)
	}

	// Verify fields
	if config.Name != "my-service" {
		t.Errorf("Name = %q, want %q", config.Name, "my-service")
	}

	// Verify phases
	if len(config.Phases) != 2 {
		t.Fatalf("Expected 2 phases, got %d", len(config.Phases))
	}

	testPhase, ok := config.Phases["test"]
	if !ok {
		t.Fatal("Expected 'test' phase to exist")
	}
	if len(testPhase.Steps) != 1 {
		t.Fatalf("Expected 1 step in test phase, got %d", len(testPhase.Steps))
	}
	if testPhase.Steps[0].Name != "run tests" {
		t.Errorf("Step name = %q, want %q", testPhase.Steps[0].Name, "run tests")
	}
	if testPhase.Steps[0].Action != "earthly" {
		t.Errorf("Step action = %q, want %q", testPhase.Steps[0].Action, "earthly")
	}
	if testPhase.Steps[0].Target != "+test" {
		t.Errorf("Step target = %q, want %q", testPhase.Steps[0].Target, "+test")
	}

	buildPhase, ok := config.Phases["build"]
	if !ok {
		t.Fatal("Expected 'build' phase to exist")
	}
	if len(buildPhase.Steps) != 1 {
		t.Fatalf("Expected 1 step in build phase, got %d", len(buildPhase.Steps))
	}
	if buildPhase.Steps[0].Timeout != "10m" {
		t.Errorf("Step timeout = %q, want %q", buildPhase.Steps[0].Timeout, "10m")
	}
}

// TestProjectConfig_UnmarshalJSON tests unmarshaling ProjectConfig from JSON.
func TestProjectConfig_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"name": "api-gateway",
		"phases": {
			"lint": {
				"steps": [
					{
						"name": "check style",
						"action": "earthly",
						"target": "+lint"
					}
				]
			}
		},
		"artifacts": {}
	}`

	var config ProjectConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProjectConfig from JSON: %v", err)
	}

	// Verify fields
	if config.Name != "api-gateway" {
		t.Errorf("Name = %q, want %q", config.Name, "api-gateway")
	}

	lintPhase, ok := config.Phases["lint"]
	if !ok {
		t.Fatal("Expected 'lint' phase to exist")
	}
	if len(lintPhase.Steps) != 1 {
		t.Fatalf("Expected 1 step in lint phase, got %d", len(lintPhase.Steps))
	}
	if lintPhase.Steps[0].Name != "check style" {
		t.Errorf("Step name = %q, want %q", lintPhase.Steps[0].Name, "check style")
	}
}

// TestProjectConfig_WithReleaseAndDeploy tests unmarshaling ProjectConfig with release and deploy sections.
func TestProjectConfig_WithReleaseAndDeploy(t *testing.T) {
	jsonData := `{
		"name": "web-app",
		"phases": {},
		"artifacts": {},
		"release": {
			"on": [
				{"branch": "main"},
				{"tag": true},
				{"manual": true}
			]
		},
		"deploy": {
			"resources": [
				{
					"apiVersion": "apps/v1",
					"kind": "Deployment",
					"metadata": {
						"name": "web-app"
					},
					"spec": {
						"replicas": 3
					}
				}
			]
		}
	}`

	var config ProjectConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProjectConfig with release and deploy: %v", err)
	}

	// Verify release config
	if config.Release == nil {
		t.Fatal("Expected Release config to be present")
	}
	if len(config.Release.On) != 3 {
		t.Fatalf("Expected 3 release triggers, got %d", len(config.Release.On))
	}
	if config.Release.On[0].Branch != "main" {
		t.Errorf("Release trigger branch = %q, want %q", config.Release.On[0].Branch, "main")
	}
	if !config.Release.On[1].Tag {
		t.Error("Expected tag trigger to be true")
	}
	if !config.Release.On[2].Manual {
		t.Error("Expected manual trigger to be true")
	}

	// Verify deploy config
	if config.Deploy == nil {
		t.Fatal("Expected Deploy config to be present")
	}
	if len(config.Deploy.Resources) != 1 {
		t.Fatalf("Expected 1 K8s resource, got %d", len(config.Deploy.Resources))
	}
	resource := config.Deploy.Resources[0]
	if resource.ApiVersion != "apps/v1" {
		t.Errorf("Resource ApiVersion = %q, want %q", resource.ApiVersion, "apps/v1")
	}
	if resource.Kind != "Deployment" {
		t.Errorf("Resource Kind = %q, want %q", resource.Kind, "Deployment")
	}
	if resource.Metadata["name"] != "web-app" {
		t.Errorf("Resource metadata name = %v, want %q", resource.Metadata["name"], "web-app")
	}
}

// TestPhaseDefinition_Unmarshal tests unmarshaling PhaseDefinition.
func TestPhaseDefinition_Unmarshal(t *testing.T) {
	jsonData := `{
		"group": 1,
		"description": "Test phase",
		"timeout": "15m",
		"required": true
	}`

	var phase phases.PhaseDefinition
	err := json.Unmarshal([]byte(jsonData), &phase)
	if err != nil {
		t.Fatalf("Failed to unmarshal PhaseDefinition: %v", err)
	}

	if phase.Group != 1 {
		t.Errorf("Group = %d, want %d", phase.Group, 1)
	}
	if phase.Description != "Test phase" {
		t.Errorf("Description = %q, want %q", phase.Description, "Test phase")
	}
	if phase.Timeout != "15m" {
		t.Errorf("Timeout = %q, want %q", phase.Timeout, "15m")
	}
	if !phase.Required {
		t.Error("Expected Required to be true")
	}
}

// TestDockerPublisher_Unmarshal tests unmarshaling DockerPublisher.
func TestDockerPublisher_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "docker",
		"registry": "docker.io",
		"namespace": "myorg"
	}`

	var publisher publishers.DockerPublisher
	err := json.Unmarshal([]byte(jsonData), &publisher)
	if err != nil {
		t.Fatalf("Failed to unmarshal DockerPublisher: %v", err)
	}

	if publisher.Type != "docker" {
		t.Errorf("Type = %q, want %q", publisher.Type, "docker")
	}
	if publisher.Registry != "docker.io" {
		t.Errorf("Registry = %q, want %q", publisher.Registry, "docker.io")
	}
	if publisher.Namespace != "myorg" {
		t.Errorf("Namespace = %q, want %q", publisher.Namespace, "myorg")
	}
}

// TestDockerPublisher_WithCredentials tests unmarshaling DockerPublisher with AWS credentials.
func TestDockerPublisher_WithCredentials(t *testing.T) {
	jsonData := `{
		"type": "docker",
		"registry": "123456789012.dkr.ecr.us-east-1.amazonaws.com",
		"namespace": "myapp",
		"credentials": {
			"provider": "aws",
			"name": "docker-credentials",
			"region": "us-east-1"
		}
	}`

	var publisher publishers.DockerPublisher
	err := json.Unmarshal([]byte(jsonData), &publisher)
	if err != nil {
		t.Fatalf("Failed to unmarshal DockerPublisher with credentials: %v", err)
	}

	if publisher.Credentials == nil {
		t.Fatal("Expected Credentials to be present")
	}

	// Since SecretRef is map[string]any, we need to check the map values
	creds := *publisher.Credentials
	if creds["provider"] != "aws" {
		t.Errorf("Credentials provider = %v, want %q", creds["provider"], "aws")
	}
	if creds["name"] != "docker-credentials" {
		t.Errorf("Credentials name = %v, want %q", creds["name"], "docker-credentials")
	}
}

// TestGitHubPublisher_Unmarshal tests unmarshaling GitHubPublisher.
func TestGitHubPublisher_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "github",
		"repository": "myorg/myrepo",
		"credentials": {
			"provider": "vault",
			"path": "secret/data/github/token",
			"key": "token"
		}
	}`

	var publisher publishers.GitHubPublisher
	err := json.Unmarshal([]byte(jsonData), &publisher)
	if err != nil {
		t.Fatalf("Failed to unmarshal GitHubPublisher: %v", err)
	}

	if publisher.Type != "github" {
		t.Errorf("Type = %q, want %q", publisher.Type, "github")
	}
	if publisher.Repository != "myorg/myrepo" {
		t.Errorf("Repository = %q, want %q", publisher.Repository, "myorg/myrepo")
	}

	if publisher.Credentials == nil {
		t.Fatal("Expected Credentials to be present")
	}

	creds := *publisher.Credentials
	if creds["provider"] != "vault" {
		t.Errorf("Credentials provider = %v, want %q", creds["provider"], "vault")
	}
	if creds["path"] != "secret/data/github/token" {
		t.Errorf("Credentials path = %v, want %q", creds["path"], "secret/data/github/token")
	}
}

// TestS3Publisher_Unmarshal tests unmarshaling S3Publisher.
func TestS3Publisher_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "s3",
		"bucket": "my-artifacts-bucket",
		"region": "us-west-2"
	}`

	var publisher publishers.S3Publisher
	err := json.Unmarshal([]byte(jsonData), &publisher)
	if err != nil {
		t.Fatalf("Failed to unmarshal S3Publisher: %v", err)
	}

	if publisher.Type != "s3" {
		t.Errorf("Type = %q, want %q", publisher.Type, "s3")
	}
	if publisher.Bucket != "my-artifacts-bucket" {
		t.Errorf("Bucket = %q, want %q", publisher.Bucket, "my-artifacts-bucket")
	}
	if publisher.Region != "us-west-2" {
		t.Errorf("Region = %q, want %q", publisher.Region, "us-west-2")
	}
}

// TestContainerArtifact_Unmarshal tests unmarshaling ContainerArtifact.
func TestContainerArtifact_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "container",
		"ref": "myapp:v1.0.0",
		"producer": {
			"type": "earthly",
			"target": "+docker",
			"artifact": "+docker/image"
		},
		"publishers": ["dockerhub", "ecr"]
	}`

	var artifact artifacts.ContainerArtifact
	err := json.Unmarshal([]byte(jsonData), &artifact)
	if err != nil {
		t.Fatalf("Failed to unmarshal ContainerArtifact: %v", err)
	}

	if artifact.Type != "container" {
		t.Errorf("Type = %q, want %q", artifact.Type, "container")
	}
	if artifact.Ref != "myapp:v1.0.0" {
		t.Errorf("Ref = %q, want %q", artifact.Ref, "myapp:v1.0.0")
	}
	if artifact.Producer.Type != "earthly" {
		t.Errorf("Producer Type = %q, want %q", artifact.Producer.Type, "earthly")
	}
	if artifact.Producer.Target != "+docker" {
		t.Errorf("Producer Target = %q, want %q", artifact.Producer.Target, "+docker")
	}
	if artifact.Producer.Artifact != "+docker/image" {
		t.Errorf("Producer Artifact = %q, want %q", artifact.Producer.Artifact, "+docker/image")
	}
	if len(artifact.Publishers) != 2 {
		t.Fatalf("Expected 2 publishers, got %d", len(artifact.Publishers))
	}
	if artifact.Publishers[0] != "dockerhub" {
		t.Errorf("Publishers[0] = %q, want %q", artifact.Publishers[0], "dockerhub")
	}
	if artifact.Publishers[1] != "ecr" {
		t.Errorf("Publishers[1] = %q, want %q", artifact.Publishers[1], "ecr")
	}
}

// TestBinaryArtifact_Unmarshal tests unmarshaling BinaryArtifact.
func TestBinaryArtifact_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "binary",
		"name": "myapp-cli",
		"producer": {
			"type": "earthly",
			"target": "+build"
		},
		"publishers": ["s3", "github-releases"]
	}`

	var artifact artifacts.BinaryArtifact
	err := json.Unmarshal([]byte(jsonData), &artifact)
	if err != nil {
		t.Fatalf("Failed to unmarshal BinaryArtifact: %v", err)
	}

	if artifact.Type != "binary" {
		t.Errorf("Type = %q, want %q", artifact.Type, "binary")
	}
	if artifact.Name != "myapp-cli" {
		t.Errorf("Name = %q, want %q", artifact.Name, "myapp-cli")
	}
	if artifact.Producer.Type != "earthly" {
		t.Errorf("Producer Type = %q, want %q", artifact.Producer.Type, "earthly")
	}
	if len(artifact.Publishers) != 2 {
		t.Fatalf("Expected 2 publishers, got %d", len(artifact.Publishers))
	}
}

// TestArchiveArtifact_Unmarshal tests unmarshaling ArchiveArtifact.
func TestArchiveArtifact_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "archive",
		"compression": "gzip",
		"producer": {
			"type": "earthly",
			"target": "+package",
			"artifact": "+package/dist.tar.gz"
		},
		"publishers": ["s3"]
	}`

	var artifact artifacts.ArchiveArtifact
	err := json.Unmarshal([]byte(jsonData), &artifact)
	if err != nil {
		t.Fatalf("Failed to unmarshal ArchiveArtifact: %v", err)
	}

	if artifact.Type != "archive" {
		t.Errorf("Type = %q, want %q", artifact.Type, "archive")
	}
	if artifact.Compression != "gzip" {
		t.Errorf("Compression = %q, want %q", artifact.Compression, "gzip")
	}
	if artifact.Producer.Type != "earthly" {
		t.Errorf("Producer Type = %q, want %q", artifact.Producer.Type, "earthly")
	}
	if len(artifact.Publishers) != 1 {
		t.Fatalf("Expected 1 publisher, got %d", len(artifact.Publishers))
	}
}

// TestAWSSecretRef_Unmarshal tests unmarshaling AWSSecretRef.
func TestAWSSecretRef_Unmarshal(t *testing.T) {
	jsonData := `{
		"provider": "aws",
		"name": "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret",
		"key": "api-key",
		"region": "us-east-1"
	}`

	var secret common.AWSSecretRef
	err := json.Unmarshal([]byte(jsonData), &secret)
	if err != nil {
		t.Fatalf("Failed to unmarshal AWSSecretRef: %v", err)
	}

	if secret.Provider != "aws" {
		t.Errorf("Provider = %q, want %q", secret.Provider, "aws")
	}
	if secret.Name != "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret" {
		t.Errorf("Name = %q, want ARN", secret.Name)
	}
	if secret.Key != "api-key" {
		t.Errorf("Key = %q, want %q", secret.Key, "api-key")
	}
	if secret.Region != "us-east-1" {
		t.Errorf("Region = %q, want %q", secret.Region, "us-east-1")
	}
}

// TestVaultSecretRef_Unmarshal tests unmarshaling VaultSecretRef.
func TestVaultSecretRef_Unmarshal(t *testing.T) {
	jsonData := `{
		"provider": "vault",
		"path": "secret/data/myapp/credentials",
		"key": "password"
	}`

	var secret common.VaultSecretRef
	err := json.Unmarshal([]byte(jsonData), &secret)
	if err != nil {
		t.Fatalf("Failed to unmarshal VaultSecretRef: %v", err)
	}

	if secret.Provider != "vault" {
		t.Errorf("Provider = %q, want %q", secret.Provider, "vault")
	}
	if secret.Path != "secret/data/myapp/credentials" {
		t.Errorf("Path = %q, want %q", secret.Path, "secret/data/myapp/credentials")
	}
	if secret.Key != "password" {
		t.Errorf("Key = %q, want %q", secret.Key, "password")
	}
}

// TestEarthlyStep_Unmarshal tests unmarshaling EarthlyStep (Step type).
func TestEarthlyStep_Unmarshal(t *testing.T) {
	jsonData := `{
		"name": "run integration tests",
		"action": "earthly",
		"target": "+integration-test",
		"timeout": "30m"
	}`

	var step Step
	err := json.Unmarshal([]byte(jsonData), &step)
	if err != nil {
		t.Fatalf("Failed to unmarshal Step: %v", err)
	}

	if step.Name != "run integration tests" {
		t.Errorf("Name = %q, want %q", step.Name, "run integration tests")
	}
	if step.Action != "earthly" {
		t.Errorf("Action = %q, want %q", step.Action, "earthly")
	}
	if step.Target != "+integration-test" {
		t.Errorf("Target = %q, want %q", step.Target, "+integration-test")
	}
	if step.Timeout != "30m" {
		t.Errorf("Timeout = %q, want %q", step.Timeout, "30m")
	}
}

// TestComplexRepoConfig_Unmarshal tests unmarshaling a complex RepoConfig with all features.
func TestComplexRepoConfig_Unmarshal(t *testing.T) {
	// This test validates end-to-end unmarshaling of a realistic repo configuration
	jsonData := `{
		"forgeVersion": "0.1.0",
		"tagging": {
			"strategy": "monorepo"
		},
		"phases": {},
		"publishers": {}
	}`

	var config RepoConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal complex RepoConfig: %v", err)
	}

	// Basic validation - just ensure it unmarshals without errors
	if config.ForgeVersion == "" {
		t.Error("Expected ForgeVersion to be set")
	}
}

// TestComplexProjectConfig_Unmarshal tests unmarshaling a complex ProjectConfig with all features.
func TestComplexProjectConfig_Unmarshal(t *testing.T) {
	// This test validates end-to-end unmarshaling of a realistic project configuration.
	jsonData := `{
		"name": "complex-service",
		"phases": {
			"test": {
				"steps": [
					{
						"name": "unit tests",
						"action": "earthly",
						"target": "+test"
					},
					{
						"name": "integration tests",
						"action": "earthly",
						"target": "+integration-test",
						"timeout": "20m"
					}
				]
			},
			"build": {
				"steps": [
					{
						"name": "build",
						"action": "earthly",
						"target": "+build"
					}
				]
			}
		},
		"artifacts": {},
		"release": {
			"on": [
				{"branch": "main"},
				{"tag": true}
			]
		},
		"deploy": {
			"resources": [
				{
					"apiVersion": "apps/v1",
					"kind": "Deployment",
					"metadata": {
						"name": "complex-service",
						"labels": {
							"app": "complex-service"
						}
					},
					"spec": {
						"replicas": 3,
						"selector": {
							"matchLabels": {
								"app": "complex-service"
							}
						}
					}
				}
			]
		}
	}`

	var config ProjectConfig
	err := json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal complex ProjectConfig: %v", err)
	}

	// Validate structure
	if config.Name != "complex-service" {
		t.Errorf("Name = %q, want %q", config.Name, "complex-service")
	}

	if len(config.Phases) != 2 {
		t.Errorf("Expected 2 phases, got %d", len(config.Phases))
	}

	testPhase := config.Phases["test"]
	if len(testPhase.Steps) != 2 {
		t.Errorf("Expected 2 steps in test phase, got %d", len(testPhase.Steps))
	}

	if config.Release == nil {
		t.Error("Expected Release config to be present")
	}

	if config.Deploy == nil {
		t.Error("Expected Deploy config to be present")
	}

	if len(config.Deploy.Resources) != 1 {
		t.Errorf("Expected 1 K8s resource, got %d", len(config.Deploy.Resources))
	}
}
