package lint

import (
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestEarthfile creates an Earthfile with targets from Earthfile syntax
func createTestEarthfile(content string) (*earthfile.Earthfile, error) {
	return earthfile.ParseString(content)
}

func TestSimpleRule(t *testing.T) {
	t.Run("creates rule with correct name and description", func(t *testing.T) {
		checkCalled := false
		rule := SimpleRule("test-rule", "test description", func(ctx *Context) []Issue {
			checkCalled = true
			return []Issue{NewIssue("test-rule", SeverityError, "test issue", nil)}
		})

		assert.Equal(t, "test-rule", rule.Name())
		assert.Equal(t, "test description", rule.Description())

		// Test execution
		ctx := NewContext(earthfile.NewEarthfile())
		issues := rule.Check(ctx)

		assert.True(t, checkCalled)
		require.Len(t, issues, 1)
		assert.Equal(t, "test-rule", issues[0].Rule)
		assert.Equal(t, SeverityError, issues[0].Severity)
		assert.Equal(t, "test issue", issues[0].Message)
	})

	t.Run("handles nil issues return", func(t *testing.T) {
		rule := SimpleRule("test-rule", "test description", func(ctx *Context) []Issue {
			return nil
		})

		ctx := NewContext(earthfile.NewEarthfile())
		issues := rule.Check(ctx)

		assert.Empty(t, issues)
	})
}

func TestCommandRule(t *testing.T) {
	t.Run("creates rule with correct properties", func(t *testing.T) {
		rule := CommandRule(
			"test-cmd-rule",
			"test description",
			earthfile.CommandTypeRun,
			func(ctx *Context, cmd *earthfile.Command) []Issue {
				return []Issue{NewIssue("test-cmd-rule", SeverityWarning, "test issue", nil)}
			},
		)

		assert.Equal(t, "test-cmd-rule", rule.Name())
		assert.Equal(t, "test description", rule.Description())
	})

	t.Run("only checks matching command types", func(t *testing.T) {
		checkCount := 0
		rule := CommandRule(
			"run-rule",
			"checks RUN commands",
			earthfile.CommandTypeRun,
			func(ctx *Context, cmd *earthfile.Command) []Issue {
				checkCount++
				return []Issue{NewIssue("run-rule", SeverityWarning, "RUN issue", nil)}
			},
		)

		// Create Earthfile with actual Earthfile syntax
		ef, err := createTestEarthfile(`
VERSION 0.8

test-target:
	FROM ubuntu
	RUN echo hello
	COPY src dst
	RUN echo world
`)
		require.NoError(t, err)

		ctx := NewContext(ef) // Use file-level context for full traversal
		issues := rule.Check(ctx)

		assert.Equal(t, 2, checkCount) // Should only check RUN commands
		assert.Len(t, issues, 2)       // Should return 2 issues
		for _, issue := range issues {
			assert.Equal(t, "run-rule", issue.Rule)
			assert.Equal(t, SeverityWarning, issue.Severity)
			assert.Equal(t, "RUN issue", issue.Message)
		}
	})

	t.Run("handles nil issues from check function", func(t *testing.T) {
		rule := CommandRule(
			"no-issue-rule",
			"no issues",
			earthfile.CommandTypeRun,
			func(ctx *Context, cmd *earthfile.Command) []Issue {
				return nil
			},
		)

		ef := earthfile.NewEarthfile()
		target := &earthfile.Target{
			Name:     "test-target",
			Commands: []*earthfile.Command{{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"echo"}}},
		}

		ctx := NewTargetContext(NewContext(ef), target)
		issues := rule.Check(ctx)

		assert.Empty(t, issues)
	})
}

