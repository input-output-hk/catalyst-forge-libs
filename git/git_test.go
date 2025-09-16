package git

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	billyfs "github.com/input-output-hk/catalyst-forge-libs/fs/billy"

	"github.com/input-output-hk/catalyst-forge-libs/git/internal/fsbridge"
)

func TestInit_InMemory_NonBare(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	opts := Options{
		FS:      memFS,
		Bare:    false,
		Workdir: ".",
	}

	repo, err := Init(ctx, &opts)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	if repo == nil {
		t.Fatal("Init() returned nil repo")
	}

	if repo.repo == nil {
		t.Error("repo.repo should not be nil")
	}

	if repo.worktree == nil {
		t.Error("repo.worktree should not be nil for non-bare repo")
	}

	if repo.fs != memFS {
		t.Error("repo.fs should match the provided filesystem")
	}
}

func TestInit_InMemory_Bare(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	opts := Options{
		FS:   memFS,
		Bare: true,
	}

	repo, err := Init(ctx, &opts)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	if repo == nil {
		t.Fatal("Init() returned nil repo")
	}

	if repo.repo == nil {
		t.Error("repo.repo should not be nil")
	}

	if repo.worktree != nil {
		t.Error("repo.worktree should be nil for bare repo")
	}

	if repo.fs != memFS {
		t.Error("repo.fs should match the provided filesystem")
	}
}

func TestInit_InMemory_VerifyGitDirectoryStructure(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	opts := Options{
		FS:      memFS,
		Bare:    false,
		Workdir: ".",
	}

	_, err := Init(ctx, &opts)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Check using the raw filesystem directly
	rawFS := memFS.Raw()

	// List all files in the root to see what was created
	files, err := rawFS.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read root directory: %v", err)
	}

	t.Logf("Files in root directory:")
	for _, file := range files {
		t.Logf("  %s", file.Name())
	}

	// For non-bare repos with same filesystem for storage and worktree,
	// go-git creates files directly in root (bare-style layout)
	// Check that basic git files exist in root
	expectedFiles := []string{
		"HEAD",
		"config",
		"refs",
		"objects",
	}

	for _, file := range expectedFiles {
		_, err := rawFS.Stat(file)
		if err != nil {
			t.Errorf("Expected file %s to exist: %v", file, err)
		}
	}
}

func TestInit_InMemory_Bare_VerifyGitDirectoryStructure(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	opts := Options{
		FS:   memFS,
		Bare: true,
	}

	_, err := Init(ctx, &opts)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Check that basic git files exist in root (bare repo)
	expectedFiles := []string{
		"HEAD",
		"config",
		"refs",
		"objects",
	}

	for _, file := range expectedFiles {
		_, err := memFS.Stat(file)
		if err != nil {
			t.Errorf("Expected file %s to exist in bare repo: %v", file, err)
		}
	}
}

func TestInit_InvalidOptions(t *testing.T) {
	ctx := context.Background()

	opts := Options{
		FS: nil, // Invalid: nil filesystem
	}

	_, err := Init(ctx, &opts)
	if err == nil {
		t.Error("Init() should fail with invalid options")
	}
}

func TestInit_DefaultOptions(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	// Test with minimal options - should use defaults
	opts := Options{
		FS: memFS,
	}

	repo, err := Init(ctx, &opts)
	if err != nil {
		t.Fatalf("Init() with default options failed: %v", err)
	}

	if repo == nil {
		t.Fatal("Init() returned nil repo")
	}

	// Verify defaults were applied
	if repo.options.Workdir != DefaultWorkdir {
		t.Errorf("Expected Workdir to be %q, got %q", DefaultWorkdir, repo.options.Workdir)
	}

	if repo.options.StorerCacheSize != DefaultStorerCacheSize {
		t.Errorf("Expected StorerCacheSize to be %d, got %d", DefaultStorerCacheSize, repo.options.StorerCacheSize)
	}
}

