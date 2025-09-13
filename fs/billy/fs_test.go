package billy

import (
	"os"
	"path/filepath"
	"testing"

	parentfs "github.com/input-output-hk/catalyst-forge-libs/fs"
)

func testMkdirAllStat(t *testing.T, fs parentfs.Filesystem, root string) {
	t.Helper()
	if err := fs.MkdirAll(filepath.Join(root, "a/b/c"), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	info, err := fs.Stat(filepath.Join(root, "a/b"))
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected directory, got file: %v", info.Name())
	}
}

func testCreateWriteReadRemove(t *testing.T, fs parentfs.Filesystem, root string) {
	t.Helper()
	p := filepath.Join(root, "file.txt")

	f, err := fs.Create(p)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	_ = f.Close()

	if e := fs.WriteFile(p, []byte("hello"), 0o644); e != nil {
		t.Fatalf("WriteFile failed: %v", e)
	}

	b, err := fs.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(b) != "hello" {
		t.Errorf("ReadFile = %q, want %q", string(b), "hello")
	}

	if e := fs.Remove(p); e != nil {
		t.Fatalf("Remove failed: %v", e)
	}
}

func testOpenAndOpenFile(t *testing.T, fs parentfs.Filesystem, root string) {
	t.Helper()
	p := filepath.Join(root, "open.txt")
	if e := fs.WriteFile(p, []byte("abc"), 0o644); e != nil {
		t.Fatalf("WriteFile failed: %v", e)
	}

	f, err := fs.Open(p)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	_ = f.Close()

	f2, err := fs.OpenFile(p, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	_ = f2.Close()
}

func testTempDirAndWalk(t *testing.T, fs parentfs.Filesystem, root string) {
	t.Helper()
	td, err := fs.TempDir(root, "pref-")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	if td == "" {
		t.Fatalf("TempDir returned empty path")
	}

	if e := fs.MkdirAll(filepath.Join(td, "x/y"), 0o755); e != nil {
		t.Fatalf("MkdirAll failed: %v", e)
	}
	if e := fs.WriteFile(filepath.Join(td, "x/y/z.txt"), []byte("z"), 0o644); e != nil {
		t.Fatalf("WriteFile failed: %v", e)
	}

	var seen int
	walkErr := fs.Walk(td, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Fatalf("walk callback error: %v", err)
		}
		seen++
		return nil
	})
	if walkErr != nil {
		t.Fatalf("Walk failed: %v", walkErr)
	}
	if seen < 2 {
		t.Errorf("Walk saw %d entries, want >= 2", seen)
	}
}

// runSuite runs a battery of consistency tests against a Filesystem impl.
func runSuite(t *testing.T, fs parentfs.Filesystem, root string) {
	t.Helper()
	testMkdirAllStat(t, fs, root)
	testCreateWriteReadRemove(t, fs, root)
	testOpenAndOpenFile(t, fs, root)
	testTempDirAndWalk(t, fs, root)
}

func TestInMemoryFS_Suite(t *testing.T) {
	runSuite(t, NewInMemoryFS(), "/")
}

func TestOSFS_Suite(t *testing.T) {
	root := t.TempDir()
	runSuite(t, NewOSFS(root), root)
}
