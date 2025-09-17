package git

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	billyfs "github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/git/internal/fsbridge"
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

// TestCurrentBranch tests getting the current branch
func TestCurrentBranch(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *testRepo
		validate func(t *testing.T, branch string, err error)
	}{
		{
			name:  "default branch after commit",
			setup: setupTestRepoWithCommit,
			validate: func(t *testing.T, branch string, err error) {
				require.NoError(t, err)
				assert.Equal(t, "master", branch)
			},
		},
		{
			name: "detached HEAD state",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Detach HEAD
				head, err := tr.repo.repo.Head()
				require.NoError(t, err)

				err = tr.repo.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, head.Hash()))
				require.NoError(t, err)

				return tr
			},
			validate: func(t *testing.T, branch string, err error) {
				require.Error(t, err, "should fail in detached HEAD state")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			branch, err := tr.repo.CurrentBranch(tr.ctx)
			tt.validate(t, branch, err)
		})
	}
}

// TestCreateBranch tests branch creation
func TestCreateBranch(t *testing.T) {
	tests := []struct {
		name        string
		branchName  string
		startRev    string
		trackRemote bool
		force       bool
		setup       func(t *testing.T) *testRepo
		wantErr     error
	}{
		{
			name:       "create branch from master",
			branchName: "feature-branch",
			startRev:   "master",
			setup:      setupTestRepoWithCommit,
			wantErr:    nil,
		},
		{
			name:       "create branch - already exists without force",
			branchName: "existing-branch",
			startRev:   "master",
			force:      false,
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				err := tr.repo.CreateBranch(tr.ctx, "existing-branch", "master", false, false)
				require.NoError(t, err)
				return tr
			},
			wantErr: ErrBranchExists,
		},
		{
			name:       "create branch - already exists with force",
			branchName: "existing-branch",
			startRev:   "master",
			force:      true,
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				err := tr.repo.CreateBranch(tr.ctx, "existing-branch", "master", false, false)
				require.NoError(t, err)
				return tr
			},
			wantErr: nil,
		},
		{
			name:       "invalid revision",
			branchName: "test-branch",
			startRev:   "non-existent",
			setup:      setupTestRepoWithCommit,
			wantErr:    ErrResolveFailed,
		},
		{
			name:       "empty branch name",
			branchName: "",
			startRev:   "master",
			setup:      setupTestRepoWithCommit,
			wantErr:    ErrInvalidRef,
		},
		{
			name:       "empty revision",
			branchName: "test-branch",
			startRev:   "",
			setup:      setupTestRepoWithCommit,
			wantErr:    ErrInvalidRef,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.CreateBranch(tr.ctx, tt.branchName, tt.startRev, tt.trackRemote, tt.force)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
			} else {
				require.NoError(t, err)
				// Verify branch was created
				if tt.branchName != "" {
					tr.verifyBranchExists(t, tt.branchName)
				}
			}
		})
	}
}

// TestCheckoutBranch tests branch checkout operations
func TestCheckoutBranch(t *testing.T) {
	tests := []struct {
		name            string
		branchName      string
		createIfMissing bool
		force           bool
		setup           func(t *testing.T) *testRepo
		wantErr         error
		wantBranch      string
	}{
		{
			name:       "checkout existing branch",
			branchName: "master",
			setup:      setupTestRepoWithCommit,
			wantBranch: "master",
		},
		{
			name:            "checkout non-existent branch with createIfMissing",
			branchName:      "new-branch",
			createIfMissing: true,
			force:           true,
			setup:           setupTestRepoWithCommit,
			wantBranch:      "new-branch",
		},
		{
			name:            "checkout non-existent branch without createIfMissing",
			branchName:      "non-existent",
			createIfMissing: false,
			setup:           setupTestRepoWithCommit,
			wantErr:         ErrBranchMissing,
		},
		{
			name:       "empty branch name",
			branchName: "",
			setup:      setupTestRepoWithCommit,
			wantErr:    ErrInvalidRef,
		},
		{
			name:       "force checkout with uncommitted changes",
			branchName: "feature-branch",
			force:      true,
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create a branch
				err := tr.repo.CreateBranch(tr.ctx, "feature-branch", "HEAD", false, false)
				require.NoError(t, err)

				// Make uncommitted changes
				tr.modifyTestFile(t, "modified content")
				_, err = tr.repo.worktree.Add("test.txt")
				require.NoError(t, err)

				return tr
			},
			wantBranch: "feature-branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.CheckoutBranch(tr.ctx, tt.branchName, tt.createIfMissing, tt.force)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
			} else {
				require.NoError(t, err)
				if tt.wantBranch != "" {
					currentBranch := tr.getCurrentBranch(t)
					assert.Equal(t, tt.wantBranch, currentBranch)
				}
			}
		})
	}
}

// TestDeleteBranch tests branch deletion
func TestDeleteBranch(t *testing.T) {
	tests := []struct {
		name       string
		branchName string
		setup      func(t *testing.T) *testRepo
		wantErr    error
	}{
		{
			name:       "delete existing branch",
			branchName: "branch-to-delete",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				tr.createTestBranch(t, "branch-to-delete")
				return tr
			},
			wantErr: nil,
		},
		{
			name:       "delete current branch",
			branchName: "master",
			setup:      setupTestRepoWithCommit,
			wantErr:    ErrBranchExists,
		},
		{
			name:       "delete non-existent branch",
			branchName: "non-existent",
			setup:      setupTestRepoWithCommit,
			wantErr:    ErrBranchMissing,
		},
		{
			name:       "empty branch name",
			branchName: "",
			setup:      setupTestRepoWithCommit,
			wantErr:    ErrInvalidRef,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.DeleteBranch(tr.ctx, tt.branchName)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
			} else {
				require.NoError(t, err)
				// Verify branch is deleted
				tr.verifyBranchNotExists(t, tt.branchName)
			}
		})
	}
}

