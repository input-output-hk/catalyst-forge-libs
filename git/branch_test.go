package git

import (
	"errors"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
