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
	// Run all core FS interface tests with fresh filesystem instances
	t.Run("ReadFS", func(t *testing.T) {
		TestReadFS(t, newFS())
	})

	t.Run("WriteFS", func(t *testing.T) {
		TestWriteFS(t, newFS())
	})

	t.Run("ManageFS", func(t *testing.T) {
		TestManageFS(t, newFS())
	})

	t.Run("WalkFS", func(t *testing.T) {
		TestWalkFS(t, newFS())
	})

	t.Run("ChrootFS", func(t *testing.T) {
		TestChrootFS(t, newFS())
	})

	// OpenFileFlags test is intentionally not included in TestSuite
	// because it requires provider-specific supportedFlags parameter.
	// Providers should call TestOpenFileFlags directly with their supported flags.

	// Run optional FS-level interface tests
	t.Run("MetadataFS", func(t *testing.T) {
		TestMetadataFS(t, newFS())
	})

	t.Run("SymlinkFS", func(t *testing.T) {
		TestSymlinkFS(t, newFS())
	})

	t.Run("TempFS", func(t *testing.T) {
		TestTempFS(t, newFS())
	})

	// Run optional File-level capability tests
	t.Run("FileCapabilities", func(t *testing.T) {
		TestFileCapabilities(t, newFS())
	})
}
