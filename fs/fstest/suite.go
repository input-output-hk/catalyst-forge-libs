// Package fstest provides a conformance test suite for validating filesystem
// provider implementations against the core.FS interface contracts.
//
// This package contains test functions that can be imported and executed by
// filesystem provider packages to verify they correctly implement the core.FS
// interface and its optional extensions (MetadataFS, SymlinkFS, TempFS).
//
// The test suite is designed to validate interface contracts, not backend-specific
// behavior. Different providers have different capabilities, and the tests verify
// that all providers honor the interface contract while gracefully handling
// documented differences.
//
// Example usage:
//
//	func TestMyProvider(t *testing.T) {
//	    fstest.TestSuite(t, func() core.FS {
//	        return myprovider.New()
//	    })
//	}
package fstest

import (
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/fs/core"
)

// TestSuite runs all applicable conformance tests against a filesystem.
// The newFS function should return a fresh, empty filesystem for each test.
// Tests will create/modify files, so each invocation should start clean.
func TestSuite(t *testing.T, newFS func() core.FS) {
	TestSuiteWithSkip(t, newFS, nil)
}

// TestSuiteWithSkip runs conformance tests with optional test skipping.
// The skipTests parameter is a slice of test names to skip (e.g., "WriteFS/CreateInNonExistentDir").
// This is useful for providers with known behavioral differences from the standard contract.
func TestSuiteWithSkip(t *testing.T, newFS func() core.FS, skipTests []string) {
	// Helper to check if a test should be skipped
	shouldSkip := func(testName string) bool {
		for _, skip := range skipTests {
			if skip == testName {
				return true
			}
		}
		return false
	}

	// Run all core FS interface tests with fresh filesystem instances
	t.Run("ReadFS", func(t *testing.T) {
		if shouldSkip("ReadFS") {
			t.Skip("Skipped by provider configuration")
			return
		}
		TestReadFS(t, newFS())
	})

	t.Run("WriteFS", func(t *testing.T) {
		if shouldSkip("WriteFS") {
			t.Skip("Skipped by provider configuration")
			return
		}
		TestWriteFSWithSkip(t, newFS(), skipTests)
	})

	t.Run("ManageFS", func(t *testing.T) {
		if shouldSkip("ManageFS") {
			t.Skip("Skipped by provider configuration")
			return
		}
		TestManageFS(t, newFS())
	})

	t.Run("WalkFS", func(t *testing.T) {
		if shouldSkip("WalkFS") {
			t.Skip("Skipped by provider configuration")
			return
		}
		TestWalkFS(t, newFS())
	})

	t.Run("ChrootFS", func(t *testing.T) {
		if shouldSkip("ChrootFS") {
			t.Skip("Skipped by provider configuration")
			return
		}
		TestChrootFS(t, newFS())
	})

	// OpenFileFlags test is intentionally not included in TestSuite
	// because it requires provider-specific supportedFlags parameter.
	// Providers should call TestOpenFileFlags directly with their supported flags.

	// Run optional FS-level interface tests
	t.Run("MetadataFS", func(t *testing.T) {
		if shouldSkip("MetadataFS") {
			t.Skip("Skipped by provider configuration")
			return
		}
		TestMetadataFS(t, newFS())
	})

	t.Run("SymlinkFS", func(t *testing.T) {
		if shouldSkip("SymlinkFS") {
			t.Skip("Skipped by provider configuration")
			return
		}
		TestSymlinkFS(t, newFS())
	})

	t.Run("TempFS", func(t *testing.T) {
		if shouldSkip("TempFS") {
			t.Skip("Skipped by provider configuration")
			return
		}
		TestTempFS(t, newFS())
	})

	// Run optional File-level capability tests
	t.Run("FileCapabilities", func(t *testing.T) {
		if shouldSkip("FileCapabilities") {
			t.Skip("Skipped by provider configuration")
			return
		}
		TestFileCapabilitiesWithSkip(t, newFS(), skipTests)
	})
}
