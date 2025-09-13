// Package ocibundle provides OCI bundle distribution functionality.
// This file contains the main client interface and implementation.
package ocibundle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"oras.land/oras-go/v2/registry/remote"

	"github.com/input-output-hk/catalyst-forge-libs/oci/internal/oras"
)

// Client provides OCI bundle operations using ORAS for registry communication.
// The client is safe for concurrent use and isolates ORAS dependencies in internal packages.
type Client struct {
	// options contains the client configuration
	options *ClientOptions

	// orasClient provides ORAS operations (injected for testability)
	orasClient oras.Client

	// mu protects concurrent access to client operations
	mu sync.RWMutex
}

// New creates a new Client with default configuration.
// It uses ORAS's default Docker credential chain for authentication.
func New() (*Client, error) {
	return NewWithOptions()
}

// NewWithOptions creates a new Client with custom configuration.
// It accepts functional options to customize authentication and other behaviors.
//
// Example usage:
//
//	client, err := NewWithOptions(
//	    WithStaticAuth("ghcr.io", "username", "password"),
//	)
//	if err != nil {
//	    return err
//	}
func NewWithOptions(opts ...ClientOption) (*Client, error) {
	options := DefaultClientOptions()

	// Apply functional options
	for _, opt := range opts {
		opt(options)
	}

	// Use provided ORAS client or default to real implementation
	orasClient := options.ORASClient
	if orasClient == nil {
		orasClient = &oras.DefaultORASClient{}
	}

	// Convert public HTTPConfig to internal AuthOptions format
	if options.HTTPConfig != nil {
		if options.Auth == nil {
			options.Auth = &oras.AuthOptions{}
		}
		options.Auth.HTTPConfig = &oras.HTTPConfig{
			AllowHTTP:     options.HTTPConfig.AllowHTTP,
			AllowInsecure: options.HTTPConfig.AllowInsecure,
			Registries:    options.HTTPConfig.Registries,
		}
	}

	client := &Client{
		options:    options,
		orasClient: orasClient,
	}

	// Validate options
	if err := validateClientOptions(options); err != nil {
		return nil, fmt.Errorf("invalid client options: %w", err)
	}

	return client, nil
}

// validateClientOptions validates the client options for correctness.
// It checks for invalid combinations and missing required values.
//
// Parameters:
//   - opts: The client options to validate
//
// Returns an error if validation fails, nil if options are valid.
func validateClientOptions(opts *ClientOptions) error {
	if opts == nil {
		return fmt.Errorf("client options cannot be nil")
	}

	// Validate authentication options if present
	if opts.Auth == nil {
		return nil
	}
	// If static auth is specified, both username and password must be provided
	if opts.Auth.StaticRegistry == "" {
		return nil
	}
	if opts.Auth.StaticUsername == "" {
		return fmt.Errorf("static username required when static registry is specified")
	}
	if opts.Auth.StaticPassword == "" {
		return fmt.Errorf("static password required when static registry is specified")
	}

	return nil
}

// createRepository creates an ORAS repository with authentication configured.
// This is an internal method that applies the client's auth options to repository creation.
//
// Parameters:
//   - ctx: Context for the operation
//   - reference: Full OCI reference (e.g., "ghcr.io/org/repo:tag")
//
// Returns:
//   - Configured ORAS repository ready for operations
//   - Error if repository creation fails
func (c *Client) createRepository(ctx context.Context, reference string) (*remote.Repository, error) {
	// Note: mutex is already held by caller, so we don't need to lock here
	repo, err := oras.NewRepository(ctx, reference, c.options.Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository for %s: %w", reference, err)
	}
	return repo, nil
}

