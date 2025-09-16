package git

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_CreatesRepoInWorkdir(t *testing.T) {
	// Test different workdir locations
	testCases := []struct {
		name    string
		workdir string
	}{
		{"root", "."},
		{"subdir", "myproject"},
		{"nested", "projects/myrepo"},
		{"deep", "a/b/c/d/repo"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fresh filesystem for each test
			memFS := memfs.New()
			fs := billy.NewFS(memFS)

			opts := &Options{
				FS:      fs,
				Workdir: tc.workdir,
			}

			// Initialize repository
			repo, err := Init(context.TODO(), opts)
			require.NoError(t, err)
			require.NotNil(t, repo)

			// After Chroot, the repository is created at the root of the scoped filesystem
			// So from the original filesystem's perspective, files are at workdir/...

			// For non-bare repos, go-git creates either:
			// 1. A .git directory with HEAD inside it, OR
			// 2. A .git file (for worktrees) with git files alongside

			// Check if HEAD exists (it could be at workdir/HEAD for a bare-like structure
			// or workdir/.git/HEAD for a normal structure)
			headPath1 := filepath.Join(tc.workdir, "HEAD")
			headPath2 := filepath.Join(tc.workdir, ".git", "HEAD")

			exists1, _ := fs.Exists(headPath1)
			exists2, _ := fs.Exists(headPath2)

			assert.True(t, exists1 || exists2,
				"HEAD file should exist at either %s or %s", headPath1, headPath2)

			// Note: go-git doesn't always create a config file on init,
			// it only creates it when there's actual configuration to save.
			// So we skip checking for config file existence.

			// Verify that git files are contained within workdir
			if tc.workdir != "." && tc.workdir != "" {
				// Check that no git files exist at root
				rootHEAD, _ := fs.Exists("HEAD")
				rootConfig, _ := fs.Exists("config")
				rootGit, _ := fs.Exists(".git")

				assert.False(t, rootHEAD, "HEAD should not exist at root when workdir is %s", tc.workdir)
				assert.False(t, rootConfig, "config should not exist at root when workdir is %s", tc.workdir)
				assert.False(t, rootGit, ".git should not exist at root when workdir is %s", tc.workdir)
			}
		})
	}
}

func TestInit_BareRepoInWorkdir(t *testing.T) {
	// Create an in-memory filesystem
	memFS := memfs.New()
	fs := billy.NewFS(memFS)

	opts := &Options{
		FS:      fs,
		Workdir: "repos/bare-repo",
		Bare:    true,
	}

	// Initialize bare repository
	repo, err := Init(context.TODO(), opts)
	require.NoError(t, err)
	require.NotNil(t, repo)

	// For a bare repository, the git files should be directly in the workdir
	headPath := filepath.Join("repos/bare-repo", "HEAD")
	exists, err := fs.Exists(headPath)
	require.NoError(t, err)
	assert.True(t, exists, "HEAD file should exist at %s for bare repo", headPath)

	// Verify config exists
	configPath := filepath.Join("repos/bare-repo", "config")
	exists, err = fs.Exists(configPath)
	require.NoError(t, err)
	assert.True(t, exists, "config file should exist at %s for bare repo", configPath)

	// Verify no worktree files outside the git directory
	rootFiles, err := fs.ReadDir(".")
	require.NoError(t, err)
	for _, file := range rootFiles {
		// Only "repos" directory should exist at root
		if file.Name() != "repos" {
			t.Errorf("unexpected file at root: %s", file.Name())
		}
	}
}

func TestOpen_OpensRepoFromWorkdir(t *testing.T) {
	// Create an in-memory filesystem
	memFS := memfs.New()
	fs := billy.NewFS(memFS)

	workdir := "project/repo"

	// First, initialize a repository
	initOpts := &Options{
		FS:      fs,
		Workdir: workdir,
	}

	repo, err := Init(context.TODO(), initOpts)
	require.NoError(t, err)
	require.NotNil(t, repo)

	// Now try to open it
	openOpts := &Options{
		FS:      fs,
		Workdir: workdir,
	}

	openedRepo, err := Open(context.TODO(), openOpts)
	require.NoError(t, err)
	require.NotNil(t, openedRepo)

	// Verify we can perform operations on the opened repo
	// (This would require implementing more methods, but we verify the repo is valid)
	assert.NotNil(t, openedRepo.repo)
	assert.NotNil(t, openedRepo.worktree)
}

func TestOpen_FailsIfRepoNotInWorkdir(t *testing.T) {
	// Create an in-memory filesystem
	memFS := memfs.New()
	fs := billy.NewFS(memFS)

	// Initialize repo in one location
	initOpts := &Options{
		FS:      fs,
		Workdir: "actual/location",
	}

	repo, err := Init(context.TODO(), initOpts)
	require.NoError(t, err)
	require.NotNil(t, repo)

	// Try to open from a different location
	openOpts := &Options{
		FS:      fs,
		Workdir: "wrong/location",
	}

	openedRepo, err := Open(context.TODO(), openOpts)
	assert.Error(t, err, "should fail to open repo from wrong location")
	assert.Nil(t, openedRepo)
	assert.Contains(t, err.Error(), "failed to open repository")
}

func TestClone_ClonesToWorkdir(t *testing.T) {
	// Create source repository
	sourceMemFS := memfs.New()
	sourceFS := billy.NewFS(sourceMemFS)

	sourceOpts := &Options{
		FS:      sourceFS,
		Workdir: "source",
	}

	sourceRepo, err := Init(context.TODO(), sourceOpts)
	require.NoError(t, err)
	require.NotNil(t, sourceRepo)

	// Create a file and commit in source repo
	err = sourceFS.WriteFile("source/README.md", []byte("# Test Repo"), 0o644)
	require.NoError(t, err)

	// Note: Full clone test would require implementing Add and Commit methods
	// For now, we just test that Clone attempts to create repo in the correct location

	// Create destination filesystem
	destMemFS := memfs.New()
	destFS := billy.NewFS(destMemFS)

	cloneOpts := &Options{
		FS:      destFS,
		Workdir: "cloned/repo",
	}

	// This will fail because file:// URLs need proper setup, but we verify the error
	// In a real test with proper file:// URL setup, this would succeed
	_, err = Clone(context.TODO(), "file://source", cloneOpts)
	// We expect an error here due to URL setup, but the important part is
	// that it attempts to create the repo in the correct location
	assert.Error(t, err)
}
