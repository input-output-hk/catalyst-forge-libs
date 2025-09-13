// Package ocibundle provides OCI bundle distribution functionality.
// This file contains functional options for configuration.
package ocibundle

import (
	"context"
	"time"

	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/input-output-hk/catalyst-forge-libs/oci/internal/oras"
)

// ClientOptions contains configuration options for the Client.
type ClientOptions struct {
	// Auth options for ORAS operations
	Auth *oras.AuthOptions

	// ORASClient allows injecting a custom ORAS client for testing
	// If nil, the default ORAS client will be used
	ORASClient oras.Client

	// HTTPConfig controls HTTP vs HTTPS and certificate validation
	HTTPConfig *HTTPConfig
}

// HTTPConfig contains configuration for HTTP transport settings.
// This allows explicit control over HTTP usage and certificate validation,
// rather than relying on brittle localhost detection.
type HTTPConfig struct {
	// AllowHTTP enables HTTP instead of HTTPS for registry connections.
	// This is useful for local registries that don't support HTTPS.
	AllowHTTP bool

	// AllowInsecure allows connections to registries with self-signed
	// or invalid certificates. This should only be used for testing.
	AllowInsecure bool

	// Registries specifies which registries this configuration applies to.
	// If empty, applies to all registries. Supports hostname matching.
	Registries []string
}

// ClientOption is a functional option for configuring the Client.
type ClientOption func(*ClientOptions)

// WithAuthNone configures the client to rely on ORAS's default Docker credential chain.
// This is the default behavior and uses ~/.docker/config.json and credential helpers
// like osxkeychain, pass, desktop, etc. as configured by the user.
func WithAuthNone() ClientOption {
	return func(opts *ClientOptions) {
		// Explicitly set to nil to ensure default behavior
		opts.Auth = nil
	}
}

// WithORASClient configures the client to use a custom ORAS client.
// This is primarily used for testing to inject mock implementations.
func WithORASClient(client oras.Client) ClientOption {
	return func(opts *ClientOptions) {
		opts.ORASClient = client
	}
}

// WithStaticAuth configures static credentials for a specific registry.
// This overrides the default Docker credential chain for the specified registry
// but allows other registries to use the default chain.
//
// Parameters:
//   - registry: The registry hostname (e.g., "ghcr.io")
//   - username: Username for authentication
//   - password: Password for authentication
//
// For other registries not matching the specified one, the default Docker
// credential chain will be used.
func WithStaticAuth(registry, username, password string) ClientOption {
	return func(opts *ClientOptions) {
		if opts.Auth == nil {
			opts.Auth = &oras.AuthOptions{}
		}
		opts.Auth.StaticRegistry = registry
		opts.Auth.StaticUsername = username
		opts.Auth.StaticPassword = password
	}
}

// WithCredentialFunc configures a custom credential callback function.
// This completely overrides the default Docker credential chain and provides
// full control over credential resolution for all registries.
//
// Parameters:
//   - fn: Function that returns credentials for a given registry.
//     Return empty credentials to fall back to anonymous access.
//     Return an error to fail authentication for that registry.
//
// The function should be safe for concurrent use and handle context cancellation.
func WithCredentialFunc(fn func(ctx context.Context, registry string) (auth.Credential, error)) ClientOption {
	return func(opts *ClientOptions) {
		if opts.Auth == nil {
			opts.Auth = &oras.AuthOptions{}
		}
		opts.Auth.CredentialFunc = fn
	}
}

// WithHTTP configures HTTP transport settings for registry connections.
// This allows explicit control over HTTP vs HTTPS usage and certificate validation.
//
// Parameters:
//   - allowHTTP: Enable HTTP instead of HTTPS for registry connections
//   - allowInsecure: Allow connections to registries with self-signed certificates
//   - registries: Specific registries to apply this config to (empty applies to all)
//
// Example usage:
//
//	client, err := New(WithHTTP(true, true, []string{"localhost:5000"}))
//
// This is preferred over automatic localhost detection for better control and testability.
func WithHTTP(allowHTTP, allowInsecure bool, registries []string) ClientOption {
	return func(opts *ClientOptions) {
		opts.HTTPConfig = &HTTPConfig{
			AllowHTTP:     allowHTTP,
			AllowInsecure: allowInsecure,
			Registries:    registries,
		}
	}
}

// WithAllowHTTP is a convenience function for enabling HTTP connections.
// This enables HTTP for all registries, useful for local development.
//
// Example usage:
//
//	client, err := New(WithAllowHTTP())
func WithAllowHTTP() ClientOption {
	return WithHTTP(true, false, nil)
}

// WithInsecureHTTP is a convenience function for enabling insecure HTTP connections.
// This enables both HTTP and allows self-signed certificates for all registries.
// WARNING: Only use this for testing environments.
//
// Example usage:
//
//	client, err := New(WithInsecureHTTP())
func WithInsecureHTTP() ClientOption {
	return WithHTTP(true, true, nil)
}

// PushOptions contains options for the Push operation.
type PushOptions struct {
	// Annotations to attach to the OCI artifact manifest
	Annotations map[string]string

	// Platform specifies the target platform for the artifact
	Platform string

	// ProgressCallback is called during push operations to report progress
	ProgressCallback func(current, total int64)

	// MaxRetries is the maximum number of retry attempts for network operations
	MaxRetries int

	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration
}

// PushOption is a functional option for configuring Push operations.
type PushOption func(*PushOptions)