// retryOperation retries a function with exponential backoff for network-related errors
func retryOperation(ctx context.Context, maxRetries int, delay time.Duration, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry operation: %w", ctx.Err())
		default:
		}

		if attempt > 0 {
			// Exponential backoff
			backoffDelay := delay * time.Duration(1<<(attempt-1))
			time.Sleep(backoffDelay)
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Only retry on network-related errors
		if !isRetryableError(err) {
			break
		}
	}

	return lastErr
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	// Network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Connection errors
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Registry-specific temporary errors (5xx status codes)
	errStr := err.Error()
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		// Check for common temporary error patterns
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "service unavailable") ||
		strings.Contains(errStr, "internal server error")
}

// Push uploads a directory as an OCI artifact to the specified reference.
// It archives the source directory and pushes it to the OCI registry with the given options.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - sourceDir: Path to directory to upload (must exist and be readable)
//   - reference: OCI reference (e.g., "ghcr.io/org/repo:tag")
//   - opts: Optional push options for annotations, platform, and progress reporting
//
// Returns:
//   - Error if the operation fails
func (c *Client) Push(ctx context.Context, sourceDir, reference string, opts ...PushOption) error {
	// Thread safety: use read lock since we're only reading options
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Parse push options
	pushOpts := DefaultPushOptions()
	for _, opt := range opts {
		opt(pushOpts)
	}

	// Validate inputs
	if sourceDir == "" {
		return fmt.Errorf("source directory cannot be empty")
	}

	if reference == "" {
		return fmt.Errorf("reference cannot be empty")
	}

	// Check if source directory exists and is readable
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", sourceDir)
	}

	// Create authenticated repository (needed for future authentication validation)
	_, err := c.createRepository(ctx, reference)
	if err != nil {
		return err
	}

	// Create temporary file for the archive
	tempFile, err := os.CreateTemp("", "ocibundle-push-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempFilePath := tempFile.Name()

	// Ensure cleanup happens in all error paths
	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			tempFile.Close()
			os.Remove(tempFilePath)
		}
	}()

	// Create archiver
	archiver := NewTarGzArchiver()

	// Archive the source directory (with progress if callback provided)
	if pushOpts.ProgressCallback != nil {
		err = archiver.ArchiveWithProgress(ctx, sourceDir, tempFile, pushOpts.ProgressCallback)
	} else {
		err = archiver.Archive(ctx, sourceDir, tempFile)
	}
	if err != nil {
		return fmt.Errorf("failed to archive directory: %w", err)
	}

	// Close the file so we can read it for pushing
	if closeErr := tempFile.Close(); closeErr != nil {
		return fmt.Errorf("failed to close temporary file: %w", closeErr)
	}

	// Reopen for reading
	tempFile, err = os.Open(tempFilePath)
	if err != nil {
		return fmt.Errorf("failed to reopen temporary file: %w", err)
	}

	// Update cleanup to only close the file (don't remove yet since we're using it)
	defer func() {
		tempFile.Close()
		if cleanupNeeded {
			os.Remove(tempFilePath)
		}
	}()

	// Get file size
	stat, err := tempFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file size: %w", err)
	}

	// Create push descriptor with options
	desc := &oras.PushDescriptor{
		MediaType:   archiver.MediaType(),
		Data:        tempFile,
		Size:        stat.Size(),
		Annotations: pushOpts.Annotations,
		Platform:    pushOpts.Platform,
	}

	// Push the artifact with retry logic
	pushErr := retryOperation(ctx, pushOpts.MaxRetries, pushOpts.RetryDelay, func() error {
		return c.orasClient.Push(ctx, reference, desc, c.options.Auth)
	})
	if pushErr != nil {
		return fmt.Errorf("failed to push artifact after %d retries: %w", pushOpts.MaxRetries, pushErr)
	}

	// Success - no cleanup needed
	cleanupNeeded = false
	return nil
}

