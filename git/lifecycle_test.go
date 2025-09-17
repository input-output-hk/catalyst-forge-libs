package git

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	billyfs "github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInit tests repository initialization with various configurations
func TestInit(t *testing.T) {
	tests := []struct {
		name     string
		opts     func() Options
		validate func(t *testing.T, repo *Repo, err error)
	}{
		{
			name: "non-bare repository",
			opts: func() Options {
				return Options{
					FS:      billyfs.NewInMemoryFS(),
					Bare:    false,
					Workdir: ".",
				}
			},
			validate: func(t *testing.T, repo *Repo, err error) {
				require.NoError(t, err)
				require.NotNil(t, repo)
				assert.NotNil(t, repo.repo, "repo.repo should not be nil")
				assert.NotNil(t, repo.worktree, "worktree should not be nil for non-bare repo")
			},
		},
		{
			name: "bare repository",
			opts: func() Options {
				return Options{
					FS:   billyfs.NewInMemoryFS(),
					Bare: true,
				}
			},
			validate: func(t *testing.T, repo *Repo, err error) {
				require.NoError(t, err)
				require.NotNil(t, repo)
				assert.NotNil(t, repo.repo, "repo.repo should not be nil")
				assert.Nil(t, repo.worktree, "worktree should be nil for bare repo")
			},
		},
		{
			name: "invalid options - nil filesystem",
			opts: func() Options {
				return Options{FS: nil}
			},
			validate: func(t *testing.T, repo *Repo, err error) {
				require.Error(t, err, "should fail with nil filesystem")
				assert.Nil(t, repo)
			},
		},
		{
			name: "default options",
			opts: func() Options {
				return Options{FS: billyfs.NewInMemoryFS()}
			},
			validate: func(t *testing.T, repo *Repo, err error) {
				require.NoError(t, err)
				require.NotNil(t, repo)
				assert.Equal(t, DefaultWorkdir, repo.options.Workdir)
				assert.Equal(t, DefaultStorerCacheSize, repo.options.StorerCacheSize)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			opts := tt.opts()
			repo, err := Init(ctx, &opts)
			tt.validate(t, repo, err)
		})
	}
}

// TestInit_GitDirectoryStructure verifies the git directory structure is created correctly
func TestInit_GitDirectoryStructure(t *testing.T) {
	tests := []struct {
		name          string
		bare          bool
		expectedFiles []string
	}{
		{
			name: "non-bare repository structure",
			bare: false,
			expectedFiles: []string{
				".git/HEAD",
				".git/objects",
				".git/refs",
			},
		},
		{
			name: "bare repository structure",
			bare: true,
			expectedFiles: []string{
				"HEAD",
				"config",
				"refs",
				"objects",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			memFS := billyfs.NewInMemoryFS()

			opts := Options{
				FS:      memFS,
				Bare:    tt.bare,
				Workdir: ".",
			}

			_, err := Init(ctx, &opts)
			require.NoError(t, err)

			// Check expected files exist
			for _, file := range tt.expectedFiles {
				_, err := memFS.Stat(file)
				assert.NoError(t, err, "expected file %s to exist", file)
			}
		})
	}
}

