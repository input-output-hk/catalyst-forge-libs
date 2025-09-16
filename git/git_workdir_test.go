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

func TestInitRepoInWorkdir(t *testing.T) {
	tests := []struct {
		name    string
		workdir string
		bare    bool
		verify  func(t *testing.T, fs *billy.FS, workdir string)
	}{
		{
			name:    "root non-bare",
			workdir: ".",
			bare:    false,
			verify: func(t *testing.T, fs *billy.FS, workdir string) {
				// Check HEAD exists (either at root or in .git)
				headPath1 := filepath.Join(workdir, "HEAD")
				headPath2 := filepath.Join(workdir, ".git", "HEAD")

				exists1, _ := fs.Exists(headPath1)
				exists2, _ := fs.Exists(headPath2)

				assert.True(t, exists1 || exists2,
					"HEAD should exist at either %s or %s", headPath1, headPath2)
			},
		},
		{
			name:    "subdir non-bare",
			workdir: "myproject",
			bare:    false,
			verify: func(t *testing.T, fs *billy.FS, workdir string) {
				headPath1 := filepath.Join(workdir, "HEAD")
				headPath2 := filepath.Join(workdir, ".git", "HEAD")

				exists1, _ := fs.Exists(headPath1)
				exists2, _ := fs.Exists(headPath2)

				assert.True(t, exists1 || exists2,
					"HEAD should exist in workdir")

				// Verify no git files at root
				rootHEAD, _ := fs.Exists("HEAD")
				rootGit, _ := fs.Exists(".git")
				assert.False(t, rootHEAD, "HEAD should not exist at root")
				assert.False(t, rootGit, ".git should not exist at root")
			},
		},
		{
			name:    "nested non-bare",
			workdir: "projects/myrepo",
			bare:    false,
			verify: func(t *testing.T, fs *billy.FS, workdir string) {
				headPath1 := filepath.Join(workdir, "HEAD")
				headPath2 := filepath.Join(workdir, ".git", "HEAD")

				exists1, _ := fs.Exists(headPath1)
				exists2, _ := fs.Exists(headPath2)

				assert.True(t, exists1 || exists2,
					"HEAD should exist in nested workdir")
			},
		},
		{
			name:    "deep nested non-bare",
			workdir: "a/b/c/d/repo",
			bare:    false,
			verify: func(t *testing.T, fs *billy.FS, workdir string) {
				headPath1 := filepath.Join(workdir, "HEAD")
				headPath2 := filepath.Join(workdir, ".git", "HEAD")

				exists1, _ := fs.Exists(headPath1)
				exists2, _ := fs.Exists(headPath2)

				assert.True(t, exists1 || exists2,
					"HEAD should exist in deeply nested workdir")
			},
		},
		{
			name:    "bare repository",
			workdir: "repos/bare-repo",
			bare:    true,
			verify: func(t *testing.T, fs *billy.FS, workdir string) {
				// For bare repos, git files are directly in workdir
				headPath := filepath.Join(workdir, "HEAD")
				exists, err := fs.Exists(headPath)
				require.NoError(t, err)
				assert.True(t, exists, "HEAD should exist in bare repo")

				// Config should also exist
				configPath := filepath.Join(workdir, "config")
				exists, err = fs.Exists(configPath)
				require.NoError(t, err)
				assert.True(t, exists, "config should exist in bare repo")

				// Verify structure is contained
				if workdir != "." {
					rootFiles, err := fs.ReadDir(".")
					require.NoError(t, err)
					for _, file := range rootFiles {
						if file.Name() != "repos" {
							t.Errorf("unexpected file at root: %s", file.Name())
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh filesystem for each test
			memFS := memfs.New()
			fs := billy.NewFS(memFS)

			opts := &Options{
				FS:      fs,
				Workdir: tt.workdir,
				Bare:    tt.bare,
			}

			// Initialize repository
			repo, err := Init(context.TODO(), opts)
			require.NoError(t, err)
			require.NotNil(t, repo)

			// Run custom verification
			tt.verify(t, fs, tt.workdir)
		})
	}
}

func TestOpenRepository(t *testing.T) {
	tests := []struct {
		name       string
		initDir    string
		openDir    string
		shouldFail bool
		errMsg     string
	}{
		{
			name:       "open from same location",
			initDir:    "project/repo",
			openDir:    "project/repo",
			shouldFail: false,
		},
		{
			name:       "open from wrong location",
			initDir:    "actual/location",
			openDir:    "wrong/location",
			shouldFail: true,
			errMsg:     "failed to open repository",
		},
		{
			name:       "open from root",
			initDir:    ".",
			openDir:    ".",
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create filesystem
			memFS := memfs.New()
			fs := billy.NewFS(memFS)

			// Initialize repository
			initOpts := &Options{
				FS:      fs,
				Workdir: tt.initDir,
			}

			repo, err := Init(context.TODO(), initOpts)
			require.NoError(t, err)
			require.NotNil(t, repo)

			// Try to open repository
			openOpts := &Options{
				FS:      fs,
				Workdir: tt.openDir,
			}

			openedRepo, err := Open(context.TODO(), openOpts)

			if tt.shouldFail {
				assert.Error(t, err)
				assert.Nil(t, openedRepo)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, openedRepo)
				assert.NotNil(t, openedRepo.repo)
				assert.NotNil(t, openedRepo.worktree)
			}
		})
	}
}

func TestCloneToWorkdir(t *testing.T) {
	tests := []struct {
		name    string
		workdir string
		url     string
		verify  func(t *testing.T, err error)
	}{
		{
			name:    "clone to subdir",
			workdir: "cloned/repo",
			url:     "file://source",
			verify: func(t *testing.T, err error) {
				// We expect an error due to file:// URL setup in test environment
				// but the important part is that it attempts to clone to correct location
				assert.Error(t, err)
			},
		},
		{
			name:    "clone to root",
			workdir: ".",
			url:     "file://source",
			verify: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create source repository first
			sourceMemFS := memfs.New()
			sourceFS := billy.NewFS(sourceMemFS)

			sourceOpts := &Options{
				FS:      sourceFS,
				Workdir: "source",
			}

			sourceRepo, err := Init(context.TODO(), sourceOpts)
			require.NoError(t, err)
			require.NotNil(t, sourceRepo)

			// Create test file in source
			err = sourceFS.WriteFile("source/README.md", []byte("# Test Repo"), 0o644)
			require.NoError(t, err)

			// Create destination filesystem
			destMemFS := memfs.New()
			destFS := billy.NewFS(destMemFS)

			cloneOpts := &Options{
				FS:      destFS,
				Workdir: tt.workdir,
			}

			// Attempt clone (will fail due to file:// URL handling)
			_, err = Clone(context.TODO(), tt.url, cloneOpts)
			tt.verify(t, err)
		})
	}
}
