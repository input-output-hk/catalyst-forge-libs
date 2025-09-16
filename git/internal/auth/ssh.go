// Package auth provides SSH authentication provider implementation.
package auth

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// SSHAuthProvider provides SSH authentication for git operations.
// It wraps go-git's SSH auth methods with URL pattern matching.
type SSHAuthProvider struct {
	// PrivateKeyPath is the path to the SSH private key file.
	PrivateKeyPath string

	// PrivateKey contains the SSH private key as bytes.
	PrivateKey []byte

	// Passphrase for encrypted private keys.
	Passphrase string

	// Username for SSH authentication (defaults to "git").
	Username string

	// UseSSHAgent enables SSH agent integration.
	UseSSHAgent bool

	// HostKeyCallback for host key verification (optional).
	// If nil, defaults to accepting any host key (insecure).
	HostKeyCallback gossh.HostKeyCallback

	// AllowedHosts restricts authentication to specific host patterns.
	// If empty, authentication is allowed for all SSH URLs.
	// Supports glob patterns like "*.github.com" or "gitlab.*".
	AllowedHosts []string
}

// NewSSHKeyProvider creates an SSH provider using a private key file.
func NewSSHKeyProvider(keyPath, passphrase string) *SSHAuthProvider {
	return &SSHAuthProvider{
		PrivateKeyPath: keyPath,
		Passphrase:     passphrase,
		Username:       "git",
	}
}

// NewSSHKeyBytesProvider creates an SSH provider using private key bytes.
func NewSSHKeyBytesProvider(keyBytes []byte, passphrase string) *SSHAuthProvider {
	return &SSHAuthProvider{
		PrivateKey: keyBytes,
		Passphrase: passphrase,
		Username:   "git",
	}
}

// NewSSHAgentProvider creates an SSH provider that uses SSH agent.
func NewSSHAgentProvider() *SSHAuthProvider {
	return &SSHAuthProvider{
		UseSSHAgent: true,
		Username:    "git",
	}
}

// WithUsername sets the SSH username (default is "git").
func (p *SSHAuthProvider) WithUsername(username string) *SSHAuthProvider {
	p.Username = username
	return p
}

// WithHostKeyCallback sets the host key verification callback.
func (p *SSHAuthProvider) WithHostKeyCallback(callback gossh.HostKeyCallback) *SSHAuthProvider {
	p.HostKeyCallback = callback
	return p
}

// WithAllowedHosts sets the allowed hosts for this provider.
func (p *SSHAuthProvider) WithAllowedHosts(hosts ...string) *SSHAuthProvider {
	p.AllowedHosts = hosts
	return p
}

// Method returns the authentication method for the given remote URL.
// Returns nil if the URL doesn't match allowed patterns.
//
//nolint:ireturn,cyclop // transport.AuthMethod is an interface required by go-git, complexity needed for URL parsing
func (p *SSHAuthProvider) Method(remoteURL string) (transport.AuthMethod, error) {
	// Special handling for git@host:path style URLs
	var host string
	if strings.HasPrefix(remoteURL, "git@") && !strings.HasPrefix(remoteURL, "git://") {
		// Parse git@host:path style URLs manually
		parts := strings.SplitN(strings.TrimPrefix(remoteURL, "git@"), ":", 2)
		if len(parts) > 0 {
			host = parts[0]
		}
	} else {
		// Standard URL parsing
		parsedURL, err := url.Parse(remoteURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		host = parsedURL.Host

		// Check scheme
		if parsedURL.Scheme != "ssh" && parsedURL.Scheme != "git" && parsedURL.Scheme != "git+ssh" {
			return nil, fmt.Errorf("SSH auth provider only supports SSH URLs, got %s", parsedURL.Scheme)
		}
	}

	// Check host restrictions if configured
	if len(p.AllowedHosts) > 0 && host != "" && !p.isHostAllowed(host) {
		return nil, nil // No auth for restricted hosts
	}

	// Use SSH agent if requested
	if p.UseSSHAgent {
		auth, err := ssh.NewSSHAgentAuth(p.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH agent auth: %w", err)
		}
		if p.HostKeyCallback != nil {
			auth.HostKeyCallback = p.HostKeyCallback
		}
		return auth, nil
	}

	// Use key file if provided
	if p.PrivateKeyPath != "" {
		// Validate file exists
		if _, err := os.Stat(p.PrivateKeyPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("SSH private key file does not exist: %s", p.PrivateKeyPath)
		}

		auth, err := ssh.NewPublicKeysFromFile(p.Username, p.PrivateKeyPath, p.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to load SSH key from file: %w", err)
		}
		if p.HostKeyCallback != nil {
			auth.HostKeyCallback = p.HostKeyCallback
		}
		return auth, nil
	}

	// Use key bytes if provided
	if len(p.PrivateKey) > 0 {
		auth, err := ssh.NewPublicKeys(p.Username, p.PrivateKey, p.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to load SSH key from bytes: %w", err)
		}
		if p.HostKeyCallback != nil {
			auth.HostKeyCallback = p.HostKeyCallback
		}
		return auth, nil
	}

	return nil, fmt.Errorf("no SSH credentials configured")
}

// isHostAllowed checks if the given host matches any of the allowed host patterns.
func (p *SSHAuthProvider) isHostAllowed(host string) bool {
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	for _, pattern := range p.AllowedHosts {
		if matchesPattern(host, pattern) {
			return true
		}
	}
	return false
}
