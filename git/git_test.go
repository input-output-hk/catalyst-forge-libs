package git

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	billyfs "github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/git/internal/fsbridge"
)

// TestRefs tests the Refs method with various reference kinds and patterns
func TestRefs(t *testing.T) {
	tests := []struct {
		name     string
		kind     RefKind
		pattern  string
		setup    func(t *testing.T) *testRepo
		expected []string
		wantErr  bool
	}{
		{
			name: "list all branches",
			kind: RefBranch,
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				err := tr.repo.CreateBranch(tr.ctx, "feature-branch", "master", false, false)
				require.NoError(t, err)
				err = tr.repo.CreateBranch(tr.ctx, "bugfix-branch", "master", false, false)
				require.NoError(t, err)
				return tr
			},
			expected: []string{"bugfix-branch", "feature-branch", "master"},
		},
		{
			name:    "list branches with pattern",
			kind:    RefBranch,
			pattern: "feature-*",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				err := tr.repo.CreateBranch(tr.ctx, "feature-one", "master", false, false)
				require.NoError(t, err)
				err = tr.repo.CreateBranch(tr.ctx, "feature-two", "master", false, false)
				require.NoError(t, err)
				err = tr.repo.CreateBranch(tr.ctx, "bugfix-one", "master", false, false)
				require.NoError(t, err)
				return tr
			},
			expected: []string{"feature-one", "feature-two"},
		},
		{
			name: "list tags",
			kind: RefTag,
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				err := tr.repo.CreateTag(tr.ctx, "v1.0.0", "HEAD", "Release v1.0.0", true)
				require.NoError(t, err)
				err = tr.repo.CreateTag(tr.ctx, "v1.1.0", "HEAD", "", false)
				require.NoError(t, err)
				return tr
			},
			expected: []string{"v1.0.0", "v1.1.0"},
		},
		{
			name:    "list tags with pattern",
			kind:    RefTag,
			pattern: "v1.*",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				err := tr.repo.CreateTag(tr.ctx, "v1.0.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(tr.ctx, "v1.1.0", "HEAD", "", false)
				require.NoError(t, err)
				err = tr.repo.CreateTag(tr.ctx, "v2.0.0", "HEAD", "", false)
				require.NoError(t, err)
				return tr
			},
			expected: []string{"v1.0.0", "v1.1.0"},
		},
		{
			name: "list remote branches",
			kind: RefRemoteBranch,
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				tr.createRemoteBranch(t, "origin", "main")
				tr.createRemoteBranch(t, "origin", "develop")
				tr.createRemoteBranch(t, "upstream", "master")
				return tr
			},
			expected: []string{"origin/develop", "origin/main", "upstream/master"},
		},
		{
			name:    "list remote branches with pattern",
			kind:    RefRemoteBranch,
			pattern: "origin/*",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				tr.createRemoteBranch(t, "origin", "main")
				tr.createRemoteBranch(t, "origin", "develop")
				tr.createRemoteBranch(t, "upstream", "master")
				return tr
			},
			expected: []string{"origin/develop", "origin/main"},
		},
		{
			name: "list other references",
			kind: RefOther,
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				// Create HEAD reference (should be classified as other)
				headRef := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("master"))
				err := tr.repo.repo.Storer.SetReference(headRef)
				require.NoError(t, err)
				return tr
			},
			expected: []string{"HEAD"},
		},
		{
			name: "empty repository",
			kind: RefBranch,
			setup: func(t *testing.T) *testRepo {
				return setupTestRepo(t, false)
			},
			expected: []string{}, // Empty repo has no branches yet
		},
		{
			name:    "pattern with no matches",
			kind:    RefBranch,
			pattern: "non-existent-*",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				err := tr.repo.CreateBranch(tr.ctx, "feature-one", "master", false, false)
				require.NoError(t, err)
				return tr
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			refs, err := tr.repo.Refs(tr.ctx, tt.kind, tt.pattern)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Sort both expected and actual for comparison
			expected := make([]string, len(tt.expected))
			copy(expected, tt.expected)
			for i := 0; i < len(expected)-1; i++ {
				for j := i + 1; j < len(expected); j++ {
					if expected[i] > expected[j] {
						expected[i], expected[j] = expected[j], expected[i]
					}
				}
			}

			actual := make([]string, len(refs))
			copy(actual, refs)
			for i := 0; i < len(actual)-1; i++ {
				for j := i + 1; j < len(actual); j++ {
					if actual[i] > actual[j] {
						actual[i], actual[j] = actual[j], actual[i]
					}
				}
			}

			assert.Equal(t, expected, actual)
		})
	}
}

// TestResolve tests the Resolve method with various revision specifications
func TestResolve(t *testing.T) {
	tests := []struct {
		name           string
		revision       string
		setup          func(t *testing.T) *testRepo
		expectedKind   RefKind
		expectedPrefix string // prefix of the canonical name or hash
		wantErr        bool
	}{
		{
			name:           "resolve branch name",
			revision:       "master",
			setup:          setupTestRepoWithCommit,
			expectedKind:   RefBranch,
			expectedPrefix: "refs/heads/master",
		},
		{
			name:           "resolve HEAD",
			revision:       "HEAD",
			setup:          setupTestRepoWithCommit,
			expectedKind:   RefOther,
			expectedPrefix: "HEAD",
		},
		{
			name:         "resolve commit hash",
			revision:     "", // Will be set dynamically in test
			setup:        setupTestRepoWithCommit,
			expectedKind: RefCommit,
		},
		{
			name:     "resolve tag",
			revision: "v1.0.0",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				err := tr.repo.CreateTag(tr.ctx, "v1.0.0", "HEAD", "", false)
				require.NoError(t, err)
				return tr
			},
			expectedKind:   RefTag,
			expectedPrefix: "refs/tags/v1.0.0",
		},
		{
			name:     "resolve remote branch",
			revision: "origin/main",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				tr.createRemoteBranch(t, "origin", "main")
				return tr
			},
			expectedKind:   RefRemoteBranch,
			expectedPrefix: "refs/remotes/origin/main",
		},
		{
			name:     "resolve non-existent revision",
			revision: "non-existent",
			setup:    setupTestRepoWithCommit,
			wantErr:  true,
		},
		{
			name:     "resolve empty revision",
			revision: "",
			setup:    setupTestRepoWithCommit,
			wantErr:  true,
		},
		{
			name:         "resolve commit hash",
			revision:     "HEAD",
			setup:        setupTestRepoWithCommit,
			expectedKind: RefCommit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)

			// Handle dynamic revision for commit hash test
			revision := tt.revision
			if tt.name == "resolve commit hash" {
				hash, err := tr.repo.repo.ResolveRevision(plumbing.Revision("HEAD"))
				require.NoError(t, err)
				revision = hash.String()[:7] // Short hash
			}

			resolved, err := tr.repo.Resolve(tr.ctx, revision)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, resolved)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resolved)
			assert.Equal(t, tt.expectedKind, resolved.Kind)
			assert.NotEmpty(t, resolved.Hash)
			assert.NotEmpty(t, resolved.CanonicalName)

			// For specific cases, check the canonical name prefix
			if tt.expectedPrefix != "" {
				assert.Equal(t, tt.expectedPrefix, resolved.CanonicalName)
			}

			// Verify the hash is valid
			hash := plumbing.NewHash(resolved.Hash)
			assert.NotEmpty(t, hash.String(), "resolved hash should be valid")
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

// TestLog tests the Log method with various filters
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
