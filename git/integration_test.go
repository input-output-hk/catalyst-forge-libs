package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	billyfs "github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationCloneAndPull tests the complete clone and fast-forward pull workflow from Section 14
func TestIntegrationCloneAndPull(t *testing.T) {
	ctx := context.Background()

	// Create origin repository
	originFS := billyfs.NewInMemoryFS()
	origin, err := Init(ctx, &Options{
		FS:      originFS,
		Bare:    true,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Create a working repository to push initial content
	workFS := billyfs.NewInMemoryFS()
	work, err := Init(ctx, &Options{
		FS:      workFS,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Add remote to work repo
	_, err = work.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"file://."},
	})
	require.NoError(t, err)

	// Create initial content
	err = workFS.WriteFile("README.md", []byte("# Test Project\n"), 0o644)
	require.NoError(t, err)

	// Stage and commit
	err = work.Add(ctx, "README.md")
	require.NoError(t, err)

	sha1, err := work.Commit(ctx, "Initial commit", Signature{
		Name:  "Test User",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, sha1)

	// Get the current branch name
	currentBranch, err := work.CurrentBranch(ctx)
	require.NoError(t, err)

	// Push to origin (using internal method for in-memory testing)
	err = pushInMemory(ctx, work, origin, currentBranch)
	require.NoError(t, err)

	// Check what's in origin first
	originRefs, err := origin.Refs(ctx, RefBranch, "")
	require.NoError(t, err)
	assert.NotEmpty(t, originRefs, "Origin should have at least one branch after push")

	// Clone from origin
	cloneFS := billyfs.NewInMemoryFS()
	cloned, err := cloneInMemory(ctx, origin, &Options{
		FS:           cloneFS,
		Workdir:      ".",
		ShallowDepth: 1,
	})
	require.NoError(t, err)
	assert.NotNil(t, cloned.worktree, "Cloned repo should have a worktree")

	// Check what remote branches we have in clone
	clonedRemoteRefs, err := cloned.Refs(ctx, RefRemoteBranch, "")
	require.NoError(t, err)
	assert.NotEmpty(t, clonedRemoteRefs, "Cloned repo should have remote branches")

	// Verify cloned content - check if we have a branch
	branch, err := cloned.CurrentBranch(ctx)
	if err == nil {
		assert.Equal(t, currentBranch, branch)
	}

	content, err := cloneFS.ReadFile("README.md")
	require.NoError(t, err)
	assert.Equal(t, "# Test Project\n", string(content))

	// Make changes in origin
	err = workFS.WriteFile("README.md", []byte("# Test Project\nUpdated\n"), 0o644)
	require.NoError(t, err)

	err = work.Add(ctx, "README.md")
	require.NoError(t, err)

	sha2, err := work.Commit(ctx, "Update README", Signature{
		Name:  "Test User",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, sha2)
	assert.NotEqual(t, sha1, sha2)

	err = pushInMemory(ctx, work, origin, currentBranch)
	require.NoError(t, err)

	// Pull changes (fast-forward)
	err = pullInMemory(ctx, cloned, origin)
	require.NoError(t, err)

	// Verify updated content
	updatedContent, err := cloneFS.ReadFile("README.md")
	require.NoError(t, err)
	assert.Equal(t, "# Test Project\nUpdated\n", string(updatedContent))
}

// TestIntegrationFeatureBranch tests the feature branch workflow from Section 14
func TestIntegrationFeatureBranch(t *testing.T) {
	ctx := context.Background()

	// Setup origin with initial content
	originFS := billyfs.NewInMemoryFS()
	origin, err := Init(ctx, &Options{
		FS:      originFS,
		Bare:    true,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Initialize working repo
	workFS := billyfs.NewInMemoryFS()
	work, err := Init(ctx, &Options{
		FS:      workFS,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Setup remote
	_, err = work.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"file://."},
	})
	require.NoError(t, err)

	// Create initial content on main
	err = workFS.WriteFile("main.go", []byte("package main\n\nfunc main() {}\n"), 0o644)
	require.NoError(t, err)

	err = work.Add(ctx, "main.go")
	require.NoError(t, err)

	_, err = work.Commit(ctx, "Initial commit", Signature{
		Name:  "Test User",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)

	// Get the current branch name
	currentBranch, err := work.CurrentBranch(ctx)
	require.NoError(t, err)

	err = pushInMemory(ctx, work, origin, currentBranch)
	require.NoError(t, err)

	// Fetch from origin
	err = fetchInMemory(ctx, work, origin, true)
	require.NoError(t, err)

	// Create feature branch from origin/currentBranch
	err = work.CreateBranch(ctx, "feature/x", fmt.Sprintf("origin/%s", currentBranch), false, false)
	require.NoError(t, err)

	// Checkout feature branch
	err = work.CheckoutBranch(ctx, "feature/x", false, false)
	require.NoError(t, err)

	currentBranch2, err := work.CurrentBranch(ctx)
	require.NoError(t, err)
	assert.Equal(t, "feature/x", currentBranch2)

	// Make changes on feature branch
	err = workFS.WriteFile("feature.go", []byte("package main\n\n// Feature X implementation\n"), 0o644)
	require.NoError(t, err)

	err = work.Add(ctx, "feature.go")
	require.NoError(t, err)

	featureSHA, err := work.Commit(ctx, "Add feature X", Signature{
		Name:  "Test User",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, featureSHA)

	// Push feature branch
	err = pushInMemory(ctx, work, origin, "feature/x")
	require.NoError(t, err)

	// Verify branch exists in origin
	refs, err := origin.Refs(ctx, RefBranch, "")
	require.NoError(t, err)
	assert.Contains(t, refs, "refs/heads/feature/x")
}

// TestIntegrationCommitAndPush tests the commit and push workflow from Section 14
func TestIntegrationCommitAndPush(t *testing.T) {
	ctx := context.Background()

	// Setup repositories
	originFS := billyfs.NewInMemoryFS()
	origin, err := Init(ctx, &Options{
		FS:      originFS,
		Bare:    true,
		Workdir: ".",
	})
	require.NoError(t, err)

	workFS := billyfs.NewInMemoryFS()
	work, err := Init(ctx, &Options{
		FS:      workFS,
		Workdir: ".",
	})
	require.NoError(t, err)

	_, err = work.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"file://."},
	})
	require.NoError(t, err)

	// Create directory structure
	err = workFS.MkdirAll("cmd/foo", 0o755)
	require.NoError(t, err)

	// Add file
	mainPath := filepath.Join("cmd", "foo", "main.go")
	err = workFS.WriteFile(mainPath, []byte("package main\n\nfunc main() {\n\t// foo implementation\n}\n"), 0o644)
	require.NoError(t, err)

	// Stage file
	err = work.Add(ctx, mainPath)
	require.NoError(t, err)

	// Commit
	commitSHA, err := work.Commit(ctx, "feat: foo", Signature{
		Name:  "Test User",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, commitSHA)

	// Get the current branch name
	currentBranch, err := work.CurrentBranch(ctx)
	require.NoError(t, err)

	// Push
	err = pushInMemory(ctx, work, origin, currentBranch)
	require.NoError(t, err)

	// Verify commit in origin
	originRefs, err := origin.Refs(ctx, RefBranch, "")
	require.NoError(t, err)
	// Check that origin has the branch we just pushed
	found := false
	for _, ref := range originRefs {
		if ref == fmt.Sprintf("refs/heads/%s", currentBranch) {
			found = true
			break
		}
	}
	assert.True(t, found, "Origin should have the pushed branch")
}

// TestIntegrationResolveDiff tests the resolve and diff workflow from Section 14
func TestIntegrationResolveDiff(t *testing.T) {
	ctx := context.Background()

	// Setup repository with history
	fs := billyfs.NewInMemoryFS()
	repo, err := Init(ctx, &Options{
		FS:      fs,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Create initial version
	err = fs.WriteFile("version.txt", []byte("v1.0.0\n"), 0o644)
	require.NoError(t, err)

	err = repo.Add(ctx, "version.txt")
	require.NoError(t, err)

	commit1, err := repo.Commit(ctx, "Initial version", Signature{
		Name:  "Test User",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)

	// Tag v1.2.3
	err = repo.CreateTag(ctx, "v1.2.3", commit1, "Version 1.2.3", true)
	require.NoError(t, err)

	// Update version
	err = fs.WriteFile("version.txt", []byte("v2.0.0\n"), 0o644)
	require.NoError(t, err)

	err = repo.Add(ctx, "version.txt")
	require.NoError(t, err)

	_, err = repo.Commit(ctx, "Update version to v2", Signature{
		Name:  "Test User",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)

	// Resolve tag
	resolved, err := repo.Resolve(ctx, "v1.2.3")
	require.NoError(t, err)
	assert.Equal(t, RefTag, resolved.Kind)
	assert.NotEmpty(t, resolved.Hash)

	// Get diff between HEAD~1 and HEAD
	diff, err := repo.Diff(ctx, "HEAD~1", "HEAD", nil)
	require.NoError(t, err)
	assert.Contains(t, diff.Text, "-v1.0.0")
	assert.Contains(t, diff.Text, "+v2.0.0")
}

// TestIntegrationCompleteWorkflow tests a complete development workflow
func TestIntegrationCompleteWorkflow(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize bare origin repository
	originFS := billyfs.NewInMemoryFS()
	origin, err := Init(ctx, &Options{
		FS:      originFS,
		Bare:    true,
		Workdir: ".",
	})
	require.NoError(t, err)

	// 2. Clone to developer workspace
	devFS := billyfs.NewInMemoryFS()
	dev, err := Init(ctx, &Options{
		FS:      devFS,
		Workdir: ".",
	})
	require.NoError(t, err)

	_, err = dev.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"file://."},
	})
	require.NoError(t, err)

	// 3. Create initial project structure
	err = devFS.MkdirAll("src", 0o755)
	require.NoError(t, err)
	err = devFS.MkdirAll("tests", 0o755)
	require.NoError(t, err)
	err = devFS.MkdirAll("docs", 0o755)
	require.NoError(t, err)

	// 4. Add initial files
	files := map[string]string{
		"README.md":          "# Project\n\nTest project for integration testing.\n",
		"src/main.go":        "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}\n",
		"tests/main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}\n",
		".gitignore":         "*.tmp\n*.log\n",
	}

	for path, content := range files {
		err = devFS.WriteFile(path, []byte(content), 0o644)
		require.NoError(t, err)
	}

	// 5. Stage all files
	err = dev.Add(ctx, ".")
	require.NoError(t, err)

	// 6. Initial commit
	sha1, err := dev.Commit(ctx, "Initial project setup", Signature{
		Name:  "Developer",
		Email: "dev@example.com",
	}, CommitOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, sha1)

	// Get the current branch name
	currentBranch, err := dev.CurrentBranch(ctx)
	require.NoError(t, err)

	// 7. Push to origin
	err = pushInMemory(ctx, dev, origin, currentBranch)
	require.NoError(t, err)

	// 8. Create and tag release
	err = dev.CreateTag(ctx, "v1.0.0", "HEAD", "Release version 1.0.0", true)
	require.NoError(t, err)

	// 9. Create feature branch
	err = dev.CreateBranch(ctx, "feature/new-feature", currentBranch, false, false)
	require.NoError(t, err)

	err = dev.CheckoutBranch(ctx, "feature/new-feature", false, false)
	require.NoError(t, err)

	// 10. Add feature
	err = devFS.WriteFile(
		"src/feature.go",
		[]byte("package main\n\n// NewFeature does something\nfunc NewFeature() string {\n\treturn \"feature\"\n}\n"),
		0o644,
	)
	require.NoError(t, err)

	err = dev.Add(ctx, "src/feature.go")
	require.NoError(t, err)

	sha2, err := dev.Commit(ctx, "Add new feature", Signature{
		Name:  "Developer",
		Email: "dev@example.com",
	}, CommitOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, sha2)
	assert.NotEqual(t, sha1, sha2)

	// 11. Push feature branch
	err = pushInMemory(ctx, dev, origin, "feature/new-feature")
	require.NoError(t, err)

	// 12. Switch back to main branch
	err = dev.CheckoutBranch(ctx, currentBranch, false, false)
	require.NoError(t, err)

	// 13. Fetch and merge feature branch
	err = fetchInMemory(ctx, dev, origin, false)
	require.NoError(t, err)

	err = mergeInMemory(ctx, dev, "origin/feature/new-feature", FastForwardOnly)
	require.NoError(t, err)

	// 14. Verify merged content
	exists, err := devFS.Exists("src/feature.go")
	require.NoError(t, err)
	assert.True(t, exists, "Feature file should exist after merge")

	// 15. Tag new release
	err = dev.CreateTag(ctx, "v1.1.0", "HEAD", "Release version 1.1.0 with new feature", true)
	require.NoError(t, err)

	// 16. List tags
	tags, err := dev.Tags(ctx, TagPatternFilter("v*"))
	require.NoError(t, err)
	assert.Len(t, tags, 2)
	assert.Contains(t, tags, "v1.0.0")
	assert.Contains(t, tags, "v1.1.0")

	// 17. Get log
	filter := LogFilter{
		MaxCount: 10,
	}
	iter, err := dev.Log(ctx, filter)
	require.NoError(t, err)

	var commits []string
	err = iter.ForEach(func(c *object.Commit) error {
		commits = append(commits, c.Message)
		return nil
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(commits), 2, "Should have at least 2 commits")

	// 18. Clean up feature branch
	err = dev.DeleteBranch(ctx, "feature/new-feature")
	require.NoError(t, err)

	// 19. Verify branch is deleted
	refs, err := dev.Refs(ctx, RefBranch, "")
	require.NoError(t, err)
	assert.NotContains(t, refs, "refs/heads/feature/new-feature")
}

// TestIntegrationStaging tests staging, unstaging, and committing workflows
func TestIntegrationStaging(t *testing.T) {
	ctx := context.Background()

	fs := billyfs.NewInMemoryFS()
	repo, err := Init(ctx, &Options{
		FS:      fs,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Create multiple files
	files := []string{"file1.txt", "file2.txt", "file3.txt"}
	for i, f := range files {
		content := fmt.Sprintf("Content of file %d\n", i+1)
		err = fs.WriteFile(f, []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Stage specific files
	err = repo.Add(ctx, "file1.txt", "file2.txt")
	require.NoError(t, err)

	// Commit staged files
	sha1, err := repo.Commit(ctx, "Add file1 and file2", Signature{
		Name:  "Test",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, sha1)

	// Modify committed files and stage
	err = fs.WriteFile("file1.txt", []byte("Modified content 1\n"), 0o644)
	require.NoError(t, err)

	err = repo.Add(ctx, "file1.txt")
	require.NoError(t, err)

	// Unstage file1
	err = repo.Unstage(ctx, "file1.txt")
	require.NoError(t, err)

	// Stage all remaining changes
	err = repo.Add(ctx, ".")
	require.NoError(t, err)

	// Commit everything
	sha2, err := repo.Commit(ctx, "Add remaining changes", Signature{
		Name:  "Test",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, sha2)
	assert.NotEqual(t, sha1, sha2)

	// Remove a file
	err = repo.Remove(ctx, "file3.txt")
	require.NoError(t, err)

	// Verify file is removed from worktree
	exists, err := fs.Exists("file3.txt")
	require.NoError(t, err)
	assert.False(t, exists)

	// Commit removal
	sha3, err := repo.Commit(ctx, "Remove file3", Signature{
		Name:  "Test",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, sha3)
}

// TestIntegrationHistory tests log filtering and history traversal
func TestIntegrationHistory(t *testing.T) {
	ctx := context.Background()

	fs := billyfs.NewInMemoryFS()
	repo, err := Init(ctx, &Options{
		FS:      fs,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Create commits by different authors
	authors := []struct {
		name  string
		email string
		file  string
		msg   string
	}{
		{"Alice", "alice@example.com", "alice.txt", "Alice's commit"},
		{"Bob", "bob@example.com", "bob.txt", "Bob's commit"},
		{"Charlie", "charlie@example.com", "charlie.txt", "Charlie's commit"},
		{"Alice", "alice@example.com", "alice2.txt", "Alice's second commit"},
	}

	for _, author := range authors {
		err = fs.WriteFile(author.file, []byte(author.msg), 0o644)
		require.NoError(t, err)

		err = repo.Add(ctx, author.file)
		require.NoError(t, err)

		_, err = repo.Commit(ctx, author.msg, Signature{
			Name:  author.name,
			Email: author.email,
		}, CommitOpts{})
		require.NoError(t, err)

		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Test author filter
	filter := LogFilter{
		Author: "Alice",
	}
	iter, err := repo.Log(ctx, filter)
	require.NoError(t, err)

	var aliceCommits []string
	err = iter.ForEach(func(c *object.Commit) error {
		aliceCommits = append(aliceCommits, c.Message)
		return nil
	})
	require.NoError(t, err)
	assert.Len(t, aliceCommits, 2, "Alice should have 2 commits")

	// Test max count
	filter = LogFilter{
		MaxCount: 2,
	}
	iter, err = repo.Log(ctx, filter)
	require.NoError(t, err)

	var limitedCommits []string
	err = iter.ForEach(func(c *object.Commit) error {
		limitedCommits = append(limitedCommits, c.Message)
		return nil
	})
	require.NoError(t, err)
	assert.Len(t, limitedCommits, 2, "Should return only 2 commits")

	// Test path filter
	filter = LogFilter{
		Path: []string{"bob.txt"},
	}
	iter, err = repo.Log(ctx, filter)
	require.NoError(t, err)

	var bobFileCommits []string
	err = iter.ForEach(func(c *object.Commit) error {
		bobFileCommits = append(bobFileCommits, c.Message)
		return nil
	})
	require.NoError(t, err)
	assert.Len(t, bobFileCommits, 1, "Should have 1 commit for bob.txt")
	assert.Equal(t, "Bob's commit", bobFileCommits[0])
}

// TestIntegrationTags tests comprehensive tag operations
func TestIntegrationTags(t *testing.T) {
	ctx := context.Background()

	fs := billyfs.NewInMemoryFS()
	repo, err := Init(ctx, &Options{
		FS:      fs,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Create some commits
	for i := 0; i < 3; i++ {
		filename := fmt.Sprintf("file%d.txt", i)
		err = fs.WriteFile(filename, []byte(fmt.Sprintf("Content %d", i)), 0o644)
		require.NoError(t, err)

		err = repo.Add(ctx, filename)
		require.NoError(t, err)

		_, err = repo.Commit(ctx, fmt.Sprintf("Commit %d", i), Signature{
			Name:  "Test",
			Email: "test@example.com",
		}, CommitOpts{})
		require.NoError(t, err)
	}

	// Create various tags
	tags := []struct {
		name      string
		target    string
		message   string
		annotated bool
	}{
		{"v1.0.0", "HEAD", "Release v1.0.0", true},
		{"v1.0.1", "HEAD~1", "Patch release", true},
		{"v2.0.0-beta", "HEAD", "", false}, // lightweight tag
		{"feature-tag", "HEAD~2", "Feature milestone", true},
	}

	for _, tag := range tags {
		err = repo.CreateTag(ctx, tag.name, tag.target, tag.message, tag.annotated)
		require.NoError(t, err)
	}

	// List all tags
	allTags, err := repo.Tags(ctx)
	require.NoError(t, err)
	assert.Len(t, allTags, 4)

	// List tags with pattern
	vTags, err := repo.Tags(ctx, TagPatternFilter("v*"))
	require.NoError(t, err)
	assert.Len(t, vTags, 3)

	// Resolve tag
	resolved, err := repo.Resolve(ctx, "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, RefTag, resolved.Kind)

	// Delete tag
	err = repo.DeleteTag(ctx, "feature-tag")
	require.NoError(t, err)

	// Verify deletion
	remainingTags, err := repo.Tags(ctx)
	require.NoError(t, err)
	assert.Len(t, remainingTags, 3)
	assert.NotContains(t, remainingTags, "feature-tag")

	// Try to delete non-existent tag
	err = repo.DeleteTag(ctx, "non-existent")
	assert.ErrorIs(t, err, ErrTagMissing)
}

// TestIntegrationReferences tests reference management
func TestIntegrationReferences(t *testing.T) {
	ctx := context.Background()

	// Setup origin and local repos
	originFS := billyfs.NewInMemoryFS()
	origin, err := Init(ctx, &Options{
		FS:      originFS,
		Bare:    true,
		Workdir: ".",
	})
	require.NoError(t, err)

	localFS := billyfs.NewInMemoryFS()
	local, err := Init(ctx, &Options{
		FS:      localFS,
		Workdir: ".",
	})
	require.NoError(t, err)

	_, err = local.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"file://."},
	})
	require.NoError(t, err)

	// Create initial commit to have a HEAD
	err = localFS.WriteFile("init.txt", []byte("initial"), 0o644)
	require.NoError(t, err)
	err = local.Add(ctx, "init.txt")
	require.NoError(t, err)
	_, err = local.Commit(ctx, "Initial commit", Signature{Name: "Test", Email: "test@example.com"}, CommitOpts{})
	require.NoError(t, err)

	// Get the initial branch name
	initialBranch, err := local.CurrentBranch(ctx)
	require.NoError(t, err)

	// Create branches and content
	branches := []string{initialBranch, "develop", "feature/a", "feature/b", "hotfix/urgent"}

	for _, branch := range branches {
		// Create branch
		if branch != initialBranch {
			err = local.CreateBranch(ctx, branch, "HEAD", false, false)
			require.NoError(t, err)
		}

		err = local.CheckoutBranch(ctx, branch, false, false)
		require.NoError(t, err)

		// Add unique content
		filename := fmt.Sprintf("%s.txt", branch)
		err = localFS.WriteFile(filename, []byte(branch), 0o644)
		require.NoError(t, err)

		err = local.Add(ctx, filename)
		require.NoError(t, err)

		_, err = local.Commit(ctx, fmt.Sprintf("Commit on %s", branch), Signature{
			Name:  "Test",
			Email: "test@example.com",
		}, CommitOpts{})
		require.NoError(t, err)

		// Push branch
		err = pushInMemory(ctx, local, origin, branch)
		require.NoError(t, err)
	}

	// Fetch all branches
	err = fetchInMemory(ctx, local, origin, false)
	require.NoError(t, err)

	// Test listing branches
	localBranches, err := local.Refs(ctx, RefBranch, "")
	require.NoError(t, err)
	assert.Len(t, localBranches, 5)

	// Test listing remote branches
	remoteBranches, err := local.Refs(ctx, RefRemoteBranch, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(remoteBranches), 5)

	// Test pattern matching for branches
	featureBranches, err := local.Refs(ctx, RefBranch, "feature/*")
	require.NoError(t, err)
	assert.Len(t, featureBranches, 2)
	assert.Contains(t, featureBranches, "refs/heads/feature/a")
	assert.Contains(t, featureBranches, "refs/heads/feature/b")

	// Create tags
	err = local.CreateTag(ctx, "v1.0", "HEAD", "Version 1.0", true)
	require.NoError(t, err)

	err = local.CreateTag(ctx, "v2.0", "HEAD", "Version 2.0", true)
	require.NoError(t, err)

	// Test listing tags as refs
	tagRefs, err := local.Refs(ctx, RefTag, "")
	require.NoError(t, err)
	assert.Len(t, tagRefs, 2)

	// Test resolving various ref types
	resolutions := []struct {
		ref      string
		expected RefKind
	}{
		{"HEAD", RefCommit},
		{initialBranch, RefBranch},
		{"origin/develop", RefRemoteBranch},
		{"v1.0", RefTag},
	}

	for _, res := range resolutions {
		resolved, err := local.Resolve(ctx, res.ref)
		require.NoError(t, err, "Failed to resolve %s", res.ref)
		assert.Equal(t, res.expected, resolved.Kind, "Wrong kind for %s", res.ref)
		assert.NotEmpty(t, resolved.Hash)
	}
}

// TestIntegrationDiff tests comprehensive diff operations
func TestIntegrationDiff(t *testing.T) {
	ctx := context.Background()

	fs := billyfs.NewInMemoryFS()
	repo, err := Init(ctx, &Options{
		FS:      fs,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Create initial files
	err = fs.MkdirAll("src", 0o755)
	require.NoError(t, err)

	files := map[string]string{
		"src/main.go":   "package main\n\nfunc main() {\n\tprintln(\"v1\")\n}\n",
		"src/helper.go": "package main\n\nfunc helper() string {\n\treturn \"help\"\n}\n",
		"README.md":     "# Project\nVersion 1\n",
	}

	for path, content := range files {
		err = fs.WriteFile(path, []byte(content), 0o644)
		require.NoError(t, err)
	}

	err = repo.Add(ctx, ".")
	require.NoError(t, err)

	commit1, err := repo.Commit(ctx, "Initial commit", Signature{
		Name:  "Test",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)

	// Make changes
	changes := map[string]string{
		"src/main.go": "package main\n\nfunc main() {\n\tprintln(\"v2\")\n\tprintln(\"with more output\")\n}\n",
		"src/new.go":  "package main\n\n// New file\nfunc newFunc() {}\n",
		"README.md":   "# Project\nVersion 2\n\nNow with more features!\n",
	}

	for path, content := range changes {
		err = fs.WriteFile(path, []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Remove a file
	err = fs.Remove("src/helper.go")
	require.NoError(t, err)

	err = repo.Add(ctx, ".")
	require.NoError(t, err)
	err = repo.Remove(ctx, "src/helper.go")
	require.NoError(t, err)

	_, err = repo.Commit(ctx, "Update project", Signature{
		Name:  "Test",
		Email: "test@example.com",
	}, CommitOpts{})
	require.NoError(t, err)

	// Test full diff
	diff, err := repo.Diff(ctx, commit1, "HEAD", nil)
	require.NoError(t, err)

	diffStr := diff.Text
	assert.Contains(t, diffStr, "-\tprintln(\"v1\")")
	assert.Contains(t, diffStr, "+\tprintln(\"v2\")")
	assert.Contains(t, diffStr, "+// New file")
	assert.Contains(t, diffStr, "-func helper()")
	assert.Contains(t, diffStr, "+Now with more features!")

	// Test diff with path filter
	diff, err = repo.Diff(ctx, "HEAD~1", "HEAD", ExtensionFilter(".md"))
	require.NoError(t, err)

	diffStr = diff.Text
	assert.Contains(t, diffStr, "README.md")
	assert.NotContains(t, diffStr, "main.go")
	assert.NotContains(t, diffStr, "helper.go")
}

// TestIntegrationErrorHandling tests error conditions and recovery
func TestIntegrationErrorHandling(t *testing.T) {
	ctx := context.Background()

	fs := billyfs.NewInMemoryFS()
	repo, err := Init(ctx, &Options{
		FS:      fs,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Test operations on empty repo
	t.Run("EmptyRepoOperations", func(t *testing.T) {
		// Cannot delete non-existent branch
		err = repo.DeleteBranch(ctx, "non-existent")
		assert.ErrorIs(t, err, ErrBranchMissing)

		// Cannot delete non-existent tag
		err = repo.DeleteTag(ctx, "non-existent")
		assert.ErrorIs(t, err, ErrTagMissing)

		// Cannot resolve invalid ref
		_, err = repo.Resolve(ctx, "invalid-ref")
		assert.Error(t, err)
	})

	// Create some content
	err = fs.WriteFile("test.txt", []byte("test"), 0o644)
	require.NoError(t, err)
	err = repo.Add(ctx, "test.txt")
	require.NoError(t, err)
	_, err = repo.Commit(ctx, "Initial", Signature{Name: "Test", Email: "test@example.com"}, CommitOpts{})
	require.NoError(t, err)

	t.Run("BranchOperations", func(t *testing.T) {
		// Create branch
		err = repo.CreateBranch(ctx, "test-branch", "HEAD", false, false)
		require.NoError(t, err)

		// Cannot create duplicate branch without force
		err = repo.CreateBranch(ctx, "test-branch", "HEAD", false, false)
		assert.ErrorIs(t, err, ErrBranchExists)

		// Can create duplicate with force
		err = repo.CreateBranch(ctx, "test-branch", "HEAD", false, true)
		assert.NoError(t, err)

		// Cannot checkout non-existent branch without create flag
		err = repo.CheckoutBranch(ctx, "non-existent", false, false)
		assert.Error(t, err)

		// Can checkout with create flag
		err = repo.CheckoutBranch(ctx, "new-branch", true, false)
		assert.NoError(t, err)

		// Cannot delete current branch
		err = repo.DeleteBranch(ctx, "new-branch")
		assert.Error(t, err)
	})

	t.Run("TagOperations", func(t *testing.T) {
		// Create tag
		err = repo.CreateTag(ctx, "test-tag", "HEAD", "Test tag", true)
		require.NoError(t, err)

		// Cannot create duplicate tag
		err = repo.CreateTag(ctx, "test-tag", "HEAD", "Duplicate", true)
		assert.ErrorIs(t, err, ErrTagExists)

		// Invalid target
		err = repo.CreateTag(ctx, "bad-tag", "invalid-target", "Bad", true)
		assert.Error(t, err)
	})

	t.Run("InvalidOperations", func(t *testing.T) {
		// Add non-existent file - should be silently ignored (matches git behavior)
		err = repo.Add(ctx, "non-existent.txt")
		assert.NoError(t, err)

		// Remove non-existent file - should be silently ignored (matches git rm behavior)
		err = repo.Remove(ctx, "non-existent.txt")
		assert.NoError(t, err)

		// Invalid diff revisions
		_, err = repo.Diff(ctx, "invalid1", "invalid2", nil)
		assert.Error(t, err)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		// Test context cancellation
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		// Operations should fail with context error
		err = repo.CreateBranch(cancelCtx, "cancelled-branch", "HEAD", false, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

// TestIntegrationConcurrentReads tests that read operations can be performed concurrently
func TestIntegrationConcurrentReads(t *testing.T) {
	ctx := context.Background()

	// Setup repository with test data
	repo := setupTestRepository(ctx, t)

	// Define concurrent operations
	operations := []func() error{
		func() error { return testConcurrentLog(ctx, repo) },
		func() error { return testConcurrentTags(ctx, repo) },
		func() error { return testConcurrentRefs(ctx, repo) },
		func() error { return testConcurrentBranch(ctx, repo) },
		func() error { return testConcurrentResolve(ctx, repo) },
	}

	// Run operations concurrently
	runConcurrentOperations(t, operations)
}

// setupTestRepository creates a repository with test data
func setupTestRepository(ctx context.Context, t *testing.T) *Repo {
	fs := billyfs.NewInMemoryFS()
	repo, err := Init(ctx, &Options{
		FS:      fs,
		Workdir: ".",
	})
	require.NoError(t, err)

	// Create some content and history
	for i := 0; i < 5; i++ {
		filename := fmt.Sprintf("file%d.txt", i)
		err = fs.WriteFile(filename, []byte(fmt.Sprintf("Content %d", i)), 0o644)
		require.NoError(t, err)

		err = repo.Add(ctx, filename)
		require.NoError(t, err)

		_, err = repo.Commit(ctx, fmt.Sprintf("Commit %d", i), Signature{
			Name:  "Test",
			Email: "test@example.com",
		}, CommitOpts{})
		require.NoError(t, err)

		err = repo.CreateTag(ctx, fmt.Sprintf("v%d.0", i), "HEAD", fmt.Sprintf("Version %d", i), true)
		require.NoError(t, err)
	}

	return repo
}

// runConcurrentOperations runs the given operations concurrently and waits for completion
func runConcurrentOperations(t *testing.T, operations []func() error) {
	errChan := make(chan error, len(operations))
	done := make(chan bool, len(operations))

	for _, op := range operations {
		go func(operation func() error) {
			if err := operation(); err != nil {
				errChan <- err
			} else {
				done <- true
			}
		}(op)
	}

	// Wait for all operations
	for i := 0; i < len(operations); i++ {
		select {
		case <-done:
			// Success
		case err := <-errChan:
			t.Fatalf("Concurrent operation failed: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}

func testConcurrentLog(ctx context.Context, repo *Repo) error {
	iter, err := repo.Log(ctx, LogFilter{MaxCount: 10})
	if err != nil {
		return err
	}
	var count int
	_ = iter.ForEach(func(_ *object.Commit) error {
		count++
		return nil
	})
	if count != 5 {
		return fmt.Errorf("expected 5 commits, got %d", count)
	}
	return nil
}

func testConcurrentTags(ctx context.Context, repo *Repo) error {
	tags, err := repo.Tags(ctx)
	if err != nil {
		return err
	}
	if len(tags) != 5 {
		return fmt.Errorf("expected 5 tags, got %d", len(tags))
	}
	return nil
}

func testConcurrentRefs(ctx context.Context, repo *Repo) error {
	refs, err := repo.Refs(ctx, RefTag, "")
	if err != nil {
		return err
	}
	if len(refs) != 5 {
		return fmt.Errorf("expected 5 tag refs, got %d", len(refs))
	}
	return nil
}

func testConcurrentBranch(ctx context.Context, repo *Repo) error {
	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return err
	}
	// Just check that we can get the branch, don't assume its name
	if branch == "" {
		return fmt.Errorf("expected a branch name, got empty string")
	}
	return nil
}

func testConcurrentResolve(ctx context.Context, repo *Repo) error {
	resolved, err := repo.Resolve(ctx, "v2.0")
	if err != nil {
		return err
	}
	if resolved.Kind != RefTag {
		return fmt.Errorf("expected tag kind, got %v", resolved.Kind)
	}
	return nil
}

// Helper functions for in-memory repository operations

func cloneInMemory(ctx context.Context, origin *Repo, opts *Options) (*Repo, error) {
	// Create new repo
	clone, err := Init(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Add origin remote
	_, err = clone.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"file://."},
	})
	if err != nil {
		return nil, err
	}

	// Fetch from origin
	err = fetchInMemory(ctx, clone, origin, false)
	if err != nil {
		return nil, err
	}

	// Checkout the first remote branch we find (typically master or main)
	refs, err := clone.Refs(ctx, RefRemoteBranch, "")
	if err != nil {
		return nil, err
	}

	// If there are remote branches, checkout the first one
	const originPrefix = "refs/remotes/origin/"
	if len(refs) == 0 {
		return clone, nil
	}
	for _, ref := range refs {
		if !strings.HasPrefix(ref, originPrefix) {
			continue
		}
		branchName := ref[len(originPrefix):]

		// Create local branch from remote
		remoteBranchRef := plumbing.NewRemoteReferenceName("origin", branchName)
		remoteRef, refErr := clone.repo.Reference(remoteBranchRef, true)
		if refErr != nil {
			continue
		}

		// Create the local branch
		localBranchRef := plumbing.NewBranchReferenceName(branchName)
		newRef := plumbing.NewHashReference(localBranchRef, remoteRef.Hash())
		if err := clone.repo.Storer.SetReference(newRef); err != nil {
			return nil, err
		}

		// Checkout the branch if we have a worktree
		if clone.worktree != nil {
			if err := clone.worktree.Checkout(&git.CheckoutOptions{Branch: localBranchRef, Force: true}); err != nil {
				if err := clone.worktree.Checkout(&git.CheckoutOptions{Hash: remoteRef.Hash(), Force: true}); err != nil {
					return nil, err
				}
			}
			if setErr := clone.repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, localBranchRef)); setErr != nil {
				return nil, setErr
			}
		}
		break
	}

	return clone, nil
}

// copyObjectsFromCommit recursively copies all objects reachable from a commit
func copyObjectsFromCommit(from, to *Repo, commit *object.Commit) error {
	// Copy the commit object itself
	obj := from.repo.Storer.NewEncodedObject()
	err := commit.Encode(obj)
	if err != nil {
		return err
	}

	_, err = to.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return err
	}

	// Copy the tree
	tree, err := commit.Tree()
	if err != nil {
		return err
	}

	err = copyTree(from, to, tree)
	if err != nil {
		return err
	}

	// Copy parent commits recursively
	parents := commit.Parents()
	err = parents.ForEach(func(parent *object.Commit) error {
		// Check if parent already exists in destination
		_, innerErr := to.repo.CommitObject(parent.Hash)
		if innerErr == nil {
			// Parent already exists, skip
			return nil
		}
		// Copy parent recursively
		return copyObjectsFromCommit(from, to, parent)
	})

	return err
}

// copyTree recursively copies a tree and all its contents
func copyTree(from, to *Repo, tree *object.Tree) error {
	// Copy the tree object itself
	obj := from.repo.Storer.NewEncodedObject()
	err := tree.Encode(obj)
	if err != nil {
		return err
	}

	_, err = to.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return err
	}

	// Copy all entries
	for _, entry := range tree.Entries {
		if entry.Mode.IsFile() {
			if err := copyBlobObject(from, to, entry.Hash); err != nil {
				return err
			}
			continue
		}
		if entry.Mode == 0o040000 {
			subtree, err := from.repo.TreeObject(entry.Hash)
			if err != nil {
				return err
			}
			if err := copyTree(from, to, subtree); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyBlobObject copies a blob identified by hash from one repo to another.
func copyBlobObject(from, to *Repo, hash plumbing.Hash) error {
	blob, err := from.repo.BlobObject(hash)
	if err != nil {
		return err
	}
	obj := from.repo.Storer.NewEncodedObject()
	err = blob.Encode(obj)
	if err != nil {
		return err
	}
	_, err = to.repo.Storer.SetEncodedObject(obj)
	return err
}

func pushInMemory(ctx context.Context, from, to *Repo, branch string) error {
	// Get references from source
	fromRef, err := from.repo.Reference(plumbing.NewBranchReferenceName(branch), false)
	if err != nil {
		return err
	}

	// Get the commit object
	commit, err := from.repo.CommitObject(fromRef.Hash())
	if err != nil {
		return err
	}

	// Copy all objects reachable from this commit
	err = copyObjectsFromCommit(from, to, commit)
	if err != nil {
		return err
	}

	// Create or update reference in destination
	toRef := plumbing.NewHashReference(
		plumbing.NewBranchReferenceName(branch),
		fromRef.Hash(),
	)

	// Store the reference in the destination
	return to.repo.Storer.SetReference(toRef)
}

func fetchInMemory(ctx context.Context, local, remote *Repo, prune bool) error {
	// Get all references from remote
	remoteRefs, err := remote.repo.References()
	if err != nil {
		return err
	}

	// Update remote references in local repo and copy objects
	return remoteRefs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsTag() || ref.Name().IsBranch() {
			// Get the commit for this ref
			commit, err := remote.repo.CommitObject(ref.Hash())
			if err == nil {
				// Copy all objects for this commit
				err = copyObjectsFromCommit(remote, local, commit)
				if err != nil {
					return err
				}
			}

			// Create the remote reference
			remoteName := plumbing.NewRemoteReferenceName("origin", ref.Name().Short())
			newRef := plumbing.NewHashReference(remoteName, ref.Hash())
			return local.repo.Storer.SetReference(newRef)
		}
		return nil
	})
}

func pullInMemory(ctx context.Context, local, origin *Repo) error {
	// Fetch latest
	err := fetchInMemory(ctx, local, origin, false)
	if err != nil {
		return err
	}

	// Get current branch
	currentBranch, err := local.CurrentBranch(ctx)
	if err != nil {
		return err
	}

	// Merge origin branch
	return mergeInMemory(ctx, local, fmt.Sprintf("origin/%s", currentBranch), FastForwardOnly)
}

func mergeInMemory(ctx context.Context, repo *Repo, ref string, strategy MergeStrategy) error {
	// For in-memory repos, we need to manually merge without fetching
	// since there's no real remote to fetch from

	// Resolve the ref to get the commit hash
	resolved, err := repo.Resolve(ctx, ref)
	if err != nil {
		return err
	}

	// Get the commit object
	hash := plumbing.NewHash(resolved.Hash)
	commit, err := repo.repo.CommitObject(hash)
	if err != nil {
		return err
	}

	// For fast-forward only, just update the current branch ref and checkout
	if repo.worktree != nil {
		// Get current branch
		currentBranch, err := repo.CurrentBranch(ctx)
		if err != nil {
			return err
		}

		// Update the branch ref to point to the merged commit
		branchRef := plumbing.NewBranchReferenceName(currentBranch)
		newRef := plumbing.NewHashReference(branchRef, commit.Hash)
		err = repo.repo.Storer.SetReference(newRef)
		if err != nil {
			return err
		}

		// Checkout to update working tree
		err = repo.worktree.Checkout(&git.CheckoutOptions{
			Hash:  commit.Hash,
			Force: true,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