// Pull downloads and extracts an OCI artifact to the specified directory.
// It downloads the artifact from the OCI registry and extracts it with security validation.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - reference: OCI reference to download (e.g., "ghcr.io/org/repo:tag")
//   - targetDir: Directory to extract the artifact to (created if it doesn't exist)
//   - opts: Optional pull options for security limits and behavior
//
// Returns:
//   - Error if the operation fails
func (c *Client) Pull(ctx context.Context, reference, targetDir string, opts ...PullOption) error {
	// Thread safety: use read lock since we're only reading options
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Parse pull options
	pullOpts := DefaultPullOptions()
	for _, opt := range opts {
		opt(pullOpts)
	}

	// Validate inputs
	if reference == "" {
		return fmt.Errorf("reference cannot be empty")
	}

	if targetDir == "" {
		return fmt.Errorf("target directory cannot be empty")
	}

	// Check if target directory exists and is empty (for atomic extraction)
	if _, statErr := os.Stat(targetDir); statErr == nil {
		// Directory exists, check if it's empty
		entries, readErr := os.ReadDir(targetDir)
		if readErr != nil {
			return fmt.Errorf("failed to read target directory: %w", readErr)
		}
		if len(entries) > 0 {
			return fmt.Errorf("target directory is not empty: %s", targetDir)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("failed to check target directory: %w", statErr)
	}

	// Create authenticated repository (needed for future authentication validation)
	_, err := c.createRepository(ctx, reference)
	if err != nil {
		return err
	}

	// Pull the artifact with retry logic
	var descriptor *oras.PullDescriptor
	pullErr := retryOperation(ctx, pullOpts.MaxRetries, pullOpts.RetryDelay, func() error {
		var err error
		descriptor, err = c.orasClient.Pull(ctx, reference, c.options.Auth)
		if err != nil {
			return fmt.Errorf("failed to pull OCI artifact %s: %w", reference, err)
		}
		return nil
	})
	if pullErr != nil {
		return fmt.Errorf("failed to pull artifact after %d retries: %w", pullOpts.MaxRetries, pullErr)
	}

	// Ensure we close the descriptor data when done
	defer descriptor.Data.Close()

	// Create archiver
	archiver := NewTarGzArchiver()

	// Create extract options from pull options
	extractOpts := ExtractOptions{
		MaxFiles:      pullOpts.MaxFiles,
		MaxSize:       pullOpts.MaxSize,
		MaxFileSize:   pullOpts.MaxFileSize,
		StripPrefix:   pullOpts.StripPrefix,
		PreservePerms: pullOpts.PreservePermissions,
	}

	// Extract the archive atomically (all or nothing)
	if err := c.extractAtomically(ctx, archiver, descriptor.Data, targetDir, extractOpts); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	return nil
}

// extractAtomically performs atomic extraction with rollback on failure
func (c *Client) extractAtomically(
	ctx context.Context,
	archiver *TarGzArchiver,
	data io.Reader,
	targetDir string,
	opts ExtractOptions,
) error {
	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "ocibundle-pull-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Extract to temporary directory first
	if err := archiver.Extract(ctx, data, tempDir, opts); err != nil {
		return fmt.Errorf("extraction to temporary directory failed: %w", err)
	}

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Move extracted files from temp directory to target directory
	if err := c.moveFiles(tempDir, targetDir); err != nil {
		// Clean up any partially moved files
		os.RemoveAll(targetDir)
		return fmt.Errorf("failed to move extracted files: %w", err)
	}

	return nil
}

// moveFiles moves all files from srcDir to dstDir
func (c *Client) moveFiles(srcDir, dstDir string) error {
	if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk path %s: %w", path, err)
		}

		// Skip the root directory
		if path == srcDir {
			return nil
		}

		// Calculate relative path from source
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path from %s to %s: %w", srcDir, path, err)
		}

		// Calculate destination path
		dstPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			// Create directory
			if err := os.MkdirAll(dstPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dstPath, err)
			}
			return nil
		}
		// Move file
		if err := os.Rename(path, dstPath); err != nil {
			return fmt.Errorf("failed to move file from %s to %s: %w", path, dstPath, err)
		}
		// Restore original permissions
		if err := os.Chmod(dstPath, info.Mode()); err != nil {
			return fmt.Errorf("failed to set permissions on %s: %w", dstPath, err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to walk directory %s: %w", srcDir, err)
	}
	return nil
}