func TestPatternRule(t *testing.T) {
	t.Run("creates rule with correct properties", func(t *testing.T) {
		rule := PatternRule("sudo-rule", "detects sudo usage", `sudo`, SeverityError)

		assert.Equal(t, "sudo-rule", rule.Name())
		assert.Equal(t, "detects sudo usage", rule.Description())
	})

	t.Run("panics on invalid regex pattern", func(t *testing.T) {
		assert.Panics(t, func() {
			PatternRule("bad-rule", "bad pattern", `[invalid`, SeverityError)
		})
	})

	t.Run("detects pattern in command arguments", func(t *testing.T) {
		rule := PatternRule("sudo-pattern", "detects sudo", `sudo`, SeverityError)

		// Create Earthfile with actual Earthfile syntax
		ef, err := createTestEarthfile(`
VERSION 0.8

test-target:
	FROM ubuntu
	RUN apt-get update
	RUN sudo apt-get upgrade
	COPY src dst
`)
		require.NoError(t, err)

		ctx := NewContext(ef) // Use file-level context
		issues := rule.Check(ctx)

		require.Len(t, issues, 1)
		assert.Equal(t, "sudo-pattern", issues[0].Rule)
		assert.Equal(t, SeverityError, issues[0].Severity)
		assert.Contains(t, issues[0].Message, "Found forbidden pattern")
		assert.Contains(t, issues[0].Message, "sudo")
	})

	t.Run("handles multiple pattern matches", func(t *testing.T) {
		rule := PatternRule("echo-pattern", "detects echo", `echo`, SeverityWarning)

		// Create Earthfile with actual Earthfile syntax
		ef, err := createTestEarthfile(`
VERSION 0.8

test-target:
	FROM ubuntu
	RUN echo hello
	RUN echo world
	RUN printf test
`)
		require.NoError(t, err)

		ctx := NewContext(ef) // Use file-level context
		issues := rule.Check(ctx)

		assert.Len(t, issues, 2) // Should match both echo commands
		for _, issue := range issues {
			assert.Equal(t, "echo-pattern", issue.Rule)
			assert.Equal(t, SeverityWarning, issue.Severity)
		}
	})
}

func TestRequireRule(t *testing.T) {
	t.Run("creates rule with correct properties", func(t *testing.T) {
		rule := RequireRule("version-rule", "requires VERSION command", func(ctx *Context) bool {
			return HasCommand(ctx, earthfile.CommandTypeVersion)
		})

		assert.Equal(t, "version-rule", rule.Name())
		assert.Equal(t, "requires VERSION command", rule.Description())
	})

	t.Run("returns no issues when requirement is met", func(t *testing.T) {
		rule := RequireRule("version-rule", "requires VERSION", func(ctx *Context) bool {
			return true // Always satisfied
		})

		ctx := NewContext(earthfile.NewEarthfile())
		issues := rule.Check(ctx)

		assert.Empty(t, issues)
	})

	t.Run("returns error issue when requirement is not met", func(t *testing.T) {
		rule := RequireRule("version-rule", "VERSION command required", func(ctx *Context) bool {
			return false // Never satisfied
		})

		ctx := NewContext(earthfile.NewEarthfile())
		issues := rule.Check(ctx)

		require.Len(t, issues, 1)
		assert.Equal(t, "version-rule", issues[0].Rule)
		assert.Equal(t, SeverityError, issues[0].Severity)
		assert.Equal(t, "VERSION command required", issues[0].Message)
		assert.Nil(t, issues[0].Location) // No specific location for requirement failures
	})
}

func TestForbidRule(t *testing.T) {
	t.Run("creates rule with correct properties", func(t *testing.T) {
		rule := ForbidRule("no-sudo", "forbids sudo usage", func(ctx *Context) bool {
			return ContainsSubstring(ctx, "sudo")
		})

		assert.Equal(t, "no-sudo", rule.Name())
		assert.Equal(t, "forbids sudo usage", rule.Description())
	})

	t.Run("returns no issues when condition is not met", func(t *testing.T) {
		rule := ForbidRule("no-sudo", "no sudo allowed", func(ctx *Context) bool {
			return false // Condition never met
		})

		ctx := NewContext(earthfile.NewEarthfile())
		issues := rule.Check(ctx)

		assert.Empty(t, issues)
	})

	t.Run("returns error issue when forbidden condition is met", func(t *testing.T) {
		rule := ForbidRule("no-sudo", "sudo is forbidden", func(ctx *Context) bool {
			return true // Condition always met
		})

		ctx := NewContext(earthfile.NewEarthfile())
		issues := rule.Check(ctx)

		require.Len(t, issues, 1)
		assert.Equal(t, "no-sudo", issues[0].Rule)
		assert.Equal(t, SeverityError, issues[0].Severity)
		assert.Equal(t, "sudo is forbidden", issues[0].Message)
		assert.Nil(t, issues[0].Location) // No specific location for forbidden condition failures
	})
}

