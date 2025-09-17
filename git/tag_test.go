package git

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				_, err = tr.repo.Commit(
					tr.ctx,
					"Second commit",
					Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
					CommitOpts{},
				)
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