// TestCheckoutRemoteBranch tests checking out remote branches
func TestCheckoutRemoteBranch(t *testing.T) {
	tests := []struct {
		name         string
		remote       string
		remoteBranch string
		localName    string
		track        bool
		setup        func(t *testing.T) *testRepo
		wantErr      error
		wantBranch   string
	}{
		{
			name:         "checkout existing remote branch",
			remote:       "origin",
			remoteBranch: "main",
			localName:    "local-main",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				tr.createRemoteBranch(t, "origin", "main")
				return tr
			},
			wantBranch: "local-main",
		},
		{
			name:         "missing remote branch",
			remote:       "origin",
			remoteBranch: "non-existent",
			localName:    "local-branch",
			setup:        setupTestRepoWithCommit,
			wantErr:      ErrResolveFailed,
		},
		{
			name:         "empty remote name",
			remote:       "",
			remoteBranch: "main",
			localName:    "local-main",
			setup:        setupTestRepoWithCommit,
			wantErr:      ErrInvalidRef,
		},
		{
			name:         "empty remote branch name",
			remote:       "origin",
			remoteBranch: "",
			localName:    "local-main",
			setup:        setupTestRepoWithCommit,
			wantErr:      ErrInvalidRef,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.CheckoutRemoteBranch(tr.ctx, tt.remote, tt.remoteBranch, tt.localName, tt.track)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
			} else {
				require.NoError(t, err)
				if tt.wantBranch != "" {
					currentBranch := tr.getCurrentBranch(t)
					assert.Equal(t, tt.wantBranch, currentBranch)
					tr.verifyBranchExists(t, tt.localName)
				}
			}
		})
	}
}

// TestGoGitDirect verifies that go-git works directly with in-memory filesystem
func TestGoGitDirect(t *testing.T) {
	memFS := billyfs.NewInMemoryFS()
	rawFS := memFS.Raw()

	storage := fsbridge.NewStorage(rawFS, 1000)
	repo, err := git.Init(storage, rawFS)

	require.NoError(t, err, "Direct go-git Init should succeed")
	require.NotNil(t, repo, "Direct go-git Init should return a repository")
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

// TestFetch tests the Fetch method with various scenarios
func TestFetch(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *testRepo
		remote   string
		prune    bool
		depth    int
		validate func(t *testing.T, err error)
	}{
		{
			name: "fetch from non-existent remote",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "nonexistent",
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
		{
			name: "fetch with empty remote (uses default)",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "",
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(
					t,
					errors.Is(err, ErrResolveFailed),
					"should return resolve failed error for default remote",
				)
			},
		},
		{
			name: "fetch with prune and depth",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "",
			prune:  true,
			depth:  1,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.Fetch(tr.ctx, tt.remote, tt.prune, tt.depth)
			tt.validate(t, err)
		})
	}
}

// TestPullFFOnly tests the PullFFOnly method
func TestPullFFOnly(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *testRepo
		remote   string
		validate func(t *testing.T, err error)
	}{
		{
			name: "pull from bare repository",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, true)
			},
			remote: "",
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidRef), "should return invalid ref error for bare repo")
			},
		},
		{
			name: "pull from non-existent remote",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "nonexistent",
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
		{
			name: "pull with empty remote (uses default)",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "",
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.PullFFOnly(tr.ctx, tt.remote)
			tt.validate(t, err)
		})
	}
}

// TestFetchAndMerge tests the FetchAndMerge method
func TestFetchAndMerge(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *testRepo
		remote   string
		fromRef  string
		strategy MergeStrategy
		validate func(t *testing.T, err error)
	}{
		{
			name: "merge with invalid remote",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote:   "nonexistent",
			fromRef:  "HEAD",
			strategy: FastForwardOnly,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
		{
			name: "merge with invalid ref",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepo(t, false)
				// Add a remote so fetch succeeds but merge fails
				_, err := tr.repo.repo.CreateRemote(&config.RemoteConfig{
					Name: DefaultRemoteName,
					URLs: []string{"file://" + t.TempDir()},
				})
				require.NoError(t, err)
				return tr
			},
			remote:   "",
			fromRef:  "invalid-ref",
			strategy: FastForwardOnly,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				// The fetch may fail due to the temp dir not being a valid git repo
				assert.Error(t, err, "should return an error")
			},
		},
		{
			name: "merge with unsupported strategy",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepo(t, false)
				// Add a remote so fetch succeeds and we get to strategy validation
				_, err := tr.repo.repo.CreateRemote(&config.RemoteConfig{
					Name: DefaultRemoteName,
					URLs: []string{"file://" + t.TempDir()},
				})
				require.NoError(t, err)
				return tr
			},
			remote:   "",
			fromRef:  "HEAD",
			strategy: MergeStrategy(99), // Invalid strategy
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				// The fetch may fail, or we may get to strategy validation
				// Either way, we expect some error
				assert.Error(t, err, "should return some error")
			},
		},
		{
			name: "merge with valid parameters but no remote",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote:   "",
			fromRef:  "HEAD",
			strategy: FastForwardOnly,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.FetchAndMerge(tr.ctx, tt.remote, tt.fromRef, tt.strategy)
			tt.validate(t, err)
		})
	}
}

// TestPush tests the Push method
func TestPush(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) *testRepo
		remote   string
		force    bool
		validate func(t *testing.T, err error)
	}{
		{
			name: "push to non-existent remote",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "nonexistent",
			force:  false,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
		{
			name: "push with empty remote (uses default)",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "",
			force:  false,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
		{
			name: "push with force flag",
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			remote: "",
			force:  true,
			validate: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed), "should return resolve failed error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			err := tr.repo.Push(tr.ctx, tt.remote, tt.force)
			tt.validate(t, err)
		})
	}
}

