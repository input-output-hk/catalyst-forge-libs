package git_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/go-git/go-git/v5/plumbing/object"
	billyfs "github.com/input-output-hk/catalyst-forge-libs/fs/billy"

	"github.com/input-output-hk/catalyst-forge-libs/git"
)

// ExampleInit demonstrates how to initialize a new git repository.
func ExampleInit() {
	// Create filesystem
	fs := billyfs.NewInMemoryFS()

	// Initialize repository
	repo, err := git.Init(context.Background(), &git.Options{
		FS:      fs,
		Workdir: ".",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Create a file
	err = fs.WriteFile("README.md", []byte("# My Project\n"), 0o644)
	if err != nil {
		log.Fatal(err)
	}

	// Stage and commit
	err = repo.Add(context.Background(), "README.md")
	if err != nil {
		log.Fatal(err)
	}

	sha, err := repo.Commit(context.Background(), "Initial commit", git.Signature{
		Name:  "Example User",
		Email: "user@example.com",
	}, git.CommitOpts{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created commit: %s\n", sha[:7])
}

// ExampleOpen demonstrates how to open an existing repository.
func ExampleOpen() {
	// Open repository from filesystem
	fs := billyfs.NewOSFS("/path/to/repo")

	repo, err := git.Open(context.Background(), &git.Options{
		FS:      fs,
		Workdir: ".",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Get current branch
	branch, err := repo.CurrentBranch(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Current branch: %s\n", branch)
}

// ExampleClone demonstrates how to clone a remote repository.
func ExampleClone() {
	fs := billyfs.NewOSFS("/tmp/cloned-repo")

	// Clone repository (authentication can be provided via Auth field)
	repo, err := git.Clone(context.Background(), "https://github.com/example/repo.git", &git.Options{
		FS:           fs,
		Workdir:      ".",
		ShallowDepth: 1, // Shallow clone for faster operation
		// Auth: myAuthProvider, // Provide AuthProvider implementation
	})
	if err != nil {
		log.Fatal(err)
	}

	// List files in the cloned repo
	files, err := fs.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Cloned %d files\n", len(files))
	_ = repo // Use repo for further operations
}

// ExampleRepo_CreateBranch demonstrates branch creation and checkout.
func ExampleRepo_CreateBranch() {
	// Assume repo is already initialized
	fs := billyfs.NewInMemoryFS()
	repo, _ := git.Init(context.Background(), &git.Options{
		FS:      fs,
		Workdir: ".",
	})

	ctx := context.Background()

	// Create an initial commit so HEAD resolves
	_ = fs.WriteFile("init.txt", []byte("init"), 0o644)
	_ = repo.Add(ctx, "init.txt")
	_, _ = repo.Commit(ctx, "Initial", git.Signature{Name: "Example", Email: "ex@example.com"}, git.CommitOpts{})

	// Create a new branch from HEAD
	err := repo.CreateBranch(ctx, "feature/new-feature", "HEAD", false, false)
	if err != nil {
		log.Fatal(err)
	}

	// Checkout the new branch
	err = repo.CheckoutBranch(ctx, "feature/new-feature", false, false)
	if err != nil {
		log.Fatal(err)
	}

	// Verify current branch
	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Switched to branch: %s\n", branch)
	// Output: Switched to branch: feature/new-feature
}

// ExampleRepo_Commit demonstrates staging files and creating commits.
func ExampleRepo_Commit() {
	// Setup repository
	fs := billyfs.NewInMemoryFS()
	repo, _ := git.Init(context.Background(), &git.Options{
		FS:      fs,
		Workdir: ".",
	})

	ctx := context.Background()

	// Create some files
	_ = fs.WriteFile("main.go", []byte("package main\n"), 0o644)
	_ = fs.WriteFile("README.md", []byte("# Project\n"), 0o644)

	// Stage specific files
	err := repo.Add(ctx, "main.go", "README.md")
	if err != nil {
		log.Fatal(err)
	}

	// Create commit with signature
	sha, err := repo.Commit(ctx, "feat: initial implementation", git.Signature{
		Name:  "Jane Doe",
		Email: "jane@example.com",
	}, git.CommitOpts{
		// Options like AllowEmpty can be set here
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Commit created: %s\n", sha[:7])
}

// ExampleRepo_Log demonstrates querying commit history.
func ExampleRepo_Log() {
	// Assume repo with history exists
	fs := billyfs.NewInMemoryFS()
	repo, _ := git.Init(context.Background(), &git.Options{
		FS:      fs,
		Workdir: ".",
	})

	// Create some commits for the example
	for i := 0; i < 5; i++ {
		_ = fs.WriteFile(fmt.Sprintf("file%d.txt", i), []byte("content"), 0o644)
		_ = repo.Add(context.Background(), ".")
		_, _ = repo.Commit(context.Background(), fmt.Sprintf("Commit %d", i), git.Signature{
			Name:  "Test",
			Email: "test@example.com",
		}, git.CommitOpts{})
	}

	ctx := context.Background()

	// Query commit history with filters
	iter, err := repo.Log(ctx, git.LogFilter{
		MaxCount: 3,      // Limit to 3 commits
		Author:   "Test", // Filter by author
	})
	if err != nil {
		log.Fatal(err)
	}
	defer iter.Close()

	// Iterate through commits
	count := 0
	_ = iter.ForEach(func(c *object.Commit) error {
		fmt.Printf("%d. %s\n", count+1, c.Message)
		count++
		return nil
	})

	// Output:
	// 1. Commit 4
	// 2. Commit 3
	// 3. Commit 2
}

// ExampleRepo_Diff demonstrates computing diffs between revisions.
func ExampleRepo_Diff() {
	// Setup repository with changes
	fs := billyfs.NewInMemoryFS()
	repo, _ := git.Init(context.Background(), &git.Options{
		FS:      fs,
		Workdir: ".",
	})

	ctx := context.Background()

	// Initial commit
	_ = fs.WriteFile("file.txt", []byte("Line 1\n"), 0o644)
	_ = repo.Add(ctx, "file.txt")
	_, _ = repo.Commit(ctx, "Initial", git.Signature{
		Name:  "Test",
		Email: "test@example.com",
	}, git.CommitOpts{})

	// Make changes
	_ = fs.WriteFile("file.txt", []byte("Line 1\nLine 2\n"), 0o644)
	_ = repo.Add(ctx, "file.txt")
	_, _ = repo.Commit(ctx, "Add line", git.Signature{
		Name:  "Test",
		Email: "test@example.com",
	}, git.CommitOpts{})

	// Compute diff
	diff, err := repo.Diff(ctx, "HEAD~1", "HEAD", nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Files changed: %d\n", diff.FileCount)
	fmt.Printf("Contains binary: %v\n", diff.IsBinary)
	// The diff text contains the unified diff output
	// fmt.Println(diff.Text)
	// Output:
	// Files changed: 1
	// Contains binary: false
}

// ExampleRepo_Tags demonstrates tag management.
func ExampleRepo_Tags() {
	// Setup repository
	fs := billyfs.NewInMemoryFS()
	repo, _ := git.Init(context.Background(), &git.Options{
		FS:      fs,
		Workdir: ".",
	})

	ctx := context.Background()

	// Create initial commit
	_ = fs.WriteFile("version", []byte("1.0.0"), 0o644)
	_ = repo.Add(ctx, "version")
	sha, _ := repo.Commit(ctx, "Initial version", git.Signature{
		Name:  "Test",
		Email: "test@example.com",
	}, git.CommitOpts{})

	// Create annotated tag
	err := repo.CreateTag(ctx, "v1.0.0", sha, "Release version 1.0.0", true)
	if err != nil {
		log.Fatal(err)
	}

	// Create lightweight tag
	err = repo.CreateTag(ctx, "latest", "HEAD", "", false)
	if err != nil {
		log.Fatal(err)
	}

	// List all tags
	tags, err := repo.Tags(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, tag := range tags {
		fmt.Printf("Tag: %s\n", tag)
	}
	// Output:
	// Tag: latest
	// Tag: v1.0.0
}

// ExampleRepo_Fetch demonstrates fetching from a remote repository.
func ExampleRepo_Fetch() {
	// Assume repo with remote configured
	fs := billyfs.NewInMemoryFS()
	repo, _ := git.Init(context.Background(), &git.Options{
		FS:      fs,
		Workdir: ".",
	})

	ctx := context.Background()

	// Fetch from remote with pruning
	err := repo.Fetch(ctx, "origin", true, 0)
	if err != nil {
		if errors.Is(err, git.ErrAlreadyUpToDate) {
			fmt.Println("Already up to date")
		} else {
			log.Fatal(err)
		}
	}

	// Fetch with shallow depth for large repos
	err = repo.Fetch(ctx, "origin", false, 10)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Fetch completed")
}

// ExampleRepo_Push demonstrates pushing changes to a remote.
func ExampleRepo_Push() {
	// Assume repo with commits and remote configured
	fs := billyfs.NewInMemoryFS()
	repo, _ := git.Init(context.Background(), &git.Options{
		FS:      fs,
		Workdir: ".",
		// Auth: myAuthProvider, // Provide AuthProvider implementation
	})

	ctx := context.Background()

	// Push current branch to origin
	err := repo.Push(ctx, "origin", false)
	if err != nil {
		switch {
		case errors.Is(err, git.ErrNotFastForward):
			fmt.Println("Push rejected: not a fast-forward")
			// Could try with force: repo.Push(ctx, "origin", true)
		case errors.Is(err, git.ErrAuthRequired):
			fmt.Println("Authentication required")
		default:
			log.Fatal(err)
		}
	}

	fmt.Println("Push successful")
}
