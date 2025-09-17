// Package auth provides unit tests for SSH authentication provider.
package auth

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func TestSSHAuthProvider_Constructors(t *testing.T) {
	t.Run("NewSSHKeyProvider", func(t *testing.T) {
		provider := NewSSHKeyProvider("/path/to/key", "passphrase")
		assert.Equal(t, "/path/to/key", provider.PrivateKeyPath)
		assert.Equal(t, "passphrase", provider.Passphrase)
		assert.Equal(t, "git", provider.Username)
	})

	t.Run("NewSSHKeyBytesProvider", func(t *testing.T) {
		keyBytes := []byte("fake-key")
		provider := NewSSHKeyBytesProvider(keyBytes, "passphrase")
		assert.Equal(t, keyBytes, provider.PrivateKey)
		assert.Equal(t, "passphrase", provider.Passphrase)
		assert.Equal(t, "git", provider.Username)
	})

	t.Run("NewSSHAgentProvider", func(t *testing.T) {
		provider := NewSSHAgentProvider()
		assert.True(t, provider.UseSSHAgent)
		assert.Equal(t, "git", provider.Username)
	})
}

func TestSSHAuthProvider_Builder(t *testing.T) {
	provider := NewSSHKeyProvider("/key", "").
		WithUsername("myuser").
		WithHostKeyCallback(gossh.InsecureIgnoreHostKey()).
		WithAllowedHosts("github.com", "gitlab.com")

	assert.Equal(t, "myuser", provider.Username)
	assert.NotNil(t, provider.HostKeyCallback)
	assert.Equal(t, []string{"github.com", "gitlab.com"}, provider.AllowedHosts)
}

func TestSSHAuthProvider_Method(t *testing.T) {
	tests := []struct {
		name      string
		provider  *SSHAuthProvider
		remoteURL string
		setup     func()
		cleanup   func()
		wantAuth  bool
		wantError bool
	}{
		{
			name:      "SSH URL with agent",
			provider:  NewSSHAgentProvider(),
			remoteURL: "ssh://git@github.com/user/repo.git",
			wantAuth:  true,
			wantError: false,
		},
		{
			name:      "git URL with agent",
			provider:  NewSSHAgentProvider(),
			remoteURL: "git://github.com/user/repo.git",
			wantAuth:  true,
			wantError: false,
		},
		{
			name:      "git@ URL with agent",
			provider:  NewSSHAgentProvider(),
			remoteURL: "git@github.com:user/repo.git",
			wantAuth:  true,
			wantError: false,
		},
		{
			name:      "HTTPS URL returns error",
			provider:  NewSSHAgentProvider(),
			remoteURL: "https://github.com/user/repo.git",
			wantAuth:  false,
			wantError: true,
		},
		{
			name:      "no credentials configured",
			provider:  &SSHAuthProvider{Username: "git"},
			remoteURL: "ssh://git@github.com/user/repo.git",
			wantAuth:  false,
			wantError: true,
		},
		{
			name:      "host not allowed returns nil",
			provider:  NewSSHAgentProvider().WithAllowedHosts("gitlab.com"),
			remoteURL: "ssh://git@github.com/user/repo.git",
			wantAuth:  false,
			wantError: false,
		},
		{
			name:      "host allowed returns auth",
			provider:  NewSSHAgentProvider().WithAllowedHosts("github.com"),
			remoteURL: "git@github.com:user/repo.git",
			wantAuth:  true,
			wantError: false,
		},
		{
			name:      "key file does not exist",
			provider:  NewSSHKeyProvider("/nonexistent/key", ""),
			remoteURL: "ssh://git@github.com/user/repo.git",
			wantAuth:  false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			// For SSH agent tests, check if SSH_AUTH_SOCK is set
			if tt.provider.UseSSHAgent && os.Getenv("SSH_AUTH_SOCK") == "" {
				t.Skip("SSH agent not available")
			}

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