// TestMergeStrategy_String tests the String method of MergeStrategy
func TestMergeStrategy_String(t *testing.T) {
	tests := []struct {
		strategy MergeStrategy
		expected string
	}{
		{FastForwardOnly, "fast-forward-only"},
		{MergeStrategy(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.strategy.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRemove tests the Remove method for unstaging and removing files
func TestRemove(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, tr *testRepo)
		paths       []string
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, tr *testRepo)
	}{
		{
			name: "remove single staged file",
			setup: func(t *testing.T, tr *testRepo) {
				// Create and stage a file
				err := tr.fs.WriteFile("test.txt", []byte("test content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			paths:       []string{"test.txt"},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Execute Remove operation
				err := tr.repo.Remove(context.Background(), "test.txt")
				require.NoError(t, err)

				// Verify file is removed from filesystem
				exists, err := tr.fs.Exists("test.txt")
				require.NoError(t, err)
				assert.False(t, exists, "file should be removed from filesystem")
			},
		},
		{
			name: "remove multiple files",
			setup: func(t *testing.T, tr *testRepo) {
				// Create and stage multiple files
				err := tr.fs.WriteFile("file1.txt", []byte("content 1"), 0o644)
				require.NoError(t, err)
				err = tr.fs.WriteFile("file2.txt", []byte("content 2"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "file1.txt", "file2.txt")
				require.NoError(t, err)
			},
			paths:       []string{"file1.txt", "file2.txt"},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify both files are removed from filesystem
				exists1, err := tr.fs.Exists("file1.txt")
				require.NoError(t, err)
				assert.False(t, exists1, "file1.txt should be removed from filesystem")

				exists2, err := tr.fs.Exists("file2.txt")
				require.NoError(t, err)
				assert.False(t, exists2, "file2.txt should be removed from filesystem")
			},
		},
		{
			name: "remove with glob pattern",
			setup: func(t *testing.T, tr *testRepo) {
				// Create files with similar names
				err := tr.fs.WriteFile("test1.txt", []byte("content 1"), 0o644)
				require.NoError(t, err)
				err = tr.fs.WriteFile("test2.txt", []byte("content 2"), 0o644)
				require.NoError(t, err)
				err = tr.fs.WriteFile("other.txt", []byte("other content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test1.txt", "test2.txt", "other.txt")
				require.NoError(t, err)
			},
			paths:       []string{"test*.txt"},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify matching files are removed from filesystem, non-matching remains
				exists1, err := tr.fs.Exists("test1.txt")
				require.NoError(t, err)
				assert.False(t, exists1, "test1.txt should be removed from filesystem")

				exists2, err := tr.fs.Exists("test2.txt")
				require.NoError(t, err)
				assert.False(t, exists2, "test2.txt should be removed from filesystem")

				existsOther, err := tr.fs.Exists("other.txt")
				require.NoError(t, err)
				assert.True(t, existsOther, "other.txt should remain in filesystem")
			},
		},
		{
			name: "remove already deleted file",
			setup: func(t *testing.T, tr *testRepo) {
				// Create, stage, then delete a file
				err := tr.fs.WriteFile("test.txt", []byte("test content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
				err = tr.fs.Remove("test.txt")
				require.NoError(t, err)
			},
			paths:       []string{"test.txt"},
			expectError: false, // Should handle already deleted files gracefully
			validate: func(t *testing.T, tr *testRepo) {
				// File should remain deleted from filesystem
				exists, err := tr.fs.Exists("test.txt")
				require.NoError(t, err)
				assert.False(t, exists, "file should remain deleted from filesystem")
			},
		},
		{
			name: "remove non-existent file",
			setup: func(t *testing.T, tr *testRepo) {
				// No setup needed
			},
			paths:       []string{"nonexistent.txt"},
			expectError: false, // Should silently ignore non-existent files
			validate: func(t *testing.T, tr *testRepo) {
				// File should not be in status
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				fileStatus := status.File("nonexistent.txt")
				assert.Equal(t, git.Untracked, fileStatus.Worktree)
				assert.Equal(t, git.Untracked, fileStatus.Staging)
			},
		},
		{
			name: "remove empty paths",
			setup: func(t *testing.T, tr *testRepo) {
				// Create and stage a file
				err := tr.fs.WriteFile("test.txt", []byte("test content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			paths:       []string{"", "test.txt", ""},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify file is removed from filesystem despite empty paths
				exists, err := tr.fs.Exists("test.txt")
				require.NoError(t, err)
				assert.False(t, exists, "file should be removed from filesystem")
			},
		},
		{
			name:        "remove no paths",
			setup:       func(t *testing.T, tr *testRepo) {},
			paths:       []string{},
			expectError: false, // No paths is not an error
			validate:    func(t *testing.T, tr *testRepo) {},
		},
		{
			name: "remove in bare repository",
			setup: func(t *testing.T, tr *testRepo) {
				// This test uses a bare repository setup
			},
			paths:       []string{"test.txt"},
			expectError: true,
			errorMsg:    "cannot remove files in bare repository",
			validate:    func(t *testing.T, tr *testRepo) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Setup test repository
			var tr *testRepo
			if tt.name == "remove in bare repository" {
				tr = setupTestRepo(t, true) // Bare repository
			} else {
				tr = setupTestRepo(t, false) // Non-bare repository
			}

			// Run setup
			tt.setup(t, tr)

			// Execute Remove operation
			err := tr.repo.Remove(ctx, tt.paths...)

			// Verify error expectation
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}

			// Run validation
			if !tt.expectError {
				tt.validate(t, tr)
			}
		})
	}
}

// TestUnstage tests the Unstage method for unstaging files from the index
func TestUnstage(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, tr *testRepo)
		paths       []string
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, tr *testRepo)
	}{
		{
			name: "unstage single staged file",
			setup: func(t *testing.T, tr *testRepo) {
				// Use setupTestRepoWithCommit which creates a repo with initial commit
				*tr = *setupTestRepoWithCommit(t)

				// Modify the file and stage it
				err := tr.fs.WriteFile("test.txt", []byte("modified content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			paths:       []string{"test.txt"},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify file is unstaged but still exists in worktree
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				fileStatus := status.File("test.txt")
				// After unstaging, index should match HEAD (Unmodified) but worktree has changes (Modified)
				assert.Equal(t, git.Unmodified, fileStatus.Staging)
				assert.Equal(t, git.Modified, fileStatus.Worktree)

				// File should still exist in filesystem
				exists, err := tr.fs.Exists("test.txt")
				require.NoError(t, err)
				assert.True(t, exists, "file should still exist in worktree")
			},
		},
		{
			name: "unstage multiple files",
			setup: func(t *testing.T, tr *testRepo) {
				// Use setupTestRepoWithCommit which creates a repo with initial commit
				*tr = *setupTestRepoWithCommit(t)

				// Create and stage additional files
				err := tr.fs.WriteFile("file1.txt", []byte("content 1"), 0o644)
				require.NoError(t, err)
				err = tr.fs.WriteFile("file2.txt", []byte("content 2"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "file1.txt", "file2.txt")
				require.NoError(t, err)
			},
			paths:       []string{"file1.txt", "file2.txt"},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify both files are unstaged but still exist
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				file1Status := status.File("file1.txt")
				file2Status := status.File("file2.txt")
				assert.Equal(t, git.Untracked, file1Status.Staging)
				assert.Equal(t, git.Untracked, file2Status.Staging)
				assert.Equal(t, git.Untracked, file1Status.Worktree)
				assert.Equal(t, git.Untracked, file2Status.Worktree)

				// Files should still exist in filesystem
				exists1, err := tr.fs.Exists("file1.txt")
				require.NoError(t, err)
				assert.True(t, exists1, "file1.txt should still exist in worktree")

				exists2, err := tr.fs.Exists("file2.txt")
				require.NoError(t, err)
				assert.True(t, exists2, "file2.txt should still exist in worktree")
			},
		},
		{
			name: "unstage with glob pattern",
			setup: func(t *testing.T, tr *testRepo) {
				// Use setupTestRepoWithCommit which creates a repo with initial commit
				*tr = *setupTestRepoWithCommit(t)

				// Create and stage additional files
				err := tr.fs.WriteFile("test1.txt", []byte("content 1"), 0o644)
				require.NoError(t, err)
				err = tr.fs.WriteFile("test2.txt", []byte("content 2"), 0o644)
				require.NoError(t, err)
				err = tr.fs.WriteFile("other.txt", []byte("other content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test1.txt", "test2.txt", "other.txt")
				require.NoError(t, err)
			},
			paths:       []string{"test*.txt"},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify matching files are unstaged, non-matching remains staged
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				test1Status := status.File("test1.txt")
				test2Status := status.File("test2.txt")
				otherStatus := status.File("other.txt")
				assert.Equal(t, git.Untracked, test1Status.Staging)
				assert.Equal(t, git.Untracked, test2Status.Staging)
				assert.Equal(t, git.Added, otherStatus.Staging)

				// All files should still exist in filesystem
				exists1, err := tr.fs.Exists("test1.txt")
				require.NoError(t, err)
				assert.True(t, exists1, "test1.txt should still exist")

				exists2, err := tr.fs.Exists("test2.txt")
				require.NoError(t, err)
				assert.True(t, exists2, "test2.txt should still exist")

				existsOther, err := tr.fs.Exists("other.txt")
				require.NoError(t, err)
				assert.True(t, existsOther, "other.txt should still exist")
			},
		},
		{
			name: "unstage already unstaged file",
			setup: func(t *testing.T, tr *testRepo) {
				// Create a file but don't stage it
				err := tr.fs.WriteFile("test.txt", []byte("test content"), 0o644)
				require.NoError(t, err)
				// Don't call Add - file should remain untracked
			},
			paths:       []string{"test.txt"},
			expectError: false, // Should silently ignore already unstaged files
			validate: func(t *testing.T, tr *testRepo) {
				// File should remain untracked
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				fileStatus := status.File("test.txt")
				assert.Equal(t, git.Untracked, fileStatus.Staging)
				assert.Equal(t, git.Untracked, fileStatus.Worktree)

				// File should still exist in filesystem
				exists, err := tr.fs.Exists("test.txt")
				require.NoError(t, err)
				assert.True(t, exists, "file should still exist in worktree")
			},
		},
		{
			name: "unstage non-existent file",
			setup: func(t *testing.T, tr *testRepo) {
				// No setup needed
			},
			paths:       []string{"nonexistent.txt"},
			expectError: false, // Should silently ignore non-existent files
			validate: func(t *testing.T, tr *testRepo) {
				// File should not be in status
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				fileStatus := status.File("nonexistent.txt")
				assert.Equal(t, git.Untracked, fileStatus.Worktree)
				assert.Equal(t, git.Untracked, fileStatus.Staging)
			},
		},
		{
			name: "unstage empty paths",
			setup: func(t *testing.T, tr *testRepo) {
				// Use setupTestRepoWithCommit which creates a repo with initial commit
				*tr = *setupTestRepoWithCommit(t)

				// Modify the file and stage it
				err := tr.fs.WriteFile("test.txt", []byte("modified content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			paths:       []string{"", "test.txt", ""},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify file is unstaged despite empty paths
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				fileStatus := status.File("test.txt")
				// After unstaging, index should match HEAD (Unmodified) but worktree has changes (Modified)
				assert.Equal(t, git.Unmodified, fileStatus.Staging)
				assert.Equal(t, git.Modified, fileStatus.Worktree)

				// File should still exist in filesystem
				exists, err := tr.fs.Exists("test.txt")
				require.NoError(t, err)
				assert.True(t, exists, "file should still exist in worktree")
			},
		},
		{
			name:        "unstage no paths",
			setup:       func(t *testing.T, tr *testRepo) {},
			paths:       []string{},
			expectError: false, // No paths is not an error
			validate:    func(t *testing.T, tr *testRepo) {},
		},
		{
			name: "unstage in bare repository",
			setup: func(t *testing.T, tr *testRepo) {
				// This test uses a bare repository setup
			},
			paths:       []string{"test.txt"},
			expectError: true,
			errorMsg:    "cannot unstage files in bare repository",
			validate:    func(t *testing.T, tr *testRepo) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Setup test repository
			var tr *testRepo
			if tt.name == "unstage in bare repository" {
				tr = setupTestRepo(t, true) // Bare repository
			} else {
				tr = setupTestRepo(t, false) // Non-bare repository
			}

			// Run setup (some tests may replace tr entirely)
			tt.setup(t, tr)

			// Execute Unstage operation
			err := tr.repo.Unstage(ctx, tt.paths...)

			// Verify error expectation
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}

			// Run validation
			if !tt.expectError {
				tt.validate(t, tr)
			}
		})
	}
}

// TestCommit tests the Commit method for creating commits
func TestCommit(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, tr *testRepo)
		message     string
		signature   Signature
		opts        CommitOpts
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, tr *testRepo, commitSHA string)
	}{
		{
			name: "commit staged changes",
			setup: func(t *testing.T, tr *testRepo) {
				// Use setupTestRepoWithCommit which creates a repo with initial commit
				*tr = *setupTestRepoWithCommit(t)

				// Modify the file and stage it
				err := tr.fs.WriteFile("test.txt", []byte("modified content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			message: "Test commit with staged changes",
			signature: Signature{
				Name:  "Test Author",
				Email: "test@example.com",
				When:  time.Now(),
			},
			opts:        CommitOpts{},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo, commitSHA string) {
				// Verify commit was created
				assert.NotEmpty(t, commitSHA)
				assert.Len(t, commitSHA, 40) // SHA-1 hash length

				// Verify the file content is committed
				content, err := tr.fs.ReadFile("test.txt")
				require.NoError(t, err)
				assert.Equal(t, "modified content", string(content))

				// Verify status after commit - the exact status may vary by go-git implementation
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				fileStatus := status.File("test.txt")
				// File should not be marked as having uncommitted changes
				assert.NotEqual(t, git.Modified, fileStatus.Worktree)
			},
		},
		{
			name: "commit empty (no changes)",
			setup: func(t *testing.T, tr *testRepo) {
				// Use setupTestRepoWithCommit which creates a repo with initial commit
				*tr = *setupTestRepoWithCommit(t)
				// Don't make any changes
			},
			message: "Empty commit",
			signature: Signature{
				Name:  "Test Author",
				Email: "test@example.com",
				When:  time.Now(),
			},
			opts:        CommitOpts{AllowEmpty: true},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo, commitSHA string) {
				// Verify commit was created even though empty
				assert.NotEmpty(t, commitSHA)
				assert.Len(t, commitSHA, 40)
			},
		},
		{
			name: "commit empty without allow empty",
			setup: func(t *testing.T, tr *testRepo) {
				// Use setupTestRepoWithCommit which creates a repo with initial commit
				*tr = *setupTestRepoWithCommit(t)
				// Don't make any changes
			},
			message: "Should fail empty commit",
			signature: Signature{
				Name:  "Test Author",
				Email: "test@example.com",
				When:  time.Now(),
			},
			opts:        CommitOpts{AllowEmpty: false},
			expectError: true,
			errorMsg:    "no changes staged for commit",
			validate:    func(t *testing.T, tr *testRepo, commitSHA string) {},
		},
		{
			name: "commit with empty message",
			setup: func(t *testing.T, tr *testRepo) {
				// Use setupTestRepoWithCommit which creates a repo with initial commit
				*tr = *setupTestRepoWithCommit(t)

				// Stage a change
				err := tr.fs.WriteFile("test.txt", []byte("modified content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			message: "",
			signature: Signature{
				Name:  "Test Author",
				Email: "test@example.com",
				When:  time.Now(),
			},
			opts:        CommitOpts{},
			expectError: true,
			errorMsg:    "commit message cannot be empty",
			validate:    func(t *testing.T, tr *testRepo, commitSHA string) {},
		},
		{
			name: "commit with invalid signature",
			setup: func(t *testing.T, tr *testRepo) {
				// Use setupTestRepoWithCommit which creates a repo with initial commit
				*tr = *setupTestRepoWithCommit(t)

				// Stage a change
				err := tr.fs.WriteFile("test.txt", []byte("modified content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			message: "Test commit",
			signature: Signature{
				Name:  "", // Empty name
				Email: "test@example.com",
				When:  time.Now(),
			},
			opts:        CommitOpts{},
			expectError: true,
			errorMsg:    "committer name and email are required",
			validate:    func(t *testing.T, tr *testRepo, commitSHA string) {},
		},
		{
			name: "commit in bare repository",
			setup: func(t *testing.T, tr *testRepo) {
				// This test uses a bare repository setup
			},
			message: "Should fail in bare repo",
			signature: Signature{
				Name:  "Test Author",
				Email: "test@example.com",
				When:  time.Now(),
			},
			opts:        CommitOpts{},
			expectError: true,
			errorMsg:    "cannot commit in bare repository",
			validate:    func(t *testing.T, tr *testRepo, commitSHA string) {},
		},
		{
			name: "commit with amend option",
			setup: func(t *testing.T, tr *testRepo) {
				// Use setupTestRepoWithCommit which creates a repo with initial commit
				*tr = *setupTestRepoWithCommit(t)

				// Stage a change
				err := tr.fs.WriteFile("test.txt", []byte("modified content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			message: "Amended commit",
			signature: Signature{
				Name:  "Test Author",
				Email: "test@example.com",
				When:  time.Now(),
			},
			opts:        CommitOpts{Amend: true},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo, commitSHA string) {
				// Verify commit was created
				assert.NotEmpty(t, commitSHA)
				assert.Len(t, commitSHA, 40)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Setup test repository
			var tr *testRepo
			if tt.name == "commit in bare repository" {
				tr = setupTestRepo(t, true) // Bare repository
			} else {
				tr = setupTestRepo(t, false) // Non-bare repository
			}

			// Run setup (some tests may replace tr entirely)
			tt.setup(t, tr)

			// Execute Commit operation
			commitSHA, err := tr.repo.Commit(ctx, tt.message, tt.signature, tt.opts)

			// Verify error expectation
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}

			// Run validation
			if !tt.expectError {
				tt.validate(t, tr, commitSHA)
			}
		})
	}
}

// TestAdd tests the Add method for staging files
func TestAdd(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, tr *testRepo)
		paths       []string
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, tr *testRepo)
	}{
		{
			name: "add single file",
			setup: func(t *testing.T, tr *testRepo) {
				// Create a test file
				err := tr.fs.WriteFile("test.txt", []byte("test content"), 0o644)
				require.NoError(t, err)
			},
			paths:       []string{"test.txt"},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify file is staged
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				fileStatus := status.File("test.txt")
				assert.Equal(t, git.Added, fileStatus.Staging)
			},
		},
		{
			name: "add multiple files",
			setup: func(t *testing.T, tr *testRepo) {
				// Create multiple test files
				err := tr.fs.WriteFile("file1.txt", []byte("content 1"), 0o644)
				require.NoError(t, err)
				err = tr.fs.WriteFile("file2.txt", []byte("content 2"), 0o644)
				require.NoError(t, err)
			},
			paths:       []string{"file1.txt", "file2.txt"},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify both files are staged
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				file1Status := status.File("file1.txt")
				file2Status := status.File("file2.txt")
				assert.Equal(t, git.Added, file1Status.Staging)
				assert.Equal(t, git.Added, file2Status.Staging)
			},
		},
		{
			name: "add with glob pattern",
			setup: func(t *testing.T, tr *testRepo) {
				// Create files with similar names
				err := tr.fs.WriteFile("test1.txt", []byte("content 1"), 0o644)
				require.NoError(t, err)
				err = tr.fs.WriteFile("test2.txt", []byte("content 2"), 0o644)
				require.NoError(t, err)
				err = tr.fs.WriteFile("other.txt", []byte("other content"), 0o644)
				require.NoError(t, err)
			},
			paths:       []string{"test*.txt"},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify matching files are staged, non-matching is not
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				test1Status := status.File("test1.txt")
				test2Status := status.File("test2.txt")
				otherStatus := status.File("other.txt")
				assert.Equal(t, git.Added, test1Status.Staging)
				assert.Equal(t, git.Added, test2Status.Staging)
				assert.Equal(t, git.Untracked, otherStatus.Worktree)
			},
		},
		{
			name: "add non-existent file",
			setup: func(t *testing.T, tr *testRepo) {
				// No setup needed
			},
			paths:       []string{"nonexistent.txt"},
			expectError: false, // git add silently ignores missing files
			validate: func(t *testing.T, tr *testRepo) {
				// File should not be in status
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				fileStatus := status.File("nonexistent.txt")
				assert.Equal(t, git.Untracked, fileStatus.Worktree)
				assert.Equal(t, git.Untracked, fileStatus.Staging)
			},
		},
		{
			name: "add empty paths",
			setup: func(t *testing.T, tr *testRepo) {
				// Create a test file
				err := tr.fs.WriteFile("test.txt", []byte("test content"), 0o644)
				require.NoError(t, err)
			},
			paths:       []string{"", "test.txt", ""},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo) {
				// Verify file is staged despite empty paths
				status, err := tr.repo.worktree.Status()
				require.NoError(t, err)
				fileStatus := status.File("test.txt")
				assert.Equal(t, git.Added, fileStatus.Staging)
			},
		},
		{
			name:        "add no paths",
			setup:       func(t *testing.T, tr *testRepo) {},
			paths:       []string{},
			expectError: false, // No paths is not an error
			validate:    func(t *testing.T, tr *testRepo) {},
		},
		{
			name: "add in bare repository",
			setup: func(t *testing.T, tr *testRepo) {
				// This test uses a bare repository setup
			},
			paths:       []string{"test.txt"},
			expectError: true,
			errorMsg:    "cannot add files in bare repository",
			validate:    func(t *testing.T, tr *testRepo) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Setup test repository
			var tr *testRepo
			if tt.name == "add in bare repository" {
				tr = setupTestRepo(t, true) // Bare repository
			} else {
				tr = setupTestRepo(t, false) // Non-bare repository
			}

			// Run setup
			tt.setup(t, tr)

			// Execute Add operation
			err := tr.repo.Add(ctx, tt.paths...)

			// Verify error expectation
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}

			// Run validation
			if !tt.expectError {
				tt.validate(t, tr)
			}
		})
	}
}

