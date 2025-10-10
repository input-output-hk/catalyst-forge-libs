package fstest

import (
	"bytes"
	"os"
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/fs/core"
)

// TestWriteFS tests write operations: Create, OpenFile, WriteFile, Mkdir, MkdirAll.
// Tests file creation, directory creation, and various write scenarios.
func TestWriteFS(t *testing.T, filesystem core.FS) {
	// Run all subtests
	t.Run("CreateAndWrite", func(t *testing.T) {
		testWriteFSCreate(t, filesystem)
	})
	t.Run("WriteFile", func(t *testing.T) {
		testWriteFSWriteFile(t, filesystem)
	})
	t.Run("OpenFile", func(t *testing.T) {
		testWriteFSOpenFile(t, filesystem)
	})
	t.Run("Mkdir", func(t *testing.T) {
		testWriteFSMkdir(t, filesystem)
	})
	t.Run("MkdirAll", func(t *testing.T) {
		testWriteFSMkdirAll(t, filesystem)
	})
	t.Run("CreateInNonExistentDir", func(t *testing.T) {
		testWriteFSCreateError(t, filesystem)
	})
}

// testWriteFSCreate tests Create() new file, write data, verify contents.
func testWriteFSCreate(t *testing.T, filesystem core.FS) {
	testData := []byte("test data for Create")

	// Create a new file
	f, err := filesystem.Create("testfile.txt")
	if err != nil {
		t.Fatalf("Create(%q): got error %v, want nil", "testfile.txt", err)
	}

	// Write data to the file
	n, err := f.Write(testData)
	if err != nil {
		_ = f.Close()
		t.Fatalf("Write(): got error %v, want nil", err)
	}
	if n != len(testData) {
		_ = f.Close()
		t.Fatalf("Write(): wrote %d bytes, want %d", n, len(testData))
	}

	// Close the file
	if err := f.Close(); err != nil {
		t.Fatalf("Close(): got error %v, want nil", err)
	}

	// Verify the file was created and contains the correct data
	data, err := filesystem.ReadFile("testfile.txt")
	if err != nil {
		t.Errorf("ReadFile(%q): got error %v, want nil", "testfile.txt", err)
		return
	}
	if !bytes.Equal(data, testData) {
		t.Errorf("ReadFile(%q): got %q, want %q", "testfile.txt", data, testData)
	}
}

// testWriteFSWriteFile tests WriteFile() convenience method.
// Tests that WriteFile creates the file with perm parameter, but doesn't verify
// actual permissions on disk (per testing philosophy lines 297-313).
func testWriteFSWriteFile(t *testing.T, filesystem core.FS) {
	testData := []byte("test data for WriteFile")

	// Write file with perm parameter
	err := filesystem.WriteFile("writefile.txt", testData, 0644)
	if err != nil {
		t.Fatalf("WriteFile(%q): got error %v, want nil", "writefile.txt", err)
	}

	// Verify the file was created and contains the correct data
	data, err := filesystem.ReadFile("writefile.txt")
	if err != nil {
		t.Errorf("ReadFile(%q): got error %v, want nil", "writefile.txt", err)
		return
	}
	if !bytes.Equal(data, testData) {
		t.Errorf("ReadFile(%q): got %q, want %q", "writefile.txt", data, testData)
	}

	// Verify the file exists via Stat (basic validation)
	info, err := filesystem.Stat("writefile.txt")
	if err != nil {
		t.Errorf("Stat(%q): got error %v, want nil", "writefile.txt", err)
		return
	}
	if info.IsDir() {
		t.Errorf("Stat(%q): IsDir() = true, want false", "writefile.txt")
	}

	// Note: We do NOT verify actual permissions on disk.
	// This is per testing philosophy: test interface contract, not backend-specific behavior.
	// Different providers handle perm differently (S3 ignores it, local applies it).
}

