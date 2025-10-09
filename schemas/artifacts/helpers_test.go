package artifacts

import (
	"encoding/json"
	"testing"
)

func TestArtifactSpec_Type(t *testing.T) {
	tests := []struct {
		name     string
		spec     ArtifactSpec
		expected string
	}{
		{
			name:     "container type",
			spec:     ArtifactSpec{"type": "container"},
			expected: "container",
		},
		{
			name:     "binary type",
			spec:     ArtifactSpec{"type": "binary"},
			expected: "binary",
		},
		{
			name:     "archive type",
			spec:     ArtifactSpec{"type": "archive"},
			expected: "archive",
		},
		{
			name:     "missing type",
			spec:     ArtifactSpec{},
			expected: "",
		},
		{
			name:     "non-string type",
			spec:     ArtifactSpec{"type": 123},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.Type()
			if got != tt.expected {
				t.Errorf("Type() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestArtifactSpec_AsContainer(t *testing.T) {
	t.Run("valid container artifact", func(t *testing.T) {
		jsonData := `{
			"type": "container",
			"ref": "myapp:v1.0.0",
			"producer": {
				"type": "earthly",
				"target": "+docker"
			},
			"publishers": ["dockerhub"]
		}`

		var spec ArtifactSpec
		if err := json.Unmarshal([]byte(jsonData), &spec); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		container, ok := spec.AsContainer()
		if !ok {
			t.Fatal("AsContainer() returned false, expected true")
		}
		if container == nil {
			t.Fatal("AsContainer() returned nil artifact")
		}
		if container.Type != "container" {
			t.Errorf("Type = %q, want %q", container.Type, "container")
		}
		if container.Ref != "myapp:v1.0.0" {
			t.Errorf("Ref = %q, want %q", container.Ref, "myapp:v1.0.0")
		}
		if container.Producer.Type != "earthly" {
			t.Errorf("Producer.Type = %q, want %q", container.Producer.Type, "earthly")
		}
		if len(container.Publishers) != 1 {
			t.Errorf("len(Publishers) = %d, want 1", len(container.Publishers))
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		spec := ArtifactSpec{"type": "binary"}
		container, ok := spec.AsContainer()
		if ok {
			t.Error("AsContainer() returned true for binary type, expected false")
		}
		if container != nil {
			t.Error("AsContainer() returned non-nil artifact for wrong type")
		}
	})
}

func TestArtifactSpec_AsBinary(t *testing.T) {
	t.Run("valid binary artifact", func(t *testing.T) {
		jsonData := `{
			"type": "binary",
			"name": "myapp-cli",
			"producer": {
				"type": "earthly",
				"target": "+build"
			},
			"publishers": ["s3", "github"]
		}`

		var spec ArtifactSpec
		if err := json.Unmarshal([]byte(jsonData), &spec); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		binary, ok := spec.AsBinary()
		if !ok {
			t.Fatal("AsBinary() returned false, expected true")
		}
		if binary == nil {
			t.Fatal("AsBinary() returned nil artifact")
		}
		if binary.Type != "binary" {
			t.Errorf("Type = %q, want %q", binary.Type, "binary")
		}
		if binary.Name != "myapp-cli" {
			t.Errorf("Name = %q, want %q", binary.Name, "myapp-cli")
		}
		if len(binary.Publishers) != 2 {
			t.Errorf("len(Publishers) = %d, want 2", len(binary.Publishers))
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		spec := ArtifactSpec{"type": "archive"}
		binary, ok := spec.AsBinary()
		if ok {
			t.Error("AsBinary() returned true for archive type, expected false")
		}
		if binary != nil {
			t.Error("AsBinary() returned non-nil artifact for wrong type")
		}
	})
}

func TestArtifactSpec_AsArchive(t *testing.T) {
	t.Run("valid archive artifact", func(t *testing.T) {
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

		var spec ArtifactSpec
		if err := json.Unmarshal([]byte(jsonData), &spec); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		archive, ok := spec.AsArchive()
		if !ok {
			t.Fatal("AsArchive() returned false, expected true")
		}
		if archive == nil {
			t.Fatal("AsArchive() returned nil artifact")
		}
		if archive.Type != "archive" {
			t.Errorf("Type = %q, want %q", archive.Type, "archive")
		}
		if archive.Compression != "gzip" {
			t.Errorf("Compression = %q, want %q", archive.Compression, "gzip")
		}
		if archive.Producer.Artifact != "+package/dist.tar.gz" {
			t.Errorf("Producer.Artifact = %q, want %q", archive.Producer.Artifact, "+package/dist.tar.gz")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		spec := ArtifactSpec{"type": "container"}
		archive, ok := spec.AsArchive()
		if ok {
			t.Error("AsArchive() returned true for container type, expected false")
		}
		if archive != nil {
			t.Error("AsArchive() returned non-nil artifact for wrong type")
		}
	})
}

func TestArtifactSpec_Validate(t *testing.T) {
	tests := []struct {
		name      string
		spec      ArtifactSpec
		wantError bool
	}{
		{
			name:      "valid container type",
			spec:      ArtifactSpec{"type": "container"},
			wantError: false,
		},
		{
			name:      "valid binary type",
			spec:      ArtifactSpec{"type": "binary"},
			wantError: false,
		},
		{
			name:      "valid archive type",
			spec:      ArtifactSpec{"type": "archive"},
			wantError: false,
		},
		{
			name:      "missing type",
			spec:      ArtifactSpec{},
			wantError: true,
		},
		{
			name:      "unknown type",
			spec:      ArtifactSpec{"type": "unknown"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if tt.wantError && err == nil {
				t.Error("Validate() error = nil, wantError = true")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Validate() error = %v, wantError = false", err)
			}
		})
	}
}

func TestContainerArtifact_RoundTrip(t *testing.T) {
	// Test that we can marshal a ContainerArtifact, convert to ArtifactSpec, and back
	original := &ContainerArtifact{
		Type: "container",
		Ref:  "myapp:latest",
		Producer: ArtifactProducer{
			Type:   "earthly",
			Target: "+docker",
		},
		Publishers: []string{"dockerhub", "ecr"},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal to ArtifactSpec
	var spec ArtifactSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("Failed to unmarshal to ArtifactSpec: %v", err)
	}

	// Convert back to ContainerArtifact
	result, ok := spec.AsContainer()
	if !ok {
		t.Fatal("AsContainer() returned false")
	}

	// Verify fields
	if result.Type != original.Type {
		t.Errorf("Type = %q, want %q", result.Type, original.Type)
	}
	if result.Ref != original.Ref {
		t.Errorf("Ref = %q, want %q", result.Ref, original.Ref)
	}
	if len(result.Publishers) != len(original.Publishers) {
		t.Errorf("len(Publishers) = %d, want %d", len(result.Publishers), len(original.Publishers))
	}
}
