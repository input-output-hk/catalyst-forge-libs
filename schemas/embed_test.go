package schema

import (
	"testing"
)

// TestCueModuleEmbedded verifies that the CUE module is properly embedded.
func TestCueModuleEmbedded(t *testing.T) {
	// Test that we can read the root directory
	entries, err := CueModule.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read embedded root directory: %v", err)
	}

	// Verify we have some entries
	if len(entries) == 0 {
		t.Fatal("Expected embedded files but got none")
	}

	// Check for expected files/directories
	expectedFiles := []string{"cue.mod", "repo.cue", "project.cue", "common", "publishers", "phases", "artifacts"}
	foundFiles := make(map[string]bool)
	for _, entry := range entries {
		foundFiles[entry.Name()] = true
	}

	for _, expected := range expectedFiles {
		if !foundFiles[expected] {
			t.Errorf("Expected file/directory %q not found in embedded FS", expected)
		}
	}
}

// TestCueModuleReadFile verifies that we can read embedded files.
func TestCueModuleReadFile(t *testing.T) {
	// Try to read repo.cue
	data, err := CueModule.ReadFile("repo.cue")
	if err != nil {
		t.Fatalf("Failed to read repo.cue: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("repo.cue is empty")
	}

	// Check it contains expected content
	content := string(data)
	if !contains(content, "package schema") {
		t.Error("repo.cue doesn't contain 'package schema'")
	}
	if !contains(content, "#RepoConfig") {
		t.Error("repo.cue doesn't contain '#RepoConfig'")
	}
}

// TestCueModuleSubdirectories verifies that subdirectories are embedded.
func TestCueModuleSubdirectories(t *testing.T) {
	subdirs := []string{"cue.mod", "common", "publishers", "phases", "artifacts"}

	for _, subdir := range subdirs {
		entries, err := CueModule.ReadDir(subdir)
		if err != nil {
			t.Errorf("Failed to read subdirectory %q: %v", subdir, err)
			continue
		}

		if len(entries) == 0 {
			t.Errorf("Subdirectory %q is empty", subdir)
		}
	}
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