// testWriteFSOpenFile tests OpenFile() with various flags.
func testWriteFSOpenFile(t *testing.T, filesystem core.FS) {
	testData := []byte("test data for OpenFile")

	// Test O_CREATE flag
	f, err := filesystem.OpenFile("openfile.txt", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile(%q, O_CREATE|O_WRONLY): got error %v, want nil", "openfile.txt", err)
	}

	// Write data
	n, err := f.Write(testData)
	if err != nil {
		_ = f.Close()
		t.Fatalf("Write(): got error %v, want nil", err)
	}
	if n != len(testData) {
		_ = f.Close()
		t.Fatalf("Write(): wrote %d bytes, want %d", n, len(testData))
	}

	if err := f.Close(); err != nil {
		t.Fatalf("Close(): got error %v, want nil", err)
	}

	// Verify the file was created
	data, err := filesystem.ReadFile("openfile.txt")
	if err != nil {
		t.Errorf("ReadFile(%q): got error %v, want nil", "openfile.txt", err)
		return
	}
	if !bytes.Equal(data, testData) {
		t.Errorf("ReadFile(%q): got %q, want %q", "openfile.txt", data, testData)
	}

	// Test O_TRUNC flag - open existing file and truncate
	newData := []byte("truncated")
	f, err = filesystem.OpenFile("openfile.txt", os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatalf("OpenFile(%q, O_WRONLY|O_TRUNC): got error %v, want nil", "openfile.txt", err)
	}

	_, err = f.Write(newData)
	if err != nil {
		_ = f.Close()
		t.Fatalf("Write(): got error %v, want nil", err)
	}

	if err := f.Close(); err != nil {
		t.Fatalf("Close(): got error %v, want nil", err)
	}

	// Verify the file was truncated and contains new data
	data, err = filesystem.ReadFile("openfile.txt")
	if err != nil {
		t.Errorf("ReadFile(%q): got error %v, want nil", "openfile.txt", err)
		return
	}
	if !bytes.Equal(data, newData) {
		t.Errorf("ReadFile(%q) after truncate: got %q, want %q", "openfile.txt", data, newData)
	}

	// Note: Detailed flag testing is delegated to TestOpenFileFlags.
	// This test just verifies basic OpenFile functionality.
}

// testWriteFSMkdir tests Mkdir() single directory creation.
func testWriteFSMkdir(t *testing.T, filesystem core.FS) {
	// Create a single directory
	err := filesystem.Mkdir("testdir", 0755)
	if err != nil {
		t.Fatalf("Mkdir(%q): got error %v, want nil", "testdir", err)
	}

	// Verify the directory was created
	info, err := filesystem.Stat("testdir")
	if err != nil {
		t.Errorf("Stat(%q): got error %v, want nil", "testdir", err)
		return
	}
	if !info.IsDir() {
		t.Errorf("Stat(%q): IsDir() = false, want true", "testdir")
	}
}

// testWriteFSMkdirAll tests MkdirAll() nested directory creation.
func testWriteFSMkdirAll(t *testing.T, filesystem core.FS) {
	// Create nested directories
	err := filesystem.MkdirAll("parent/child/grandchild", 0755)
	if err != nil {
		t.Fatalf("MkdirAll(%q): got error %v, want nil", "parent/child/grandchild", err)
	}

	// Verify the full path exists
	info, err := filesystem.Stat("parent/child/grandchild")
	if err != nil {
		t.Errorf("Stat(%q): got error %v, want nil", "parent/child/grandchild", err)
		return
	}
	if !info.IsDir() {
		t.Errorf("Stat(%q): IsDir() = false, want true", "parent/child/grandchild")
	}

	// Verify intermediate directories were created
	info, err = filesystem.Stat("parent")
	if err != nil {
		t.Errorf("Stat(%q): got error %v, want nil", "parent", err)
		return
	}
	if !info.IsDir() {
		t.Errorf("Stat(%q): IsDir() = false, want true", "parent")
	}

	info, err = filesystem.Stat("parent/child")
	if err != nil {
		t.Errorf("Stat(%q): got error %v, want nil", "parent/child", err)
		return
	}
	if !info.IsDir() {
		t.Errorf("Stat(%q): IsDir() = false, want true", "parent/child")
	}
}

// testWriteFSCreateError tests error case: Create in non-existent directory returns error.
func testWriteFSCreateError(t *testing.T, filesystem core.FS) {
	// Try to create a file in a non-existent directory
	_, err := filesystem.Create("nonexistent/testfile.txt")
	if err == nil {
		t.Errorf("Create(%q): got nil error, want error", "nonexistent/testfile.txt")
	}
	// Note: We don't check for a specific error type here because different
	// providers may return different error types for this scenario.
	// The important thing is that an error is returned.
}
