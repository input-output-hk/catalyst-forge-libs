// Package lint provides a flexible, rule-based linting framework for Earthfiles.
// It enables developers to enforce coding standards, security policies, and best practices
// through composable linting rules.
package lint

// Rule defines the interface that all linting rules must implement.
// Rules are the core building blocks of the linting framework, each responsible
// for detecting specific patterns or issues in Earthfiles.
type Rule interface {
	// Name returns a unique identifier for the rule.
	// This should be a kebab-case string like "no-sudo" or "require-version".
	Name() string

	// Description returns a human-readable description of what the rule checks.
	// This should explain the purpose and intent of the rule.
	Description() string

	// Check examines the provided Context and returns any issues found.
	// The context provides access to the Earthfile, current target/command,
	// and hierarchical navigation capabilities.
	Check(ctx *Context) []Issue
}
