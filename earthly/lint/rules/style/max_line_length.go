// Package style provides style-related linting rules for Earthfiles.
// These rules enforce consistent formatting and naming conventions.
package style

import (
	"fmt"
	"strings"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/lint"
)

// DefaultMaxLineLength is the default maximum line length for the rule.
const DefaultMaxLineLength = 120

// MaxLineLengthRule enforces maximum line length in Earthfiles.
// Lines exceeding the configured maximum length will be flagged as issues.
type MaxLineLengthRule struct {
	maxLength int
}

// NewMaxLineLengthRule creates a new max line length rule.
// If maxLength is 0 or negative, it uses the default value of 120.
func NewMaxLineLengthRule(maxLength int) *MaxLineLengthRule {
	if maxLength <= 0 {
		maxLength = DefaultMaxLineLength
	}
	return &MaxLineLengthRule{
		maxLength: maxLength,
	}
}

// Name returns the unique identifier for this rule.
func (r *MaxLineLengthRule) Name() string {
	return "max-line-length"
}

// Description returns a human-readable description of what this rule checks.
func (r *MaxLineLengthRule) Description() string {
	if r.maxLength > 0 && r.maxLength != DefaultMaxLineLength {
		return fmt.Sprintf("Enforces maximum line length of %d characters (default: %d)",
			r.getEffectiveMaxLength(), DefaultMaxLineLength)
	}
	return fmt.Sprintf("Enforces maximum line length of %d characters", DefaultMaxLineLength)
}

// Check examines all commands in the Earthfile and reports issues for
// command lines that exceed the maximum allowed length.
func (r *MaxLineLengthRule) Check(ctx *lint.Context) []lint.Issue {
	var issues []lint.Issue

	maxLen := r.getEffectiveMaxLength()

	// Walk through all commands in the Earthfile
	err := ctx.WalkAll(func(walkCtx *lint.Context) error {
		if walkCtx.Command != nil {
			// Reconstruct the command line as it would appear in Earthfile
			commandLine := r.reconstructCommandLine(walkCtx.Command)

			if len(commandLine) > maxLen {
				issues = append(issues, lint.NewIssue(
					r.Name(),
					lint.SeverityInfo,
					fmt.Sprintf("Command line exceeds maximum length of %d characters (current: %d)",
						maxLen, len(commandLine)),
					walkCtx.Command.SourceLocation(),
				).WithContext("line_length", len(commandLine)).WithContext("command_line", commandLine))
			}
		}
		return nil
	})
	if err != nil {
		return nil
	}

	return issues
}

// reconstructCommandLine reconstructs the command line as it would appear in an Earthfile.
// This creates a reasonable approximation for line length checking.
func (r *MaxLineLengthRule) reconstructCommandLine(cmd *earthfile.Command) string {
	if cmd == nil {
		return ""
	}

	// Start with the command type
	line := strings.ToUpper(cmd.Type.String())

	// Add arguments with spaces
	for _, arg := range cmd.Args {
		line += " " + arg
	}

	return line
}

// getEffectiveMaxLength returns the effective maximum line length,
// using the default if not configured.
func (r *MaxLineLengthRule) getEffectiveMaxLength() int {
	if r.maxLength <= 0 {
		return DefaultMaxLineLength
	}
	return r.maxLength
}
