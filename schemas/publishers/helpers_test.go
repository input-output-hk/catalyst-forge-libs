package publishers

import (
	"encoding/json"
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/schema/common"
)

func TestPublisherConfig_Type(t *testing.T) {
	tests := []struct {
		name     string
		config   PublisherConfig
		expected string
	}{
		{
			name:     "docker type",
			config:   PublisherConfig{"type": "docker"},
			expected: "docker",
		},
		{
			name:     "github type",
			config:   PublisherConfig{"type": "github"},
			expected: "github",
		},
		{
			name:     "s3 type",
			config:   PublisherConfig{"type": "s3"},
			expected: "s3",
		},
		{
			name:     "missing type",
			config:   PublisherConfig{},
			expected: "",
		},
		{
			name:     "non-string type",
			config:   PublisherConfig{"type": 123},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.Type()
			if got != tt.expected {
				t.Errorf("Type() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPublisherConfig_AsDocker(t *testing.T) {
	t.Run("valid docker publisher", func(t *testing.T) {
		jsonData := `{
			"type": "docker",
			"registry": "docker.io",
			"namespace": "myorg"
		}`

		var config PublisherConfig
		if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		docker, ok := config.AsDocker()
		if !ok {
			t.Fatal("AsDocker() returned false, expected true")
		}
		if docker == nil {
			t.Fatal("AsDocker() returned nil publisher")
		}
		if docker.Type != "docker" {
			t.Errorf("Type = %q, want %q", docker.Type, "docker")
		}
		if docker.Registry != "docker.io" {
			t.Errorf("Registry = %q, want %q", docker.Registry, "docker.io")
		}
		if docker.Namespace != "myorg" {
			t.Errorf("Namespace = %q, want %q", docker.Namespace, "myorg")
		}
	})

	t.Run("docker publisher with credentials", func(t *testing.T) {
		jsonData := `{
			"type": "docker",
			"registry": "ghcr.io",
			"namespace": "myorg",
			"credentials": {
				"provider": "aws",
				"name": "docker-creds",
				"region": "us-east-1"
			}
		}`

		var config PublisherConfig
		if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		docker, ok := config.AsDocker()
		if !ok {
			t.Fatal("AsDocker() returned false, expected true")
		}
		if docker.Credentials == nil {
			t.Fatal("Expected credentials to be present")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		config := PublisherConfig{"type": "github"}
		docker, ok := config.AsDocker()
		if ok {
			t.Error("AsDocker() returned true for github type, expected false")
		}
		if docker != nil {
			t.Error("AsDocker() returned non-nil publisher for wrong type")
		}
	})
}

func TestPublisherConfig_AsGitHub(t *testing.T) {
	t.Run("valid github publisher", func(t *testing.T) {
		jsonData := `{
			"type": "github",
			"repository": "owner/repo"
		}`

		var config PublisherConfig
		if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		github, ok := config.AsGitHub()
		if !ok {
			t.Fatal("AsGitHub() returned false, expected true")
		}
		if github == nil {
			t.Fatal("AsGitHub() returned nil publisher")
		}
		if github.Type != "github" {
			t.Errorf("Type = %q, want %q", github.Type, "github")
		}
		if github.Repository != "owner/repo" {
			t.Errorf("Repository = %q, want %q", github.Repository, "owner/repo")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		config := PublisherConfig{"type": "s3"}
		github, ok := config.AsGitHub()
		if ok {
			t.Error("AsGitHub() returned true for s3 type, expected false")
		}
		if github != nil {
			t.Error("AsGitHub() returned non-nil publisher for wrong type")
		}
	})
}

func TestPublisherConfig_AsS3(t *testing.T) {
	t.Run("valid s3 publisher", func(t *testing.T) {
		jsonData := `{
			"type": "s3",
			"bucket": "my-bucket",
			"region": "us-west-2"
		}`

		var config PublisherConfig
		if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		s3, ok := config.AsS3()
		if !ok {
			t.Fatal("AsS3() returned false, expected true")
		}
		if s3 == nil {
			t.Fatal("AsS3() returned nil publisher")
		}
		if s3.Type != "s3" {
			t.Errorf("Type = %q, want %q", s3.Type, "s3")
		}
		if s3.Bucket != "my-bucket" {
			t.Errorf("Bucket = %q, want %q", s3.Bucket, "my-bucket")
		}
		if s3.Region != "us-west-2" {
			t.Errorf("Region = %q, want %q", s3.Region, "us-west-2")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		config := PublisherConfig{"type": "docker"}
		s3, ok := config.AsS3()
		if ok {
			t.Error("AsS3() returned true for docker type, expected false")
		}
		if s3 != nil {
			t.Error("AsS3() returned non-nil publisher for wrong type")
		}
	})
}

func TestPublisherConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    PublisherConfig
		wantError bool
	}{
		{
			name:      "valid docker type",
			config:    PublisherConfig{"type": "docker"},
			wantError: false,
		},
		{
			name:      "valid github type",
			config:    PublisherConfig{"type": "github"},
			wantError: false,
		},
		{
			name:      "valid s3 type",
			config:    PublisherConfig{"type": "s3"},
			wantError: false,
		},
		{
			name:      "missing type",
			config:    PublisherConfig{},
			wantError: true,
		},
		{
			name:      "unknown type",
			config:    PublisherConfig{"type": "unknown"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError && err == nil {
				t.Error("Validate() error = nil, wantError = true")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Validate() error = %v, wantError = false", err)
			}
		})
	}
}

func TestDockerPublisher_RoundTrip(t *testing.T) {
	// Test that we can marshal a DockerPublisher, convert to PublisherConfig, and back
	original := &DockerPublisher{
		Type:      "docker",
		Registry:  "docker.io",
		Namespace: "test",
		Credentials: &common.SecretRef{
			"provider": "aws",
			"name":     "creds",
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal to PublisherConfig
	var config PublisherConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to unmarshal to PublisherConfig: %v", err)
	}

	// Convert back to DockerPublisher
	result, ok := config.AsDocker()
	if !ok {
		t.Fatal("AsDocker() returned false")
	}

	// Verify fields
	if result.Type != original.Type {
		t.Errorf("Type = %q, want %q", result.Type, original.Type)
	}
	if result.Registry != original.Registry {
		t.Errorf("Registry = %q, want %q", result.Registry, original.Registry)
	}
	if result.Namespace != original.Namespace {
		t.Errorf("Namespace = %q, want %q", result.Namespace, original.Namespace)
	}
}