// TestLog tests the Log operation with various filters
func TestLog(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) *testRepo
		filter      LogFilter
		expectError bool
		validate    func(t *testing.T, iter *CommitIter, err error)
	}{
		{
			name:        "basic log without filters",
			setup:       setupTestRepoWithCommit,
			filter:      LogFilter{},
			expectError: false,
			validate: func(t *testing.T, iter *CommitIter, err error) {
				require.NoError(t, err)
				require.NotNil(t, iter)

				// Should have at least one commit
				commit, err := iter.Next()
				require.NoError(t, err)
				require.NotNil(t, commit)
				assert.Equal(t, "Initial commit", commit.Message)

				// Should be end of iteration
				nextCommit, err := iter.Next()
				require.NoError(t, err)
				assert.Nil(t, nextCommit)

				iter.Close()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)

			ctx := context.Background()
			iter, err := tr.repo.Log(ctx, tt.filter)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			tt.validate(t, iter, err)
		})
	}
}

// TestDiff tests the Diff operation between revisions
func TestDiff(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) *testRepo
		revA        string
		revB        string
		filters     []ChangeFilter
		expectError bool
		validate    func(t *testing.T, patch *PatchText, err error)
	}{
		{
			name: "diff between commits",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create second commit with changes
				tr.modifyTestFile(t, "modified content")
				_, err := tr.repo.worktree.Add("test.txt")
				require.NoError(t, err)
				_, err = tr.repo.worktree.Commit("Second commit", &git.CommitOptions{})
				require.NoError(t, err)

				return tr
			},
			revA:        "HEAD~1",
			revB:        "HEAD",
			filters:     nil,
			expectError: false,
			validate: func(t *testing.T, patch *PatchText, err error) {
				require.NoError(t, err)
				require.NotNil(t, patch)
				assert.Contains(t, patch.Text, "diff --git")
				assert.Contains(t, patch.Text, "test.txt")
				assert.False(t, patch.IsBinary)
				assert.Greater(t, patch.FileCount, 0)
			},
		},
		{
			name: "diff with path filter",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create multiple files in second commit
				err := tr.fs.WriteFile("file1.go", []byte("go content"), 0o644)
				require.NoError(t, err)
				err = tr.fs.WriteFile("file2.md", []byte("markdown content"), 0o644)
				require.NoError(t, err)

				_, err = tr.repo.worktree.Add("file1.go")
				require.NoError(t, err)
				_, err = tr.repo.worktree.Add("file2.md")
				require.NoError(t, err)

				_, err = tr.repo.worktree.Commit("Add multiple files", &git.CommitOptions{})
				require.NoError(t, err)

				return tr
			},
			revA: "HEAD~1",
			revB: "HEAD",
			filters: []ChangeFilter{
				ChangeExtensionFilter(".go"), // Only include .go files
			},
			expectError: false,
			validate: func(t *testing.T, patch *PatchText, err error) {
				require.NoError(t, err)
				require.NotNil(t, patch)
				assert.Contains(t, patch.Text, "file1.go")
				assert.NotContains(t, patch.Text, "file2.md")
				assert.Equal(t, 1, patch.FileCount)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			ctx := context.Background()

			patch, err := tr.repo.Diff(ctx, tt.revA, tt.revB, tt.filters...)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			tt.validate(t, patch, err)
		})
	}
}

