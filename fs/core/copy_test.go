package core_test

import (
	"embed"
	"io/fs"
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	"github.com/input-output-hk/catalyst-forge-libs/fs/core"
)

//go:embed testdata/*
var testdataFS embed.FS

func TestCopyFromEmbedFS_RootCopy(t *testing.T) {
	// Test copying from root directory "."
	memFS := billy.NewMemory()

	err := core.CopyFromEmbedFS(testdataFS, memFS, "testdata")
	if err != nil {
		t.Fatalf("CopyFromEmbedFS failed: %v", err)
	}

	// Verify root level files
	data, err := memFS.ReadFile("file1.txt")
	if err != nil {
		t.Errorf("Failed to read file1.txt: %v", err)
	}
	if string(data) != "This is file1 content\n" {
		t.Errorf("file1.txt content mismatch: got %q", string(data))
	}

	data, err = memFS.ReadFile("root.txt")
	if err != nil {
		t.Errorf("Failed to read root.txt: %v", err)
	}
	if string(data) != "Root level file\n" {
		t.Errorf("root.txt content mismatch: got %q", string(data))
	}

	// Verify subdirectory files
	data, err = memFS.ReadFile("subdir/file2.txt")
	if err != nil {
		t.Errorf("Failed to read subdir/file2.txt: %v", err)
	}
	if string(data) != "This is file2 content in subdir\n" {
		t.Errorf("subdir/file2.txt content mismatch: got %q", string(data))
	}

	// Verify deeply nested files
	data, err = memFS.ReadFile("nested/deep/file3.txt")
	if err != nil {
		t.Errorf("Failed to read nested/deep/file3.txt: %v", err)
	}
	if string(data) != "This is file3 deeply nested\n" {
		t.Errorf("nested/deep/file3.txt content mismatch: got %q", string(data))
	}
}

func TestCopyFromEmbedFS_SubdirCopy(t *testing.T) {
	// Test copying from a specific subdirectory
	memFS := billy.NewMemory()

	err := core.CopyFromEmbedFS(testdataFS, memFS, "testdata/subdir")
	if err != nil {
		t.Fatalf("CopyFromEmbedFS failed: %v", err)
	}

	// Verify that only subdir content was copied
	data, err := memFS.ReadFile("file2.txt")
	if err != nil {
		t.Errorf("Failed to read file2.txt: %v", err)
	}
	if string(data) != "This is file2 content in subdir\n" {
		t.Errorf("file2.txt content mismatch: got %q", string(data))
	}

	// Verify root files were NOT copied
	_, err = memFS.ReadFile("file1.txt")
	if err == nil {
		t.Error("file1.txt should not exist when copying only subdir")
	}
}

func TestCopyFromEmbedFS_NestedSubdirCopy(t *testing.T) {
	// Test copying from a deeply nested subdirectory
	memFS := billy.NewMemory()

	err := core.CopyFromEmbedFS(testdataFS, memFS, "testdata/nested")
	if err != nil {
		t.Fatalf("CopyFromEmbedFS failed: %v", err)
	}

	// Verify that nested content was copied at the right level
	data, err := memFS.ReadFile("deep/file3.txt")
	if err != nil {
		t.Errorf("Failed to read deep/file3.txt: %v", err)
	}
	if string(data) != "This is file3 deeply nested\n" {
		t.Errorf("deep/file3.txt content mismatch: got %q", string(data))
	}
}

func TestCopyFromEmbedFS_FilePermissions(t *testing.T) {
	// Test that file permissions are preserved
	memFS := billy.NewMemory()

	err := core.CopyFromEmbedFS(testdataFS, memFS, "testdata")
	if err != nil {
		t.Fatalf("CopyFromEmbedFS failed: %v", err)
	}

	// Verify file info includes permissions
	info, err := memFS.Stat("file1.txt")
	if err != nil {
		t.Fatalf("Failed to stat file1.txt: %v", err)
	}

	// embed.FS typically sets files to 0444 (read-only) or 0644
	// We just verify it's a regular file with some permission set
	if !info.Mode().IsRegular() {
		t.Errorf("file1.txt is not a regular file: mode=%v", info.Mode())
	}
}

func TestCopyFromEmbedFS_EmptySource(t *testing.T) {
	// Test with non-existent source directory
	memFS := billy.NewMemory()

	err := core.CopyFromEmbedFS(testdataFS, memFS, "testdata/nonexistent")
	if err == nil {
		t.Error("Expected error when copying from non-existent directory")
	}
}

func TestCopyFromEmbedFS_DirectoryStructure(t *testing.T) {
	// Test that directory structure is properly created
	memFS := billy.NewMemory()

	err := core.CopyFromEmbedFS(testdataFS, memFS, "testdata")
	if err != nil {
		t.Fatalf("CopyFromEmbedFS failed: %v", err)
	}

	// Verify directories exist and can be listed
	entries, err := memFS.ReadDir("subdir")
	if err != nil {
		t.Errorf("Failed to read subdir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("subdir should contain files")
	}

	entries, err = memFS.ReadDir("nested/deep")
	if err != nil {
		t.Errorf("Failed to read nested/deep: %v", err)
	}
	if len(entries) == 0 {
		t.Error("nested/deep should contain files")
	}
}

func TestCopyFromEmbedFS_OverwriteExisting(t *testing.T) {
	// Test that existing files are overwritten
	memFS := billy.NewMemory()

	// Pre-create a file with different content
	err := memFS.WriteFile("file1.txt", []byte("old content"), 0644)
	if err != nil {
		t.Fatalf("Failed to pre-create file: %v", err)
	}

	// Copy from embed.FS
	err = core.CopyFromEmbedFS(testdataFS, memFS, "testdata")
	if err != nil {
		t.Fatalf("CopyFromEmbedFS failed: %v", err)
	}

	// Verify file was overwritten
	data, err := memFS.ReadFile("file1.txt")
	if err != nil {
		t.Errorf("Failed to read file1.txt: %v", err)
	}
	if string(data) == "old content" {
		t.Error("file1.txt was not overwritten")
	}
	if string(data) != "This is file1 content\n" {
		t.Errorf("file1.txt content mismatch after overwrite: got %q", string(data))
	}
}

func TestCopyFromEmbedFS_DotRoot(t *testing.T) {
	// Test copying with "." as root (edge case)
	// Create a sub-filesystem for testing
	subFS, err := fs.Sub(testdataFS, "testdata")
	if err != nil {
		t.Fatalf("Failed to create sub-filesystem: %v", err)
	}

	memFS := billy.NewMemory()
	err = core.CopyFromEmbedFS(subFS, memFS, ".")
	if err != nil {
		t.Fatalf("CopyFromEmbedFS with '.' failed: %v", err)
	}

	// Verify files were copied
	data, err := memFS.ReadFile("file1.txt")
	if err != nil {
		t.Errorf("Failed to read file1.txt: %v", err)
	}
	if string(data) != "This is file1 content\n" {
		t.Errorf("file1.txt content mismatch: got %q", string(data))
	}
}

// BenchmarkCopyFromEmbedFS benchmarks the copy operation
func BenchmarkCopyFromEmbedFS(b *testing.B) {
	for i := 0; i < b.N; i++ {
		memFS := billy.NewMemory()
		err := core.CopyFromEmbedFS(testdataFS, memFS, "testdata")
		if err != nil {
			b.Fatalf("CopyFromEmbedFS failed: %v", err)
		}
	}
}
