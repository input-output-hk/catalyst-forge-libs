// Package secrets provides secure, provider-agnostic secrets management
// with automatic memory cleanup and just-in-time resolution.
package core

import (
	"context"
	"time"
)

// contextKey is a typed key for context values to avoid string key issues
type contextKey string

const (
	userIDKey    contextKey = "user_id"
	requestIDKey contextKey = "request_id"
	sourceIPKey  contextKey = "source_ip"
	otherKey     contextKey = "other_value"
)

// AuditLogger defines the interface for audit logging of secret access events.
// Implementations should be thread-safe and handle logging failures gracefully.
type AuditLogger interface {
	// LogAccess logs a secret access attempt with context information.
	// ctx contains request context (user ID, request ID, etc.)
	// action describes the operation (e.g., "resolve", "store", "delete")
	// ref identifies the secret being accessed
	// success indicates whether the operation succeeded
	// err contains the error if the operation failed (nil if successful)
	LogAccess(ctx context.Context, action string, ref SecretRef, success bool, err error)
}

// AuditEntry represents a structured audit log entry for secret access.
// It contains all relevant information about a secret access event.
type AuditEntry struct {
	// Timestamp when the audit event occurred
	Timestamp time.Time

	// Action performed (e.g., "resolve", "store", "delete", "rotate")
	Action string

	// SecretRef contains the path and version of the accessed secret
	SecretRef SecretRef

	// Success indicates whether the operation was successful
	Success bool

	// Error contains error details if the operation failed
	Error string

	// Context values extracted from the request context
	// These may include user ID, request ID, IP address, etc.
	Context map[string]string
}

// NewAuditEntry creates a new AuditEntry with the current timestamp and provided values.
func NewAuditEntry(
	ctx context.Context,
	action string,
	ref SecretRef,
	success bool,
	err error,
) *AuditEntry {
	entry := &AuditEntry{
		Timestamp: time.Now(),
		Action:    action,
		SecretRef: ref,
		Success:   success,
		Context:   make(map[string]string),
	}

	// Set error message if provided
	if err != nil {
		entry.Error = err.Error()
	}

	// Extract context values if available
	if ctx != nil {
		// Extract common context values (these can be extended based on application needs)
		if userID, ok := ctx.Value(userIDKey).(string); ok {
			entry.Context["user_id"] = userID
		}
		if requestID, ok := ctx.Value(requestIDKey).(string); ok {
			entry.Context["request_id"] = requestID
		}
		if sourceIP, ok := ctx.Value(sourceIPKey).(string); ok {
			entry.Context["source_ip"] = sourceIP
		}
	}

	return entry
}