// TestCreateTag tests tag creation operations
func TestCreateTag(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) *testRepo
		tagName     string
		target      string
		message     string
		annotated   bool
		expectError bool
		validate    func(t *testing.T, tr *testRepo, err error)
	}{
		{
			name:        "create lightweight tag on HEAD",
			setup:       setupTestRepoWithCommit,
			tagName:     "v1.0.0",
			target:      "HEAD",
			message:     "",
			annotated:   false,
			expectError: false,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.NoError(t, err)

				// Verify tag was created
				tags, err := tr.repo.Tags(context.Background())
				require.NoError(t, err)
				assert.Contains(t, tags, "v1.0.0")

				// Verify it's a lightweight tag (no message)
				ref, err := tr.repo.repo.Reference(plumbing.NewTagReferenceName("v1.0.0"), true)
				require.NoError(t, err)
				assert.Equal(t, plumbing.HashReference, ref.Type())
			},
		},
		{
			name:        "create annotated tag with message",
			setup:       setupTestRepoWithCommit,
			tagName:     "v2.0.0",
			target:      "HEAD",
			message:     "Release version 2.0.0",
			annotated:   true,
			expectError: false,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.NoError(t, err)

				// Verify tag was created
				tags, err := tr.repo.Tags(context.Background())
				require.NoError(t, err)
				assert.Contains(t, tags, "v2.0.0")

				// Verify it's an annotated tag (tag object exists)
				tagRef, err := tr.repo.repo.Reference(plumbing.NewTagReferenceName("v2.0.0"), true)
				require.NoError(t, err)
				tagObj, err := tr.repo.repo.TagObject(tagRef.Hash())
				require.NoError(t, err)
				assert.Equal(t, "Release version 2.0.0", strings.TrimSpace(tagObj.Message))
			},
		},
		{
			name: "create tag on specific commit",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create second commit
				tr.modifyTestFile(t, "second commit content")
				_, err := tr.repo.worktree.Add("test.txt")
				require.NoError(t, err)
				_, err = tr.repo.worktree.Commit("Second commit", &git.CommitOptions{})
				require.NoError(t, err)

				return tr
			},
			tagName:     "v1.5.0",
			target:      "HEAD~1",
			message:     "Tag on first commit",
			annotated:   true,
			expectError: false,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.NoError(t, err)

				// Verify tag points to first commit, not HEAD
				tagRef, err := tr.repo.repo.Reference(plumbing.NewTagReferenceName("v1.5.0"), true)
				require.NoError(t, err)
				tagObj, err := tr.repo.repo.TagObject(tagRef.Hash())
				require.NoError(t, err)

				head, err := tr.repo.repo.Head()
				require.NoError(t, err)

				assert.NotEqual(t, head.Hash(), tagObj.Target)
			},
		},
		{
			name: "fail to create duplicate tag",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create first tag
				err := tr.repo.CreateTag(context.Background(), "duplicate", "HEAD", "", false)
				require.NoError(t, err)

				return tr
			},
			tagName:     "duplicate",
			target:      "HEAD",
			message:     "",
			annotated:   false,
			expectError: true,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrTagExists))
			},
		},
		{
			name:        "fail with empty tag name",
			setup:       setupTestRepoWithCommit,
			tagName:     "",
			target:      "HEAD",
			message:     "",
			annotated:   false,
			expectError: true,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidRef))
			},
		},
		{
			name:        "fail with empty target",
			setup:       setupTestRepoWithCommit,
			tagName:     "test-tag",
			target:      "",
			message:     "",
			annotated:   false,
			expectError: true,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidRef))
			},
		},
		{
			name:        "fail with invalid target revision",
			setup:       setupTestRepoWithCommit,
			tagName:     "invalid-target",
			target:      "nonexistent-branch",
			message:     "",
			annotated:   false,
			expectError: true,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrResolveFailed))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			ctx := context.Background()

			err := tr.repo.CreateTag(ctx, tt.tagName, tt.target, tt.message, tt.annotated)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			tt.validate(t, tr, err)
		})
	}
}