func TestHasCommand(t *testing.T) {
	tests := []struct {
		name     string
		commands []*earthfile.Command
		cmdType  earthfile.CommandType
		expected bool
	}{
		{
			name: "finds existing command type",
			commands: []*earthfile.Command{
				{Name: "FROM", Type: earthfile.CommandTypeFrom, Args: []string{"ubuntu"}},
				{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"echo"}},
			},
			cmdType:  earthfile.CommandTypeRun,
			expected: true,
		},
		{
			name: "does not find non-existing command type",
			commands: []*earthfile.Command{
				{Name: "FROM", Type: earthfile.CommandTypeFrom, Args: []string{"ubuntu"}},
			},
			cmdType:  earthfile.CommandTypeRun,
			expected: false,
		},
		{
			name:     "handles empty command list",
			commands: []*earthfile.Command{},
			cmdType:  earthfile.CommandTypeRun,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := earthfile.NewEarthfile()
			target := &earthfile.Target{Name: "test", Commands: tt.commands}
			ctx := NewTargetContext(NewContext(ef), target)

			result := HasCommand(ctx, tt.cmdType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsPattern(t *testing.T) {
	tests := []struct {
		name     string
		commands []*earthfile.Command
		pattern  string
		expected bool
	}{
		{
			name: "finds matching pattern",
			commands: []*earthfile.Command{
				{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"echo", "hello"}},
				{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"sudo", "apt-get"}},
			},
			pattern:  `sudo`,
			expected: true,
		},
		{
			name: "does not find non-matching pattern",
			commands: []*earthfile.Command{
				{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"echo", "hello"}},
			},
			pattern:  `sudo`,
			expected: false,
		},
		{
			name: "matches regex patterns",
			commands: []*earthfile.Command{
				{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"echo", "test123"}},
			},
			pattern:  `\d+`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := earthfile.NewEarthfile()
			target := &earthfile.Target{Name: "test", Commands: tt.commands}
			ctx := NewTargetContext(NewContext(ef), target)

			result := ContainsPattern(ctx, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsSubstring(t *testing.T) {
	tests := []struct {
		name      string
		commands  []*earthfile.Command
		substring string
		expected  bool
	}{
		{
			name: "finds matching substring",
			commands: []*earthfile.Command{
				{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"echo", "hello"}},
				{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"sudo", "apt-get"}},
			},
			substring: "sudo",
			expected:  true,
		},
		{
			name: "does not find non-matching substring",
			commands: []*earthfile.Command{
				{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"echo", "hello"}},
			},
			substring: "sudo",
			expected:  false,
		},
		{
			name: "matches partial words",
			commands: []*earthfile.Command{
				{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"pseudonym"}},
			},
			substring: "sudo",
			expected:  false, // "pseudonym" contains "sudo" but not as substring match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := earthfile.NewEarthfile()
			target := &earthfile.Target{Name: "test", Commands: tt.commands}
			ctx := NewTargetContext(NewContext(ef), target)

			result := ContainsSubstring(ctx, tt.substring)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuilderFunctionsReturnInterfaces(t *testing.T) {
	// Test that all builder functions return Rule interface
	simpleRule := SimpleRule("test", "test", func(ctx *Context) []Issue { return nil })
	commandRule := CommandRule(
		"test",
		"test",
		earthfile.CommandTypeRun,
		func(ctx *Context, cmd *earthfile.Command) []Issue { return nil },
	)
	patternRule := PatternRule("test", "test", "pattern", SeverityInfo)
	requireRule := RequireRule("test", "test", func(ctx *Context) bool { return true })
	forbidRule := ForbidRule("test", "test", func(ctx *Context) bool { return false })

	// All should implement the Rule interface
	_ = simpleRule
	_ = commandRule
	_ = patternRule
	_ = requireRule
	_ = forbidRule

	// All should have the expected methods
	assert.NotEmpty(t, simpleRule.Name())
	assert.NotEmpty(t, commandRule.Name())
	assert.NotEmpty(t, patternRule.Name())
	assert.NotEmpty(t, requireRule.Name())
	assert.NotEmpty(t, forbidRule.Name())
}
