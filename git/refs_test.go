package git

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			expected: []string{"refs/heads/bugfix-branch", "refs/heads/feature-branch", "refs/heads/master"},
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
			expected: []string{"refs/heads/feature-one", "refs/heads/feature-two"},
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
			expected: []string{"refs/tags/v1.0.0", "refs/tags/v1.1.0"},
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
			expected: []string{"refs/tags/v1.0.0", "refs/tags/v1.1.0"},
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
			expected: []string{
				"refs/remotes/origin/develop",
				"refs/remotes/origin/main",
				"refs/remotes/upstream/master",
			},
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
			expected: []string{"refs/remotes/origin/develop", "refs/remotes/origin/main"},
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
			expected: nil,
		},
		{
			name:    "pattern with no matches",
			kind:    RefBranch,
			pattern: "nonexistent-*",
			setup: func(t *testing.T) *testRepo {
				tr := setupTestRepoWithCommit(t)
				err := tr.repo.CreateBranch(tr.ctx, "feature-branch", "master", false, false)
				require.NoError(t, err)
				return tr
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.setup(t)
			ctx := context.Background()

			refs, err := tr.repo.Refs(ctx, tt.kind, tt.pattern)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Handle nil vs empty slice comparison
			if (tt.expected == nil && refs == nil) ||
				(tt.expected == nil && len(refs) == 0) ||
				(len(tt.expected) == 0 && refs == nil) {
				// Test passes for nil/empty slice equivalency
				return
			}
			assert.Equal(t, tt.expected, refs)
		})
	}
}

// TestResolve tests the Resolve method with various revision types
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
			expectedKind:   RefCommit,
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
				if tt.expectedPrefix == "HEAD" {
					assert.Equal(t, tt.expectedPrefix, resolved.CanonicalName)
				} else {
					assert.Contains(t, resolved.CanonicalName, tt.expectedPrefix)
				}
			}
		})
	}
}