// TestOpen tests opening existing repositories
func TestOpen(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *billyfs.FS
		opts     func(fs *billyfs.FS) Options
		validate func(t *testing.T, repo *Repo, err error)
	}{
		{
			name: "open existing non-bare repository",
			setup: func(t *testing.T) *billyfs.FS {
				memFS := billyfs.NewInMemoryFS()
				ctx := context.Background()
				_, err := Init(ctx, &Options{
					FS:      memFS,
					Bare:    false,
					Workdir: ".",
				})
				require.NoError(t, err)
				return memFS
			},
			opts: func(fs *billyfs.FS) Options {
				return Options{
					FS:      fs,
					Bare:    false,
					Workdir: ".",
				}
			},
			validate: func(t *testing.T, repo *Repo, err error) {
				require.NoError(t, err)
				require.NotNil(t, repo)
				assert.NotNil(t, repo.repo)
				assert.NotNil(t, repo.worktree)
			},
		},
		{
			name: "open existing bare repository",
			setup: func(t *testing.T) *billyfs.FS {
				memFS := billyfs.NewInMemoryFS()
				ctx := context.Background()
				_, err := Init(ctx, &Options{
					FS:   memFS,
					Bare: true,
				})
				require.NoError(t, err)
				return memFS
			},
			opts: func(fs *billyfs.FS) Options {
				return Options{
					FS:   fs,
					Bare: true,
				}
			},
			validate: func(t *testing.T, repo *Repo, err error) {
				require.NoError(t, err)
				require.NotNil(t, repo)
				assert.NotNil(t, repo.repo)
				assert.Nil(t, repo.worktree)
			},
		},
		{
			name: "open non-existent repository",
			setup: func(t *testing.T) *billyfs.FS {
				return billyfs.NewInMemoryFS()
			},
			opts: func(fs *billyfs.FS) Options {
				return Options{
					FS:      fs,
					Bare:    false,
					Workdir: ".",
				}
			},
			validate: func(t *testing.T, repo *Repo, err error) {
				require.Error(t, err, "should fail for non-existent repository")
				assert.Nil(t, repo)
			},
		},
		{
			name: "invalid options - nil filesystem",
			setup: func(t *testing.T) *billyfs.FS {
				return nil
			},
			opts: func(fs *billyfs.FS) Options {
				return Options{FS: nil}
			},
			validate: func(t *testing.T, repo *Repo, err error) {
				require.Error(t, err, "should fail with nil filesystem")
				assert.Nil(t, repo)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			fs := tt.setup(t)
			opts := tt.opts(fs)
			repo, err := Open(ctx, &opts)
			tt.validate(t, repo, err)
		})
	}
}

// TestClone tests repository cloning
func TestClone(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		opts     func() Options
		validate func(t *testing.T, repo *Repo, err error)
	}{
		{
			name: "empty URL",
			url:  "",
			opts: func() Options {
				return Options{FS: billyfs.NewInMemoryFS()}
			},
			validate: func(t *testing.T, repo *Repo, err error) {
				require.Error(t, err, "should fail with empty URL")
				assert.Nil(t, repo)
			},
		},
		{
			name: "invalid options - nil filesystem",
			url:  "https://github.com/user/repo.git",
			opts: func() Options {
				return Options{FS: nil}
			},
			validate: func(t *testing.T, repo *Repo, err error) {
				require.Error(t, err, "should fail with nil filesystem")
				assert.Nil(t, repo)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			opts := tt.opts()
			repo, err := Clone(ctx, tt.url, &opts)
			tt.validate(t, repo, err)
		})
	}
}

// mockAuthProvider is a test implementation of AuthProvider for testing auth flow
type mockAuthProvider struct {
	auth   transport.AuthMethod
	called bool
}

//nolint:ireturn // transport.AuthMethod is an interface required by go-git
func (m *mockAuthProvider) Method(remoteURL string) (transport.AuthMethod, error) {
	m.called = true
	return m.auth, nil
}

// TestClone_WithAuthProvider tests that auth providers are called during clone
func TestClone_WithAuthProvider(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	// Create a mock auth provider that returns valid auth
	mockAuth := &mockAuthProvider{
		auth: &http.BasicAuth{
			Username: "test",
			Password: "password",
		},
	}

	opts := Options{
		FS:   memFS,
		Auth: mockAuth,
	}

	// This will fail because the URL doesn't exist, but we want to test
	// that auth provider is called properly
	_, err := Clone(ctx, "https://github.com/user/nonexistent-repo.git", &opts)

	require.Error(t, err, "should fail for non-existent repository")
	assert.True(t, mockAuth.called, "auth provider should have been called")
}