// TestDeleteTag tests tag deletion operations
func TestDeleteTag(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) *testRepo
		tagName     string
		expectError bool
		validate    func(t *testing.T, tr *testRepo, err error)
	}{
		{
			name: "delete existing lightweight tag",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create a tag to delete
				err := tr.repo.CreateTag(context.Background(), "to-delete", "HEAD", "", false)
				require.NoError(t, err)

				return tr
			},
			tagName:     "to-delete",
			expectError: false,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.NoError(t, err)

				// Verify tag was deleted
				tags, err := tr.repo.Tags(context.Background())
				require.NoError(t, err)
				assert.NotContains(t, tags, "to-delete")
			},
		},
		{
			name: "delete existing annotated tag",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create an annotated tag to delete
				err := tr.repo.CreateTag(context.Background(), "annotated-delete", "HEAD", "Delete me", true)
				require.NoError(t, err)

				return tr
			},
			tagName:     "annotated-delete",
			expectError: false,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.NoError(t, err)

				// Verify tag was deleted
				tags, err := tr.repo.Tags(context.Background())
				require.NoError(t, err)
				assert.NotContains(t, tags, "annotated-delete")
			},
		},
		{
			name:        "fail to delete non-existent tag",
			setup:       setupTestRepoWithCommit,
			tagName:     "nonexistent",
			expectError: true,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrTagMissing))
			},
		},
		{
			name:        "fail with empty tag name",
			setup:       setupTestRepoWithCommit,
			tagName:     "",
			expectError: true,
			validate: func(t *testing.T, tr *testRepo, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidRef))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			ctx := context.Background()

			err := tr.repo.DeleteTag(ctx, tt.tagName)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			tt.validate(t, tr, err)
		})
	}
}

