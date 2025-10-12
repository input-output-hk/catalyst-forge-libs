package config

import (
	"context"
	"io/fs"
	"testing"
)

// mockFS is a minimal mock implementation of core.ReadFS for testing.
type mockFS struct{}

func (m *mockFS) Open(_ string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func (m *mockFS) Stat(_ string) (fs.FileInfo, error) {
	return nil, fs.ErrNotExist
}

func (m *mockFS) ReadDir(_ string) ([]fs.DirEntry, error) {
	return nil, fs.ErrNotExist
}

func (m *mockFS) ReadFile(_ string) ([]byte, error) {
	return nil, fs.ErrNotExist
}

func (m *mockFS) Exists(_ string) (bool, error) {
	return false, nil
}

// TestLoadRepoConfig tests the basic LoadRepoConfig function signature and behavior.
func TestLoadRepoConfig(t *testing.T) {
	ctx := context.Background()
	fs := &mockFS{}

	// mockFS doesn't provide real file content, so we expect an error
	_, err := LoadRepoConfig(ctx, fs, "repo.cue")
	if err == nil {
		t.Error("Expected error from empty mockFS, got nil")
	}
}

// TestLoadRepoConfigWithOptions tests the options-based loading function.
func TestLoadRepoConfigWithOptions(t *testing.T) {
	ctx := context.Background()
	fs := &mockFS{}

	tests := []struct {
		name string
		opts LoadOptions
	}{
		{
			name: "default options",
			opts: LoadOptions{},
		},
		{
			name: "skip validation",
			opts: LoadOptions{SkipValidation: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mockFS doesn't provide real file content, so we expect an error
			_, err := LoadRepoConfigWithOptions(ctx, fs, "repo.cue", tt.opts)
			if err == nil {
				t.Error("Expected error from empty mockFS, got nil")
			}
		})
	}
}

// TestLoadProjectConfig tests the basic LoadProjectConfig function signature and behavior.
func TestLoadProjectConfig(t *testing.T) {
	ctx := context.Background()
	fs := &mockFS{}

	// mockFS doesn't provide real file content, so we expect an error
	_, err := LoadProjectConfig(ctx, fs, "project.cue")
	if err == nil {
		t.Error("Expected error from empty mockFS, got nil")
	}
}

// TestLoadProjectConfigWithOptions tests the options-based loading function.
func TestLoadProjectConfigWithOptions(t *testing.T) {
	ctx := context.Background()
	fs := &mockFS{}

	tests := []struct {
		name string
		opts LoadOptions
	}{
		{
			name: "default options",
			opts: LoadOptions{},
		},
		{
			name: "skip validation",
			opts: LoadOptions{SkipValidation: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mockFS doesn't provide real file content, so we expect an error
			_, err := LoadProjectConfigWithOptions(ctx, fs, "project.cue", tt.opts)
			if err == nil {
				t.Error("Expected error from empty mockFS, got nil")
			}
		})
	}
}

// TestSupportedSchemaVersion verifies the schema version constant.
func TestSupportedSchemaVersion(t *testing.T) {
	expected := "0.1.0"
	if SupportedSchemaVersion != expected {
		t.Errorf("Expected SupportedSchemaVersion to be %q, got %q", expected, SupportedSchemaVersion)
	}
}

// TestLoadOptions verifies LoadOptions struct fields.
func TestLoadOptions(t *testing.T) {
	tests := []struct {
		name           string
		opts           LoadOptions
		wantSkipValidation bool
	}{
		{
			name:           "default zero value",
			opts:           LoadOptions{},
			wantSkipValidation: false,
		},
		{
			name:           "skip validation enabled",
			opts:           LoadOptions{SkipValidation: true},
			wantSkipValidation: true,
		},
		{
			name:           "skip validation disabled",
			opts:           LoadOptions{SkipValidation: false},
			wantSkipValidation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts.SkipValidation != tt.wantSkipValidation {
				t.Errorf("Expected SkipValidation to be %v, got %v", tt.wantSkipValidation, tt.opts.SkipValidation)
			}
		})
	}
}
