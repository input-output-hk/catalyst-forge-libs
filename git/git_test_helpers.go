package git

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/input-output-hk/catalyst-forge-libs/fs"
	fsb "github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	"github.com/stretchr/testify/require"
)

// testRepo is a helper struct that contains a test repository and its filesystem
type testRepo struct {
	repo *Repo
	fs   fs.Filesystem
	ctx  context.Context
}

// setupTestRepo creates a new test repository with an in-memory filesystem
func setupTestRepo(t *testing.T, bare bool) *testRepo {
	t.Helper()

	ctx := context.Background()
	memFS := fsb.NewInMemoryFS()

	opts := Options{
		FS:      memFS,
		Bare:    bare,
		Workdir: ".",
	}

	repo, err := Init(ctx, &opts)
	require.NoError(t, err, "failed to initialize test repository")
	require.NotNil(t, repo, "repository should not be nil")

	return &testRepo{
		repo: repo,
		fs:   memFS,
		ctx:  ctx,
	}
}

// setupTestRepoWithCommit creates a test repository with an initial commit
func setupTestRepoWithCommit(t *testing.T) *testRepo {
	t.Helper()

	tr := setupTestRepo(t, false)

	// Create a test file
	testFile, err := tr.fs.Create("test.txt")
	require.NoError(t, err, "failed to create test file")

	_, err = testFile.Write([]byte("initial content"))
	require.NoError(t, err, "failed to write test file")

	err = testFile.Close()
	require.NoError(t, err, "failed to close test file")

	// Add and commit the file
	_, err = tr.repo.worktree.Add("test.txt")
	require.NoError(t, err, "failed to add test file")

	_, err = tr.repo.worktree.Commit("Initial commit", &git.CommitOptions{})
	require.NoError(t, err, "failed to create initial commit")

	return tr
}

// createTestBranch creates a branch in the test repository
func (tr *testRepo) createTestBranch(t *testing.T, branchName string) {
	t.Helper()

	head, err := tr.repo.repo.Head()
	require.NoError(t, err, "failed to get HEAD")

	branchRef := plumbing.NewBranchReferenceName(branchName)
	newRef := plumbing.NewHashReference(branchRef, head.Hash())
	err = tr.repo.repo.Storer.SetReference(newRef)
	require.NoError(t, err, "failed to create branch reference")
}

// createRemoteBranch creates a mock remote branch reference
func (tr *testRepo) createRemoteBranch(t *testing.T, remoteName, branchName string) {
	t.Helper()

	head, err := tr.repo.repo.Head()
	require.NoError(t, err, "failed to get HEAD")

	remoteBranchRef := plumbing.NewRemoteReferenceName(remoteName, branchName)
	remoteRef := plumbing.NewHashReference(remoteBranchRef, head.Hash())
	err = tr.repo.repo.Storer.SetReference(remoteRef)
	require.NoError(t, err, "failed to create remote branch reference")
}

// modifyTestFile modifies the test.txt file with new content
func (tr *testRepo) modifyTestFile(t *testing.T, content string) {
	t.Helper()

	err := tr.fs.WriteFile("test.txt", []byte(content), 0o666)
	require.NoError(t, err, "failed to modify test file")
}

// getCurrentBranch gets the current branch name
func (tr *testRepo) getCurrentBranch(t *testing.T) string {
	t.Helper()

	branch, err := tr.repo.CurrentBranch(tr.ctx)
	require.NoError(t, err, "failed to get current branch")

	return branch
}

// verifyBranchExists checks that a branch exists
func (tr *testRepo) verifyBranchExists(t *testing.T, branchName string) {
	t.Helper()

	branchRef, err := tr.repo.repo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	require.NoError(t, err, "branch should exist: %s", branchName)
	require.NotNil(t, branchRef, "branch reference should not be nil")
}

// verifyBranchNotExists checks that a branch does not exist
func (tr *testRepo) verifyBranchNotExists(t *testing.T, branchName string) {
	t.Helper()

	_, err := tr.repo.repo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	require.Error(t, err, "branch should not exist: %s", branchName)
}
