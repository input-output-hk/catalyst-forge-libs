// Package auth provides HTTPS authentication provider implementation.
package auth

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// HTTPSAuthProvider provides HTTPS authentication for git operations.
// It wraps go-git's http.BasicAuth with URL pattern matching.
type HTTPSAuthProvider struct {
	// The underlying go-git auth method
	auth *http.BasicAuth

	// AllowedHosts restricts authentication to specific host patterns.
	// If empty, authentication is allowed for all HTTPS URLs.
	// Supports glob patterns like "*.github.com" or "gitlab.*".
	AllowedHosts []string
}

// NewHTTPSAuthProvider creates a new HTTPS authentication provider.
// For GitHub/GitLab OAuth tokens, pass the token as the password.
func NewHTTPSAuthProvider(username, password string) *HTTPSAuthProvider {
	if username == "" && password != "" {
		// Many providers accept the token as username with empty password
		username = password
		password = ""
	}

	return &HTTPSAuthProvider{
		auth: &http.BasicAuth{
			Username: username,
			Password: password,
		},
	}
}

// NewHTTPSTokenProvider creates an HTTPS provider for token authentication.
// Most git providers (GitHub, GitLab, Bitbucket) use the token as password.
func NewHTTPSTokenProvider(token string) *HTTPSAuthProvider {
	return &HTTPSAuthProvider{
		auth: &http.BasicAuth{
			Username: "token", // Some providers need a username
			Password: token,
		},
	}
}

// WithAllowedHosts sets the allowed hosts for this provider.
// Only URLs matching these patterns will be authenticated.
func (p *HTTPSAuthProvider) WithAllowedHosts(hosts ...string) *HTTPSAuthProvider {
	p.AllowedHosts = hosts
	return p
}

// Method returns the authentication method for the given remote URL.
// Returns nil if the URL doesn't match allowed patterns.
//
//nolint:ireturn // go-git requires returning transport.AuthMethod interface
func (p *HTTPSAuthProvider) Method(remoteURL string) (transport.AuthMethod, error) {
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Only support HTTPS URLs
	if parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("HTTPS auth provider only supports https:// URLs, got %s", parsedURL.Scheme)
	}

	// Check host restrictions if configured
	if len(p.AllowedHosts) > 0 && !p.isHostAllowed(parsedURL.Host) {
		return nil, nil // No auth for restricted hosts
	}

	return p.auth, nil
}

// isHostAllowed checks if the given host matches any of the allowed host patterns.
func (p *HTTPSAuthProvider) isHostAllowed(host string) bool {
	for _, pattern := range p.AllowedHosts {
		if matchesPattern(host, pattern) {
			return true
		}
	}
	return false
}

// matchesPattern checks if a host matches a pattern with "*" wildcards.
func matchesPattern(host, pattern string) bool {
	// Exact match
	if host == pattern {
		return true
	}

	// Only support patterns with exactly one "*"
	if strings.Count(pattern, "*") != 1 {
		return false
	}

	// Handle wildcard patterns
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*.")
		return strings.HasSuffix(host, suffix) || host == suffix
	}

	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		// Also match "prefix.something"
		return strings.HasPrefix(host, prefix+".")
	}

	return false
}
