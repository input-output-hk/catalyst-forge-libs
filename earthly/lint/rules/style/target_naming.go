// Package style provides style-related linting rules for Earthfiles.
// These rules enforce consistent formatting and naming conventions.
package style

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/lint"
)

// TargetNamingRule enforces kebab-case naming convention for target names.
// Kebab-case uses lowercase letters and hyphens, following the pattern: ^[a-z][a-z0-9-]*$
type TargetNamingRule struct{}

// NewTargetNamingRule creates a new target naming rule.
func NewTargetNamingRule() *TargetNamingRule {
	return &TargetNamingRule{}
}

// Name returns the unique identifier for this rule.
func (r *TargetNamingRule) Name() string {
	return "target-naming"
}

// Description returns a human-readable description of what this rule checks.
func (r *TargetNamingRule) Description() string {
	return "Enforces kebab-case naming convention for target names (lowercase with hyphens)"
}

// Check examines all targets in the Earthfile and reports issues for
// target names that don't follow kebab-case convention.
func (r *TargetNamingRule) Check(ctx *lint.Context) []lint.Issue {
	var issues []lint.Issue

	// Walk through all targets in the Earthfile
	err := ctx.WalkTargets(func(targetCtx *lint.Context) error {
		targetName := targetCtx.Target.Name

		// Check if target name needs improvement
		if issue := r.checkTargetName(targetName, targetCtx); issue != nil {
			issues = append(issues, *issue)
		}

		return nil
	})
	// If walking failed, return empty issues (don't crash)
	if err != nil {
		return nil
	}

	return issues
}

// checkTargetName checks a single target name and returns an issue if it violates the rule.
func (r *TargetNamingRule) checkTargetName(targetName string, targetCtx *lint.Context) *lint.Issue {
	// Basic validation - ensure it follows allowed character rules
	if !isValidKebabCase(targetName) {
		var location *lint.SourceLocation
		if len(targetCtx.Target.Commands) > 0 {
			location = targetCtx.Target.Commands[0].SourceLocation()
		}

		return &lint.Issue{
			Rule:     r.Name(),
			Severity: lint.SeverityInfo,
			Message: fmt.Sprintf(
				"Target name '%s' contains invalid characters for kebab-case",
				targetName,
			),
			Location: location,
			Context:  map[string]interface{}{"target_name": targetName},
		}
	}

	// Heuristic: Flag long target names without hyphens that might benefit from splitting
	// This helps identify compound words that should be hyphenated
	if len(targetName) > 12 && !strings.Contains(targetName, "-") {
		var location *lint.SourceLocation
		if len(targetCtx.Target.Commands) > 0 {
			location = targetCtx.Target.Commands[0].SourceLocation()
		}

		return &lint.Issue{
			Rule:     r.Name(),
			Severity: lint.SeverityInfo,
			Message: fmt.Sprintf(
				"Target name '%s' is long and could benefit from kebab-case (hyphenated) formatting",
				targetName,
			),
			Location: location,
			Context:  map[string]interface{}{"target_name": targetName},
		}
	}

	return nil
}

// isValidKebabCase checks if a string follows kebab-case naming convention.
// For this rule, we consider a name valid if:
// - It contains only lowercase letters, numbers, and hyphens
// - No consecutive hyphens, no leading/trailing hyphens
// - Single words (no hyphens) are acceptable
// - Compound words should use kebab-case (hyphenated)
func isValidKebabCase(s string) bool {
	if s == "" {
		return false
	}

	// Must contain only valid characters
	validPattern := regexp.MustCompile(`^[a-z0-9-]+$`)
	if !validPattern.MatchString(s) {
		return false
	}

	// No leading/trailing hyphens
	if strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") {
		return false
	}

	// No consecutive hyphens
	if strings.Contains(s, "--") {
		return false
	}

	// For now, all valid names are considered acceptable
	// In the future, this could be enhanced to detect compound words
	// that should be hyphenated for better readability
	return true
}