// TestTags tests tag listing operations
func TestTags(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) *testRepo
		filters     []TagFilter
		expectError bool
		validate    func(t *testing.T, tags []string, err error)
	}{
		{
			name: "list all tags",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create multiple tags
				err := tr.repo.CreateTag(context.Background(), "v1.0.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "v2.0.0", "HEAD", "Version 2.0.0", true)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "v1.1.0", "HEAD", "", false)
				require.NoError(t, err)

				return tr
			},
			filters:     nil, // No filters means all tags
			expectError: false,
			validate: func(t *testing.T, tags []string, err error) {
				require.NoError(t, err)
				assert.Len(t, tags, 3)
				assert.Contains(t, tags, "v1.0.0")
				assert.Contains(t, tags, "v2.0.0")
				assert.Contains(t, tags, "v1.1.0")

				// Verify alphabetical sorting
				assert.Equal(t, []string{"v1.0.0", "v1.1.0", "v2.0.0"}, tags)
			},
		},
		{
			name: "list tags with pattern filter",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create tags with different patterns
				err := tr.repo.CreateTag(context.Background(), "v1.0.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "v2.0.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "beta-1.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "alpha-1.0", "HEAD", "", false)
				require.NoError(t, err)

				return tr
			},
			filters:     []TagFilter{TagPatternFilter("v*")},
			expectError: false,
			validate: func(t *testing.T, tags []string, err error) {
				require.NoError(t, err)
				assert.Len(t, tags, 2)
				assert.Contains(t, tags, "v1.0.0")
				assert.Contains(t, tags, "v2.0.0")
				assert.NotContains(t, tags, "beta-1.0")
				assert.NotContains(t, tags, "alpha-1.0")
			},
		},
		{
			name: "list tags with prefix filter",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create tags with different prefixes
				err := tr.repo.CreateTag(context.Background(), "v1.0.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "v2.0.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "beta-1.0", "HEAD", "", false)
				require.NoError(t, err)

				return tr
			},
			filters:     []TagFilter{TagPrefixFilter("v")},
			expectError: false,
			validate: func(t *testing.T, tags []string, err error) {
				require.NoError(t, err)
				assert.Len(t, tags, 2)
				assert.Contains(t, tags, "v1.0.0")
				assert.Contains(t, tags, "v2.0.0")
				assert.NotContains(t, tags, "beta-1.0")
			},
		},
		{
			name: "list tags with suffix filter",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create tags with different suffixes
				err := tr.repo.CreateTag(context.Background(), "v1.0.0-rc", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "v2.0.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "beta-rc", "HEAD", "", false)
				require.NoError(t, err)

				return tr
			},
			filters:     []TagFilter{TagSuffixFilter("-rc")},
			expectError: false,
			validate: func(t *testing.T, tags []string, err error) {
				require.NoError(t, err)
				assert.Len(t, tags, 2)
				assert.Contains(t, tags, "v1.0.0-rc")
				assert.Contains(t, tags, "beta-rc")
				assert.NotContains(t, tags, "v2.0.0")
			},
		},
		{
			name: "list tags with exclude filter",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create tags
				err := tr.repo.CreateTag(context.Background(), "v1.0.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "v1.0.0-rc", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "v2.0.0-rc", "HEAD", "", false)
				require.NoError(t, err)

				return tr
			},
			filters:     []TagFilter{TagExcludeFilter("*-rc")},
			expectError: false,
			validate: func(t *testing.T, tags []string, err error) {
				require.NoError(t, err)
				assert.Len(t, tags, 1)
				assert.Contains(t, tags, "v1.0.0")
				assert.NotContains(t, tags, "v1.0.0-rc")
				assert.NotContains(t, tags, "v2.0.0-rc")
			},
		},
		{
			name: "list tags with multiple filters",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				// Create various tags
				err := tr.repo.CreateTag(context.Background(), "v1.0.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "v1.0.0-rc", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "v2.0.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "beta-1.0", "HEAD", "", false)
				require.NoError(t, err)

				return tr
			},
			// Only tags starting with "v" AND not ending with "-rc"
			filters: []TagFilter{
				TagPrefixFilter("v"),
				TagExcludeFilter("*-rc"),
			},
			expectError: false,
			validate: func(t *testing.T, tags []string, err error) {
				require.NoError(t, err)
				assert.Len(t, tags, 2)
				assert.Contains(t, tags, "v1.0.0")
				assert.Contains(t, tags, "v2.0.0")
				assert.NotContains(t, tags, "v1.0.0-rc")
				assert.NotContains(t, tags, "beta-1.0")
			},
		},
		{
			name: "list tags with exact match pattern",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				err := tr.repo.CreateTag(context.Background(), "exact-match", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(context.Background(), "not-match", "HEAD", "", false)
				require.NoError(t, err)

				return tr
			},
			filters:     []TagFilter{TagPatternFilter("exact-match")},
			expectError: false,
			validate: func(t *testing.T, tags []string, err error) {
				require.NoError(t, err)
				assert.Len(t, tags, 1)
				assert.Equal(t, []string{"exact-match"}, tags)
			},
		},
		{
			name: "list tags with no matches",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)

				err := tr.repo.CreateTag(context.Background(), "v1.0.0", "HEAD", "", false)
				require.NoError(t, err)

				return tr
			},
			filters:     []TagFilter{TagPatternFilter("nonexistent*")},
			expectError: false,
			validate: func(t *testing.T, tags []string, err error) {
				require.NoError(t, err)
				assert.Len(t, tags, 0)
			},
		},
		{
			name:        "list tags in empty repository",
			setup:       setupTestRepoWithCommit,
			filters:     nil,
			expectError: false,
			validate: func(t *testing.T, tags []string, err error) {
				require.NoError(t, err)
				assert.Len(t, tags, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			ctx := context.Background()

			tags, err := tr.repo.Tags(ctx, tt.filters...)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			tt.validate(t, tags, err)
		})
	}
}
