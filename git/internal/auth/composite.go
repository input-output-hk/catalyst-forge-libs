// Package auth provides composite authentication provider implementation.
package auth

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

// ProviderConfig configures a provider with URL pattern matching.
type ProviderConfig struct {
	// Provider is the authentication provider to use.
	Provider Provider

	// URLPatterns are URL patterns this provider should handle.
	// Supports glob patterns like "https://*.github.com" or "ssh://gitlab.*".
	// If empty, this provider will be tried for all URLs.
	URLPatterns []string
}

// CompositeAuthProvider combines multiple authentication providers with fallback support.
// It tries providers in order until one successfully provides authentication for a URL.
type CompositeAuthProvider struct {
	// Providers is the ordered list of providers to try.
	Providers []ProviderConfig

	// ContinueOnError determines whether to continue trying other providers
	// if a provider returns an error, or stop immediately.
	ContinueOnError bool
}

// NewCompositeAuthProvider creates a new composite authentication provider.
func NewCompositeAuthProvider() *CompositeAuthProvider {
	return &CompositeAuthProvider{
		ContinueOnError: true, // Default to continuing on errors
	}
}

// AddProvider adds a provider to the fallback chain.
// URLPatterns can be used to restrict this provider to specific URL patterns.
func (c *CompositeAuthProvider) AddProvider(provider Provider, urlPatterns ...string) *CompositeAuthProvider {
	c.Providers = append(c.Providers, ProviderConfig{
		Provider:    provider,
		URLPatterns: urlPatterns,
	})
	return c
}

// SetContinueOnError configures error handling strategy.
func (c *CompositeAuthProvider) SetContinueOnError(continueOnError bool) *CompositeAuthProvider {
	c.ContinueOnError = continueOnError
	return c
}

// Method returns the appropriate authentication method for the given remote URL.
// It tries each configured provider in order until one provides authentication.
//
//nolint:ireturn // transport.AuthMethod is an interface required by go-git
func (c *CompositeAuthProvider) Method(remoteURL string) (transport.AuthMethod, error) {
	if len(c.Providers) == 0 {
		return nil, fmt.Errorf("no authentication providers configured")
	}

	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	var lastError error

	for i, config := range c.Providers {
		// Check if this provider should handle this URL
		if !c.shouldTryProvider(parsedURL, config.URLPatterns) {
			continue
		}

		// Try this provider
		method, err := config.Provider.Method(remoteURL)
		if err != nil {
			lastError = fmt.Errorf("provider %d failed: %w", i, err)
			if !c.ContinueOnError {
				return nil, lastError
			}
			continue
		}

		// If method is not nil, we found authentication
		if method != nil {
			return method, nil
		}
		// nil method means provider declined this URL, try next
	}

	// If we have an error and didn't find any auth, return the error
	if lastError != nil {
		return nil, lastError
	}

	// No provider could authenticate this URL
	return nil, nil
}

// shouldTryProvider checks if a provider should be tried for the given URL.
func (c *CompositeAuthProvider) shouldTryProvider(parsedURL *url.URL, patterns []string) bool {
	// No patterns means this provider handles all URLs
	if len(patterns) == 0 {
		return true
	}

	// Check if URL matches any pattern
	for _, pattern := range patterns {
		if c.matchesURLPattern(parsedURL, pattern) {
			return true
		}
	}
	return false
}

// matchesURLPattern checks if a URL matches a pattern.
func (c *CompositeAuthProvider) matchesURLPattern(parsedURL *url.URL, pattern string) bool {
	patternURL, err := url.Parse(pattern)
	if err != nil {
		// Simple string contains as fallback
		return strings.Contains(parsedURL.String(), pattern)
	}

	// Check scheme if specified in pattern
	if patternURL.Scheme != "" && patternURL.Scheme != parsedURL.Scheme {
		return false
	}

	// Check host with wildcard support
	if patternURL.Host != "" {
		if !matchesPattern(parsedURL.Host, patternURL.Host) {
			return false
		}
	}

	return true
}
