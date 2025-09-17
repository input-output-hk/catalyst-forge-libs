// Package auth provides unit tests for composite authentication provider.
package auth

import (
	"fmt"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider is a test implementation of Provider
type mockProvider struct {
	auth      transport.AuthMethod
	shouldErr bool
	errMsg    string
	called    bool
}

//nolint:ireturn // test mock returns interface as required by Provider
func (m *mockProvider) Method(remoteURL string) (transport.AuthMethod, error) {
	m.called = true
	if m.shouldErr {
		return nil, fmt.Errorf("%s", m.errMsg)
	}
	return m.auth, nil
}

func TestCompositeAuthProvider_Basic(t *testing.T) {
	t.Run("no providers configured", func(t *testing.T) {
		comp := NewCompositeAuthProvider()
		auth, err := comp.Method("https://github.com/user/repo.git")
		assert.Error(t, err)
		assert.Nil(t, auth)
		assert.Contains(t, err.Error(), "no authentication providers configured")
	})

	t.Run("first provider succeeds", func(t *testing.T) {
		expectedAuth := &http.BasicAuth{Username: "user", Password: "pass"}
		provider1 := &mockProvider{auth: expectedAuth}
		provider2 := &mockProvider{auth: &http.BasicAuth{Username: "other", Password: "other"}}

		comp := NewCompositeAuthProvider().
			AddProvider(provider1).
			AddProvider(provider2)

		auth, err := comp.Method("https://github.com/user/repo.git")
		require.NoError(t, err)
		assert.Equal(t, expectedAuth, auth)
		assert.True(t, provider1.called)
		assert.False(t, provider2.called) // Should not be called
	})

	t.Run("first provider returns nil, second succeeds", func(t *testing.T) {
		expectedAuth := &http.BasicAuth{Username: "user", Password: "pass"}
		provider1 := &mockProvider{auth: nil} // Returns nil (no auth for this URL)
		provider2 := &mockProvider{auth: expectedAuth}

		comp := NewCompositeAuthProvider().
			AddProvider(provider1).
			AddProvider(provider2)

		auth, err := comp.Method("https://github.com/user/repo.git")
		require.NoError(t, err)
		assert.Equal(t, expectedAuth, auth)
		assert.True(t, provider1.called)
		assert.True(t, provider2.called)
	})

	t.Run("all providers return nil", func(t *testing.T) {
		provider1 := &mockProvider{auth: nil}
		provider2 := &mockProvider{auth: nil}

		comp := NewCompositeAuthProvider().
			AddProvider(provider1).
			AddProvider(provider2)

		auth, err := comp.Method("https://github.com/user/repo.git")
		require.NoError(t, err)
		assert.Nil(t, auth)
	})
}

func TestCompositeAuthProvider_ErrorHandling(t *testing.T) {
	t.Run("continue on error", func(t *testing.T) {
		expectedAuth := &http.BasicAuth{Username: "user", Password: "pass"}
		provider1 := &mockProvider{shouldErr: true, errMsg: "auth failed"}
		provider2 := &mockProvider{auth: expectedAuth}

		comp := NewCompositeAuthProvider().
			SetContinueOnError(true).
			AddProvider(provider1).
			AddProvider(provider2)

		auth, err := comp.Method("https://github.com/user/repo.git")
		require.NoError(t, err) // Error is swallowed when second provider succeeds
		assert.Equal(t, expectedAuth, auth)
		assert.True(t, provider1.called)
		assert.True(t, provider2.called)
	})

	t.Run("stop on error", func(t *testing.T) {
		provider1 := &mockProvider{shouldErr: true, errMsg: "auth failed"}
		provider2 := &mockProvider{auth: &http.BasicAuth{Username: "user", Password: "pass"}}

		comp := NewCompositeAuthProvider().
			SetContinueOnError(false).
			AddProvider(provider1).
			AddProvider(provider2)

		auth, err := comp.Method("https://github.com/user/repo.git")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth failed")
		assert.Nil(t, auth)
		assert.True(t, provider1.called)
		assert.False(t, provider2.called) // Should not be called
	})
}

func TestCompositeAuthProvider_URLPatterns(t *testing.T) {
	t.Run("provider with matching pattern", func(t *testing.T) {
		expectedAuth := &http.BasicAuth{Username: "user", Password: "pass"}
		githubProvider := &mockProvider{auth: expectedAuth}
		gitlabProvider := &mockProvider{auth: &http.BasicAuth{Username: "other", Password: "other"}}

		comp := NewCompositeAuthProvider().
			AddProvider(githubProvider, "https://*.github.com").
			AddProvider(gitlabProvider, "https://*.gitlab.com")

		auth, err := comp.Method("https://github.com/user/repo.git")
		require.NoError(t, err)
		assert.Equal(t, expectedAuth, auth)
		assert.True(t, githubProvider.called)
		assert.False(t, gitlabProvider.called)
	})

	t.Run("no matching pattern", func(t *testing.T) {
		githubProvider := &mockProvider{auth: &http.BasicAuth{Username: "user", Password: "pass"}}

		comp := NewCompositeAuthProvider().
			AddProvider(githubProvider, "https://*.github.com")

		auth, err := comp.Method("https://bitbucket.org/user/repo.git")
		require.NoError(t, err)
		assert.Nil(t, auth)
		assert.False(t, githubProvider.called) // Should not be called
	})
}
