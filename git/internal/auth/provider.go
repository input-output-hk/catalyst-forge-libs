// Package auth provides authentication helpers for git operations.
// It provides pattern matching on top of go-git's existing auth methods.
package auth

import (
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// Provider interface that all auth providers must implement.
// Returns go-git's transport.AuthMethod directly.
type Provider interface {
	// Method returns the appropriate transport.AuthMethod for the given remote URL.
	// Returns nil if no authentication is needed/available for this URL.
	// Returns an error if authentication setup fails.
	Method(remoteURL string) (transport.AuthMethod, error)
}
