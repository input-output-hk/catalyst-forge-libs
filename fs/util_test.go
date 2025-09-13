package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetAbs(t *testing.T) {
	t.Run("absolute path passthrough", func(t *testing.T) {
		abs := "/tmp"
		got, err := GetAbs(abs)
		if err != nil {
			t.Fatalf("GetAbs(%q) returned error: %v", abs, err)
		}
		if got != abs {
			t.Errorf("GetAbs(%q) = %q, want %q", abs, got, abs)
		}
	})

	t.Run("relative path conversion", func(t *testing.T) {
		got, err := GetAbs(".")
		if err != nil {
			t.Fatalf("GetAbs(.) returned error: %v", err)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("GetAbs(.) = %q, want absolute path", got)
		}
	})
}

func TestExists(t *testing.T) {
	t.Run("existing file returns true", func(t *testing.T) {
		f, err := os.CreateTemp("", "fs-exists-*")
		if err != nil {
			t.Fatalf("CreateTemp failed: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(f.Name()) })
		_ = f.Close()

		ok, err := Exists(f.Name())
		if err != nil {
			t.Fatalf("Exists(%q) returned error: %v", f.Name(), err)
		}
		if !ok {
			t.Errorf("Exists(%q) = false, want true", f.Name())
		}
	})

	t.Run("missing file returns false without error", func(t *testing.T) {
		p := filepath.Join(os.TempDir(), "does-not-exist-12345")
		ok, err := Exists(p)
		if err != nil {
			t.Fatalf("Exists(%q) returned error: %v", p, err)
		}
		if ok {
			t.Errorf("Exists(%q) = true, want false", p)
		}
	})
}