// WithAnnotations sets annotations to be attached to the OCI artifact.
func WithAnnotations(annotations map[string]string) PushOption {
	return func(opts *PushOptions) {
		if opts.Annotations == nil {
			opts.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			opts.Annotations[k] = v
		}
	}
}

// WithPlatform sets the target platform for the OCI artifact.
func WithPlatform(platform string) PushOption {
	return func(opts *PushOptions) {
		opts.Platform = platform
	}
}

// WithProgressCallback sets a callback function for progress reporting.
func WithProgressCallback(callback func(current, total int64)) PushOption {
	return func(opts *PushOptions) {
		opts.ProgressCallback = callback
	}
}

// WithMaxRetries sets the maximum number of retry attempts for network operations.
func WithMaxRetries(maxRetries int) PushOption {
	return func(opts *PushOptions) {
		opts.MaxRetries = maxRetries
	}
}

// WithRetryDelay sets the delay between retry attempts.
func WithRetryDelay(delay time.Duration) PushOption {
	return func(opts *PushOptions) {
		opts.RetryDelay = delay
	}
}

// PullOptions contains options for the Pull operation.
type PullOptions struct {
	// MaxFiles is the maximum number of files allowed in the archive.
	// Set to 0 for unlimited (not recommended for security).
	MaxFiles int

	// MaxSize is the maximum total uncompressed size of all files combined.
	// Set to 0 for unlimited (not recommended for security).
	MaxSize int64

	// MaxFileSize is the maximum size allowed for any individual file.
	// Set to 0 for unlimited (not recommended for security).
	MaxFileSize int64

	// AllowHiddenFiles determines whether hidden files (starting with .) are allowed.
	AllowHiddenFiles bool

	// PreservePermissions determines whether to preserve original file permissions.
	// When false, permissions are sanitized for security.
	PreservePermissions bool

	// StripPrefix removes this prefix from all file paths during extraction.
	// Useful for removing leading directory names from archived paths.
	StripPrefix string

	// MaxRetries is the maximum number of retry attempts for network operations.
	MaxRetries int

	// RetryDelay is the delay between retry attempts.
	RetryDelay time.Duration
}

// PullOption is a functional option for configuring Pull operations.
type PullOption func(*PullOptions)

// WithPullMaxFiles sets the maximum number of files allowed in the archive.
func WithPullMaxFiles(maxFiles int) PullOption {
	return func(opts *PullOptions) {
		opts.MaxFiles = maxFiles
	}
}

// WithPullMaxSize sets the maximum total uncompressed size of all files combined.
func WithPullMaxSize(maxSize int64) PullOption {
	return func(opts *PullOptions) {
		opts.MaxSize = maxSize
	}
}

// WithPullMaxFileSize sets the maximum size allowed for any individual file.
func WithPullMaxFileSize(maxFileSize int64) PullOption {
	return func(opts *PullOptions) {
		opts.MaxFileSize = maxFileSize
	}
}

// WithPullAllowHiddenFiles determines whether hidden files are allowed.
func WithPullAllowHiddenFiles(allow bool) PullOption {
	return func(opts *PullOptions) {
		opts.AllowHiddenFiles = allow
	}
}

// WithPullPreservePermissions determines whether to preserve original file permissions.
func WithPullPreservePermissions(preserve bool) PullOption {
	return func(opts *PullOptions) {
		opts.PreservePermissions = preserve
	}
}

// WithPullStripPrefix sets the prefix to remove from all file paths during extraction.
func WithPullStripPrefix(prefix string) PullOption {
	return func(opts *PullOptions) {
		opts.StripPrefix = prefix
	}
}

// WithPullMaxRetries sets the maximum number of retry attempts for network operations.
func WithPullMaxRetries(maxRetries int) PullOption {
	return func(opts *PullOptions) {
		opts.MaxRetries = maxRetries
	}
}

// WithPullRetryDelay sets the delay between retry attempts.
func WithPullRetryDelay(delay time.Duration) PullOption {
	return func(opts *PullOptions) {
		opts.RetryDelay = delay
	}
}

// WithMaxFiles is an alias for WithPullMaxFiles for convenience.
func WithMaxFiles(maxFiles int) PullOption {
	return WithPullMaxFiles(maxFiles)
}

// WithMaxSize is an alias for WithPullMaxSize for convenience.
func WithMaxSize(maxSize int64) PullOption {
	return WithPullMaxSize(maxSize)
}

// DefaultPullOptions returns the default pull options.
func DefaultPullOptions() *PullOptions {
	return &PullOptions{
		MaxFiles:            10000,
		MaxSize:             1 * 1024 * 1024 * 1024, // 1GB
		MaxFileSize:         100 * 1024 * 1024,      // 100MB
		AllowHiddenFiles:    false,
		PreservePermissions: false,
		StripPrefix:         "",
		MaxRetries:          3,
		RetryDelay:          2 * time.Second,
	}
}

// DefaultPushOptions returns the default push options.
func DefaultPushOptions() *PushOptions {
	return &PushOptions{
		Annotations:      make(map[string]string),
		Platform:         "",
		ProgressCallback: nil,
		MaxRetries:       3,
		RetryDelay:       2 * time.Second,
	}
}

// DefaultClientOptions returns the default client options.
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		Auth:       nil, // Use default Docker credential chain
		HTTPConfig: nil, // Use default HTTPS with certificate validation
	}
}
