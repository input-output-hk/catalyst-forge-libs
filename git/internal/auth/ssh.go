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
//nolint:ireturn // go-git requires returning transport.AuthMethod interface
func (p *SSHAuthProvider) Method(remoteURL string) (transport.AuthMethod, error) {
	host, scheme, err := extractSSHHost(remoteURL)
	if err != nil {
		return nil, err
	}

	if !isSupportedScheme(scheme) {
		return nil, fmt.Errorf("SSH auth provider only supports SSH URLs, got %s", scheme)
	}

	// Check host restrictions if configured
	if len(p.AllowedHosts) > 0 && host != "" && !p.isHostAllowed(host) {
		return nil, nil // No auth for restricted hosts
	}

	if p.UseSSHAgent {
		return buildAgentAuth(p)
	}
	if p.PrivateKeyPath != "" {
		return buildFileAuth(p)
	}
	if len(p.PrivateKey) > 0 {
		return buildBytesAuth(p)
	}

	return nil, fmt.Errorf("no SSH credentials configured")
}

func extractSSHHost(remoteURL string) (string, string, error) {
	// Special handling for git@host:path style URLs
	if strings.HasPrefix(remoteURL, "git@") && !strings.HasPrefix(remoteURL, "git://") {
		parts := strings.SplitN(strings.TrimPrefix(remoteURL, "git@"), ":", 2)
		if len(parts) > 0 {
			return parts[0], "ssh", nil
		}
		return "", "", fmt.Errorf("invalid SSH URL: %s", remoteURL)
	}

	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}
	return parsedURL.Host, parsedURL.Scheme, nil
}

func isSupportedScheme(s string) bool {
	return s == "ssh" || s == "git" || s == "git+ssh"
}

//nolint:ireturn // go-git requires returning transport.AuthMethod interface
func buildAgentAuth(p *SSHAuthProvider) (transport.AuthMethod, error) {
	auth, err := ssh.NewSSHAgentAuth(p.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH agent auth: %w", err)
	}
	if p.HostKeyCallback != nil {
		auth.HostKeyCallback = p.HostKeyCallback
	}
	return auth, nil
}

//nolint:ireturn // go-git requires returning transport.AuthMethod interface
func buildFileAuth(p *SSHAuthProvider) (transport.AuthMethod, error) {
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

//nolint:ireturn // go-git requires returning transport.AuthMethod interface
func buildBytesAuth(p *SSHAuthProvider) (transport.AuthMethod, error) {
	auth, err := ssh.NewPublicKeys(p.Username, p.PrivateKey, p.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key from bytes: %w", err)
	}
	if p.HostKeyCallback != nil {
		auth.HostKeyCallback = p.HostKeyCallback
	}
	return auth, nil
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
