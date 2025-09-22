// Package lint provides a flexible, rule-based linting framework for Earthfiles.
// It enables developers to enforce coding standards, security policies, and best practices
// through composable linting rules.
package lint

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile"
)

// CheckFunc represents a function that performs rule checking on a context.
// It returns a slice of issues found, or nil if no issues were detected.
type CheckFunc func(ctx *Context) []Issue

// CommandCheckFunc represents a function that checks a specific command.
// It returns a slice of issues found for that command, or nil if no issues were detected.
type CommandCheckFunc func(ctx *Context, cmd *earthfile.Command) []Issue

// RequirementFunc represents a function that checks if a requirement is satisfied.
// It returns true if the requirement is met, false otherwise.
type RequirementFunc func(ctx *Context) bool

// ForbiddenFunc represents a function that checks if something forbidden exists.
// It returns true if the forbidden pattern is found, false otherwise.
type ForbiddenFunc func(ctx *Context) bool

// SimpleRule creates a rule that uses a simple check function.
// This is the most basic rule builder for rules that need full access to the context.
//
//nolint:ireturn // Builder functions should return interfaces
func SimpleRule(name, description string, check CheckFunc) Rule {
	return &simpleRule{
		name:        name,
		description: description,
		check:       check,
	}
}

// simpleRule implements the Rule interface using a CheckFunc.
type simpleRule struct {
	name        string
	description string
	check       CheckFunc
}

// Name returns the unique identifier for this rule.
func (r *simpleRule) Name() string {
	return r.name
}

// Description returns a human-readable description of what this rule checks.
func (r *simpleRule) Description() string {
	return r.description
}

// Check executes the rule's check function and returns any issues found.
func (r *simpleRule) Check(ctx *Context) []Issue {
	return r.check(ctx)
}

// CommandRule creates a rule that checks specific command types.
// The rule will only be applied to commands matching the specified command type.
//
//nolint:ireturn // Builder functions should return interfaces
func CommandRule(name, description string, cmdType earthfile.CommandType, check CommandCheckFunc) Rule {
	return &commandRule{
		name:        name,
		description: description,
		cmdType:     cmdType,
		check:       check,
	}
}

// commandRule implements the Rule interface for command-specific rules.
type commandRule struct {
	name        string
	description string
	cmdType     earthfile.CommandType
	check       CommandCheckFunc
}

// Name returns the unique identifier for this rule.
func (r *commandRule) Name() string {
	return r.name
}

// Description returns a human-readable description of what this rule checks.
func (r *commandRule) Description() string {
	return r.description
}

// Check examines all commands of the specified type and applies the check function.
func (r *commandRule) Check(ctx *Context) []Issue {
	var issues []Issue

	// Walk through all contexts and check commands
	_ = ctx.WalkAll(func(walkCtx *Context) error {
		if walkCtx.Command != nil && walkCtx.Command.Type == r.cmdType {
			if cmdIssues := r.check(walkCtx, walkCtx.Command); cmdIssues != nil {
				issues = append(issues, cmdIssues...)
			}
		}
		return nil
	})

	return issues
}

// PatternRule creates a rule that detects patterns using regular expressions.
// This is useful for detecting hardcoded secrets, forbidden syntax, etc.
//
//nolint:ireturn // Builder functions should return interfaces
func PatternRule(name, description, pattern string, severity Severity) Rule {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		panic(fmt.Sprintf("invalid pattern in rule %s: %v", name, err))
	}

	return &patternRule{
		name:        name,
		description: description,
		pattern:     regex,
		severity:    severity,
	}
}

// patternRule implements the Rule interface for regex-based pattern detection.
type patternRule struct {
	name        string
	description string
	pattern     *regexp.Regexp
	severity    Severity
}

// Name returns the unique identifier for this rule.
func (r *patternRule) Name() string {
	return r.name
}

// Description returns a human-readable description of what this rule checks.
func (r *patternRule) Description() string {
	return r.description
}

// Check searches for the pattern in all commands and returns issues for matches.
func (r *patternRule) Check(ctx *Context) []Issue {
	var issues []Issue

	_ = ctx.WalkAll(func(walkCtx *Context) error {
		if walkCtx.Command != nil {
			// Check command arguments
			for _, arg := range walkCtx.Command.Args {
				if r.pattern.MatchString(arg) {
					issues = append(issues, NewIssue(
						r.name,
						r.severity,
						fmt.Sprintf("Found forbidden pattern: %s", r.pattern.String()),
						walkCtx.Command.SourceLocation(),
					))
				}
			}
		}
		return nil
	})

	return issues
}

// RequireRule creates a rule that ensures a requirement is satisfied.
// The rule returns an error if the requirement function returns false.
//
//nolint:ireturn // Builder functions should return interfaces
func RequireRule(name, description string, requirement RequirementFunc) Rule {
	return &requireRule{
		name:        name,
		description: description,
		requirement: requirement,
	}
}

