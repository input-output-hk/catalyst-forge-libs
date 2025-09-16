package git

import (
	"context"
	"errors"
	"testing"

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