func TestGoGitDirect_WithMemoryFS(t *testing.T) {
	// Test if go-git works directly with go-billy memfs
	memFS := billyfs.NewInMemoryFS()
	rawFS := memFS.Raw()

	storage := fsbridge.NewStorage(rawFS, 1000)
	repo, err := git.Init(storage, rawFS)
	if err != nil {
		t.Fatalf("Direct go-git Init failed: %v", err)
	}

	if repo == nil {
		t.Fatal("Direct go-git Init returned nil repo")
	}

	t.Log("Direct go-git Init with memory filesystem succeeded")
}

func TestOpen_InMemory_NonBare(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	// First create a repository
	initOpts := Options{
		FS:      memFS,
		Bare:    false,
		Workdir: ".",
	}

	_, err := Init(ctx, &initOpts)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Now try to open it
	openOpts := Options{
		FS:      memFS,
		Bare:    false,
		Workdir: ".",
	}

	repo, err := Open(ctx, &openOpts)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	if repo == nil {
		t.Fatal("Open() returned nil repo")
	}

	if repo.repo == nil {
		t.Error("repo.repo should not be nil")
	}

	if repo.worktree == nil {
		t.Error("repo.worktree should not be nil for non-bare repo")
	}

	if repo.fs != memFS {
		t.Error("repo.fs should match the provided filesystem")
	}
}

func TestOpen_InMemory_Bare(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	// First create a bare repository
	initOpts := Options{
		FS:   memFS,
		Bare: true,
	}

	_, err := Init(ctx, &initOpts)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Now try to open it
	openOpts := Options{
		FS:   memFS,
		Bare: true,
	}

	repo, err := Open(ctx, &openOpts)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	if repo == nil {
		t.Fatal("Open() returned nil repo")
	}

	if repo.repo == nil {
		t.Error("repo.repo should not be nil")
	}

	if repo.worktree != nil {
		t.Error("repo.worktree should be nil for bare repo")
	}

	if repo.fs != memFS {
		t.Error("repo.fs should match the provided filesystem")
	}
}

func TestOpen_NonExistentRepository(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	// Try to open a repository that doesn't exist
	opts := Options{
		FS:      memFS,
		Bare:    false,
		Workdir: ".",
	}

	_, err := Open(ctx, &opts)
	if err == nil {
		t.Fatal("Open() should fail for non-existent repository")
	}
}

func TestOpen_InvalidOptions(t *testing.T) {
	ctx := context.Background()

	opts := Options{
		FS: nil, // Invalid: nil filesystem
	}

	_, err := Open(ctx, &opts)
	if err == nil {
		t.Fatal("Open() should fail with invalid options")
	}
}

func TestClone_EmptyURL(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	opts := Options{
		FS: memFS,
	}

	_, err := Clone(ctx, "", &opts)
	if err == nil {
		t.Fatal("Clone() should fail with empty URL")
	}
}

func TestClone_InvalidOptions(t *testing.T) {
	ctx := context.Background()

	opts := Options{
		FS: nil, // Invalid: nil filesystem
	}

	_, err := Clone(ctx, "https://github.com/user/repo.git", &opts)
	if err == nil {
		t.Fatal("Clone() should fail with invalid options")
	}
}

func TestClone_WithMockAuthProvider(t *testing.T) {
	ctx := context.Background()
	memFS := billyfs.NewInMemoryFS()

	// Create a mock auth provider that returns valid auth
	mockAuth := &cloneMockAuthProvider{
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
	if err == nil {
		t.Fatal("Clone() should fail for non-existent repository")
	}

	// Check that our auth provider was called
	if !mockAuth.called {
		t.Error("Auth provider should have been called")
	}
}

// cloneMockAuthProvider is a test implementation of AuthProvider for clone tests
type cloneMockAuthProvider struct {
	auth   transport.AuthMethod
	called bool
}

//nolint:ireturn // transport.AuthMethod is an interface required by go-git
func (m *cloneMockAuthProvider) Method(remoteURL string) (transport.AuthMethod, error) {
	m.called = true
	return m.auth, nil
}
