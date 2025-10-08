package style

import (
	"strings"
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/lint"
)

func TestMaxLineLengthRule(t *testing.T) {
	t.Run("detects lines exceeding max length", func(t *testing.T) {
		rule := NewMaxLineLengthRule(80) // Set low limit for testing

		// Create a long line that exceeds 80 characters
		longLine := "RUN " + strings.Repeat("x", 80) // This will be over 80 chars with "RUN "

		earthfileContent := `
VERSION 0.8

short-target:
	FROM ubuntu
	RUN echo hello
` + "\t" + longLine + `
	RUN echo world
`

		ef, err := earthfile.ParseString(earthfileContent)
		require.NoError(t, err)

		ctx := lint.NewContext(ef)
		issues := rule.Check(ctx)

		assert.Len(t, issues, 1)
		assert.Equal(t, "max-line-length", issues[0].Rule)
		assert.Equal(t, lint.SeverityInfo, issues[0].Severity)
		assert.Contains(t, issues[0].Message, "exceeds maximum")
		assert.Contains(t, issues[0].Message, "80")
	})

	t.Run("accepts lines within max length", func(t *testing.T) {
		rule := NewMaxLineLengthRule(120)

		earthfileContent := `
VERSION 0.8

test-target:
	FROM ubuntu
	RUN echo hello world this is a reasonable length command
	COPY src dst
`

		ef, err := earthfile.ParseString(earthfileContent)
		require.NoError(t, err)

		ctx := lint.NewContext(ef)
		issues := rule.Check(ctx)

		assert.Empty(t, issues)
	})

	t.Run("respects custom max length", func(t *testing.T) {
		rule50 := NewMaxLineLengthRule(50)
		rule200 := NewMaxLineLengthRule(200)

		// Create a line that's 60 characters long
		mediumLine := "RUN " + strings.Repeat("x", 50) // Total ~54 chars

		earthfileContent := `
VERSION 0.8

test-target:
	FROM ubuntu
` + "\t" + mediumLine + `
`

		ef, err := earthfile.ParseString(earthfileContent)
		require.NoError(t, err)

		ctx := lint.NewContext(ef)

		// Should fail with 50 char limit
		issues50 := rule50.Check(ctx)
		assert.Len(t, issues50, 1)

		// Should pass with 200 char limit
		issues200 := rule200.Check(ctx)
		assert.Empty(t, issues200)
	})

	t.Run("handles multiple long lines", func(t *testing.T) {
		rule := NewMaxLineLengthRule(80)

		longLine1 := "RUN " + strings.Repeat("a", 80)
		longLine2 := "COPY " + strings.Repeat("b", 80)

		earthfileContent := `
VERSION 0.8

test-target:
	FROM ubuntu
` + "\t" + longLine1 + `
` + "\t" + longLine2 + `
	RUN echo short
`

		ef, err := earthfile.ParseString(earthfileContent)
		require.NoError(t, err)

		ctx := lint.NewContext(ef)
		issues := rule.Check(ctx)

		assert.Len(t, issues, 2)
		for _, issue := range issues {
			assert.Equal(t, "max-line-length", issue.Rule)
			assert.Equal(t, lint.SeverityInfo, issue.Severity)
		}
	})

	t.Run("default max length", func(t *testing.T) {
		rule := NewMaxLineLengthRule(0) // Should use default

		// Create a line longer than default 120
		longLine := "RUN " + strings.Repeat("x", 130)

		earthfileContent := `
VERSION 0.8

test-target:
	FROM ubuntu
` + "\t" + longLine + `
`

		ef, err := earthfile.ParseString(earthfileContent)
		require.NoError(t, err)

		ctx := lint.NewContext(ef)
		issues := rule.Check(ctx)

		assert.Len(t, issues, 1)
		assert.Contains(t, issues[0].Message, "120")
	})

	t.Run("rule properties", func(t *testing.T) {
		rule := NewMaxLineLengthRule(100)

		assert.Equal(t, "max-line-length", rule.Name())
		assert.Contains(t, rule.Description(), "line length")
		assert.Contains(t, rule.Description(), "120") // Default mentioned in description
	})
}


