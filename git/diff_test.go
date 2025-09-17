package git

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				_, err = tr.repo.Commit(
					tr.ctx,
					"Second commit",
					Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
					CommitOpts{},
				)
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

				_, err = tr.repo.Commit(
					tr.ctx,
					"Add multiple files",
					Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
					CommitOpts{},
				)
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
