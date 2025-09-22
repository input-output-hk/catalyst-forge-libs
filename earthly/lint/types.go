// Package lint provides a flexible, rule-based linting framework for Earthfiles.
// It enables developers to enforce coding standards, security policies, and best practices
// through composable linting rules.
package lint

import (
	"fmt"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile"
)

// Severity represents the severity level of a linting issue.
type Severity int

const (
	// SeverityError indicates a critical issue that should block builds.
	SeverityError Severity = iota
	// SeverityWarning indicates a potential issue that should be addressed.
	SeverityWarning
	// SeverityInfo indicates a suggestion or style improvement.
	SeverityInfo
)

// String returns the string representation of the severity level.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

// SourceLocation represents a position in the source file.
// This reuses the SourceLocation from the earthfile package.
type SourceLocation = earthfile.SourceLocation

// Fix represents an automatic fix that can be applied to resolve an issue.
type Fix struct {
	// Description explains what the fix does.
	Description string
	// Before contains the original text that will be replaced.
	Before string
	// After contains the replacement text.
	After string
	// Location specifies where in the file the fix should be applied.
	Location *SourceLocation
}

// Issue represents a single linting issue found in an Earthfile.
type Issue struct {
	// Rule is the identifier of the rule that found this issue.
	Rule string
	// Severity indicates the importance level of the issue.
	Severity Severity
	// Message is a human-readable description of the issue.
	Message string
	// Location specifies where in the source file the issue occurs.
	Location *SourceLocation
	// Fix contains an optional automatic fix for the issue.
	Fix *Fix
	// Context provides additional metadata about the issue.
	Context map[string]interface{}
}

// String returns a formatted string representation of the issue.
func (i Issue) String() string {
	if i.Location != nil {
		return fmt.Sprintf("%s:%d:%d [%s] %s",
			i.Location.File,
			i.Location.StartLine,
			i.Location.StartColumn,
			i.Rule,
			i.Message)
	}
	return fmt.Sprintf("[%s] %s", i.Rule, i.Message)
}

// IsValid checks if the issue has all required fields.
func (i Issue) IsValid() bool {
	return i.Rule != "" && i.Message != ""
}

// NewIssue creates a new Issue with the given parameters.
func NewIssue(rule string, severity Severity, message string, location *SourceLocation) Issue {
	return Issue{
		Rule:     rule,
		Severity: severity,
		Message:  message,
		Location: location,
		Context:  make(map[string]interface{}),
	}
}

// WithFix adds a fix to an issue and returns the modified issue.
func (i Issue) WithFix(description, before, after string, location *SourceLocation) Issue {
	i.Fix = &Fix{
		Description: description,
		Before:      before,
		After:       after,
		Location:    location,
	}
	return i
}

// WithContext adds context metadata to an issue and returns the modified issue.
func (i Issue) WithContext(key string, value interface{}) Issue {
	if i.Context == nil {
		i.Context = make(map[string]interface{})
	}
	i.Context[key] = value
	return i
}
