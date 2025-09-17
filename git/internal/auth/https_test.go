// Package auth provides unit tests for HTTPS authentication provider.
package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPSAuthProvider_NewProviders(t *testing.T) {
	t.Run("NewHTTPSAuthProvider", func(t *testing.T) {
		provider := NewHTTPSAuthProvider("user", "pass")
		assert.NotNil(t, provider)
		assert.NotNil(t, provider.auth)
	})

	t.Run("NewHTTPSTokenProvider", func(t *testing.T) {
		provider := NewHTTPSTokenProvider("token123")
		assert.NotNil(t, provider)
		assert.NotNil(t, provider.auth)
		assert.Equal(t, "token", provider.auth.Username)
		assert.Equal(t, "token123", provider.auth.Password)
	})
}

func TestHTTPSAuthProvider_Method(t *testing.T) {
	tests := []struct {
		name      string
		provider  *HTTPSAuthProvider
		remoteURL string
		wantAuth  bool
		wantError bool
	}{
		{
			name:      "HTTPS URL returns auth",
			provider:  NewHTTPSAuthProvider("user", "pass"),
			remoteURL: "https://github.com/user/repo.git",
			wantAuth:  true,
			wantError: false,
		},
		{
			name:      "SSH URL returns error",
			provider:  NewHTTPSAuthProvider("user", "pass"),
			remoteURL: "ssh://git@github.com/user/repo.git",
			wantAuth:  false,
			wantError: true,
		},
		{
			name:      "allowed host matches",
			provider:  NewHTTPSAuthProvider("user", "pass").WithAllowedHosts("github.com"),
			remoteURL: "https://github.com/user/repo.git",
			wantAuth:  true,
			wantError: false,
		},
		{
			name:      "host not allowed returns nil",
			provider:  NewHTTPSAuthProvider("user", "pass").WithAllowedHosts("gitlab.com"),
			remoteURL: "https://github.com/user/repo.git",
			wantAuth:  false,
			wantError: false,
		},
		{
			name:      "invalid URL",
			provider:  NewHTTPSAuthProvider("user", "pass"),
			remoteURL: "://invalid-url",
			wantAuth:  false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := tt.provider.Method(tt.remoteURL)

			if tt.wantError {
				require.Error(t, err)
				assert.Nil(t, auth)
			} else {
				require.NoError(t, err)
				if tt.wantAuth {
					assert.NotNil(t, auth)
				} else {
					assert.Nil(t, auth)
				}
			}
		})
	}
}

func TestHTTPSAuthProvider_matchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		pattern  string
		expected bool
	}{
		{
			name:     "exact match",
			host:     "github.com",
			pattern:  "github.com",
			expected: true,
		},
		{
			name:     "wildcard prefix match",
			host:     "myorg.github.com",
			pattern:  "*.github.com",
			expected: true,
		},
		{
			name:     "wildcard suffix match",
			host:     "gitlab.example.com",
			pattern:  "gitlab.*",
			expected: true, // suffix patterns work with .*
		},
		{
			name:     "no match",
			host:     "github.com",
			pattern:  "gitlab.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesPattern(tt.host, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}
