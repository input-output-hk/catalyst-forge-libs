package git

import (
	"context"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				// Execute Unstage operation
				err := tr.repo.Unstage(context.Background(), "test.txt")
				require.NoError(t, err)

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
				// Create a file
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
				assert.Equal(t, git.Added, fileStatus.Staging, "file should be staged")
			},
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

// TestCommit tests the Commit method for creating commits
func TestCommit(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, tr *testRepo)
		message     string
		who         Signature
		opts        CommitOpts
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, tr *testRepo, commitHash string)
	}{
		{
			name: "commit staged changes",
			setup: func(t *testing.T, tr *testRepo) {
				// Create and stage a file
				err := tr.fs.WriteFile("test.txt", []byte("test content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			message:     "Initial commit",
			who:         Signature{Name: "Test User", Email: "test@example.com", When: time.Now()},
			opts:        CommitOpts{},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo, commitHash string) {
				require.NotEmpty(t, commitHash, "commit hash should not be empty")

				// Verify commit exists
				commit, err := tr.repo.repo.CommitObject(plumbing.NewHash(commitHash))
				require.NoError(t, err)
				assert.Equal(t, "Initial commit", commit.Message)
				assert.Equal(t, "Test User", commit.Author.Name)
				assert.Equal(t, "test@example.com", commit.Author.Email)
			},
		},
		{
			name: "commit empty (no changes)",
			setup: func(t *testing.T, tr *testRepo) {
				// No staged changes
			},
			message:     "Empty commit",
			who:         Signature{Name: "Test User", Email: "test@example.com", When: time.Now()},
			opts:        CommitOpts{AllowEmpty: true},
			expectError: false,
			validate: func(t *testing.T, tr *testRepo, commitHash string) {
				require.NotEmpty(t, commitHash, "commit hash should not be empty")

				// Verify commit exists
				commit, err := tr.repo.repo.CommitObject(plumbing.NewHash(commitHash))
				require.NoError(t, err)
				assert.Equal(t, "Empty commit", commit.Message)
			},
		},
		{
			name: "commit empty without allow empty",
			setup: func(t *testing.T, tr *testRepo) {
				// No staged changes
			},
			message:     "Should fail empty commit",
			who:         Signature{Name: "Test User", Email: "test@example.com", When: time.Now()},
			opts:        CommitOpts{AllowEmpty: false},
			expectError: true,
			errorMsg:    "no changes staged for commit",
			validate:    func(t *testing.T, tr *testRepo, commitHash string) {},
		},
		{
			name: "commit with empty message",
			setup: func(t *testing.T, tr *testRepo) {
				// Create and stage a file
				err := tr.fs.WriteFile("test.txt", []byte("test content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			message:     "",
			who:         Signature{Name: "Test User", Email: "test@example.com", When: time.Now()},
			opts:        CommitOpts{},
			expectError: true,
			errorMsg:    "commit message cannot be empty",
			validate:    func(t *testing.T, tr *testRepo, commitHash string) {},
		},
		{
			name: "commit with invalid signature",
			setup: func(t *testing.T, tr *testRepo) {
				// Create and stage a file
				err := tr.fs.WriteFile("test.txt", []byte("test content"), 0o644)
				require.NoError(t, err)
				err = tr.repo.Add(context.Background(), "test.txt")
				require.NoError(t, err)
			},
			message:     "Test commit",
			who:         Signature{Name: "", Email: "test@example.com", When: time.Now()},
			opts:        CommitOpts{},
			expectError: true,
			errorMsg:    "committer name and email are required",
			validate:    func(t *testing.T, tr *testRepo, commitHash string) {},
		},
		{
			name: "commit in bare repository",
			setup: func(t *testing.T, tr *testRepo) {
				// This test uses a bare repository setup
			},
			message:     "Should fail in bare repo",
			who:         Signature{Name: "Test User", Email: "test@example.com", When: time.Now()},
			opts:        CommitOpts{},
			expectError: true,
			errorMsg:    "cannot commit in bare repository",
			validate:    func(t *testing.T, tr *testRepo, commitHash string) {},
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

			// Run setup
			tt.setup(t, tr)

			// Execute Commit operation
			commitHash, err := tr.repo.Commit(ctx, tt.message, tt.who, tt.opts)

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
				tt.validate(t, tr, commitHash)
			}
		})
	}
}