// requireRule implements the Rule interface for requirement validation.
type requireRule struct {
	name        string
	description string
	requirement RequirementFunc
}

// Name returns the unique identifier for this rule.
func (r *requireRule) Name() string {
	return r.name
}

// Description returns a human-readable description of what this rule checks.
func (r *requireRule) Description() string {
	return r.description
}

// Check validates the requirement and returns an error issue if not satisfied.
func (r *requireRule) Check(ctx *Context) []Issue {
	if !r.requirement(ctx) {
		return []Issue{NewIssue(
			r.name,
			SeverityError,
			r.description,
			nil, // No specific location for requirement failures
		)}
	}
	return nil
}

// ForbidRule creates a rule that forbids certain patterns or conditions.
// The rule returns an error if the forbidden function returns true.
//
//nolint:ireturn // Builder functions should return interfaces
func ForbidRule(name, description string, forbidden ForbiddenFunc) Rule {
	return &forbidRule{
		name:        name,
		description: description,
		forbidden:   forbidden,
	}
}

// forbidRule implements the Rule interface for forbidding patterns.
type forbidRule struct {
	name        string
	description string
	forbidden   ForbiddenFunc
}

// Name returns the unique identifier for this rule.
func (r *forbidRule) Name() string {
	return r.name
}

// Description returns a human-readable description of what this rule checks.
func (r *forbidRule) Description() string {
	return r.description
}

// Check validates that the forbidden condition is not met and returns an error if it is.
func (r *forbidRule) Check(ctx *Context) []Issue {
	if r.forbidden(ctx) {
		return []Issue{NewIssue(
			r.name,
			SeverityError,
			r.description,
			nil, // No specific location for forbidden condition failures
		)}
	}
	return nil
}

// Helper functions for common rule patterns

// HasCommand checks if the current context contains a command of the specified type.
func HasCommand(ctx *Context, cmdType earthfile.CommandType) bool {
	// If we're in a command context, check that command
	if ctx.IsCommandLevel() {
		return ctx.Command.Type == cmdType
	}

	// If we're in a target context, check commands in this target
	if ctx.IsTargetLevel() {
		for _, cmd := range ctx.Target.Commands {
			if cmd.Type == cmdType {
				return true
			}
		}
		return false
	}

	// If we're in a file context, check all targets
	found := false
	_ = ctx.WalkTargets(func(targetCtx *Context) error {
		for _, cmd := range targetCtx.Target.Commands {
			if cmd.Type == cmdType {
				found = true
				return fmt.Errorf("found") // Stop walking
			}
		}
		return nil
	})
	return found
}

// ContainsPattern checks if any command arguments contain the specified pattern.
func ContainsPattern(ctx *Context, pattern string) bool {
	regex := regexp.MustCompile(pattern)

	// If we're in a command context, check that command's args
	if ctx.IsCommandLevel() {
		for _, arg := range ctx.Command.Args {
			if regex.MatchString(arg) {
				return true
			}
		}
		return false
	}

	// If we're in a target context, check all commands in this target
	if ctx.IsTargetLevel() {
		for _, cmd := range ctx.Target.Commands {
			for _, arg := range cmd.Args {
				if regex.MatchString(arg) {
					return true
				}
			}
		}
		return false
	}

	// If we're in a file context, check all commands in all targets
	found := false
	_ = ctx.WalkTargets(func(targetCtx *Context) error {
		for _, cmd := range targetCtx.Target.Commands {
			for _, arg := range cmd.Args {
				if regex.MatchString(arg) {
					found = true
					return fmt.Errorf("found") // Stop walking
				}
			}
		}
		return nil
	})
	return found
}

// ContainsSubstring checks if any command arguments contain the specified substring.
func ContainsSubstring(ctx *Context, substring string) bool {
	// If we're in a command context, check that command's args
	if ctx.IsCommandLevel() {
		for _, arg := range ctx.Command.Args {
			if strings.Contains(arg, substring) {
				return true
			}
		}
		return false
	}

	// If we're in a target context, check all commands in this target
	if ctx.IsTargetLevel() {
		for _, cmd := range ctx.Target.Commands {
			for _, arg := range cmd.Args {
				if strings.Contains(arg, substring) {
					return true
				}
			}
		}
		return false
	}

	// If we're in a file context, check all commands in all targets
	found := false
	_ = ctx.WalkTargets(func(targetCtx *Context) error {
		for _, cmd := range targetCtx.Target.Commands {
			for _, arg := range cmd.Args {
				if strings.Contains(arg, substring) {
					found = true
					return fmt.Errorf("found") // Stop walking
				}
			}
		}
		return nil
	})
	return found
}
