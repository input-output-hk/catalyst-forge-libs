// Package s3 provides client initialization and configuration.
//
// The Client provides a high-level interface for interacting with Amazon S3,
// supporting operations like upload, download, list, copy, and delete with
// configurable options for performance tuning and error handling.
package s3

import (
	"context"
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/input-output-hk/catalyst-forge-libs/fs"
	"github.com/input-output-hk/catalyst-forge-libs/fs/billy"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/s3api"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Client represents an S3 client with configurable options.
// It provides thread-safe access to S3 operations with built-in
// retry logic, concurrency control, and progress tracking.
type Client struct {
	// s3Client is the underlying AWS SDK S3 client
	s3Client s3api.S3API

	// rawClient holds the actual AWS S3 client for operations that need it
	rawClient *s3.Client

	// config holds the AWS configuration
	config aws.Config

	// mu protects concurrent access to client configuration
	mu sync.RWMutex

	// fs is the filesystem abstraction for file operations
	fs fs.Filesystem
}

// New creates a new S3 client with the provided options.
// It loads AWS credentials using the default credential chain
// and applies the specified configuration options.
//
// Example:
//
//	client, err := s3.New(
//	    s3.WithRegion("us-west-2"),
//	    s3.WithMaxRetries(3),
//	)
func New(opts ...s3types.Option) (*Client, error) {
	// Apply functional options first to check for custom config
	clientCfg := &s3types.ClientConfig{
		MaxRetries:     3,               // Default retry count
		Timeout:        0,               // No timeout by default
		Concurrency:    5,               // Default concurrency
		PartSize:       8 * 1024 * 1024, // 8MB default part size
		ForcePathStyle: false,
	}

	for _, opt := range opts {
		opt(clientCfg)
	}

	// Start with default AWS configuration or use custom config
	var cfg aws.Config
	var err error

	if clientCfg.CustomAWSConfig != nil {
		cfg = *clientCfg.CustomAWSConfig
	} else {
		cfg, err = config.LoadDefaultConfig(context.Background())
		if err != nil {
			return nil, errors.NewError("client initialization", err)
		}
	}

	// Apply region from options if specified, otherwise ensure a region is set
	if clientCfg.Region != "" {
		cfg.Region = clientCfg.Region
	} else if cfg.Region == "" {
		cfg.Region = "us-east-1" // AWS default region
	}

	if clientCfg.MaxRetries > 0 {
		cfg.RetryMaxAttempts = clientCfg.MaxRetries
	}

	// Create S3 client with options
	var s3Opts []func(*s3.Options)

	// Add path style option if needed
	if clientCfg.ForcePathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	// Handle custom HTTP client for timeout
	if clientCfg.Timeout > 0 {
		httpClient := &http.Client{
			Timeout: clientCfg.Timeout,
		}
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.HTTPClient = httpClient
		})
	}

	s3Client := s3.NewFromConfig(cfg, s3Opts...)

	// Initialize filesystem - use provided one or default to OS filesystem
	var filesystem fs.Filesystem
	if clientCfg.Filesystem != nil {
		filesystem = clientCfg.Filesystem
	} else {
		// Default to OS filesystem rooted at /
		filesystem = billy.NewOSFS("/")
	}

	client := &Client{
		s3Client:  s3Client,
		rawClient: s3Client,
		config:    cfg,
		fs:        filesystem,
	}

	return client, nil
}

// NewWithClient creates a new S3 client with a custom S3API implementation.
// This is primarily used for testing with mocked clients.
func NewWithClient(s3Client s3api.S3API) *Client {
	return &Client{
		s3Client: s3Client,
		config:   aws.Config{},
		fs:       billy.NewOSFS("/"), // Default to OS filesystem
	}
}

// SetFilesystem sets the filesystem implementation for the client.
// This is useful for testing or when the filesystem needs to be changed after creation.
func (c *Client) SetFilesystem(filesystem fs.Filesystem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fs = filesystem
}

// Close releases any resources held by the client.
// Currently a no-op but included for future extensibility.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Future: close any connection pools, cleanup resources
	return nil
}
