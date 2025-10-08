package style

import (
	"fmt"
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/lint"
)

func TestTargetNamingRule(t *testing.T) {
	t.Run("detects invalid target names", func(t *testing.T) {
		rule := NewTargetNamingRule()

		// Test cases with invalid target names
		testCases := []struct {
			name             string
			earthfile        string
			expectedIssues   int
			expectedRule     string
			expectedSeverity lint.Severity
		}{
			{
				name: "single word target (should pass)",
				earthfile: `
VERSION 0.8

testtarget:
	FROM ubuntu
	RUN echo hello
`,
				expectedIssues:   0,
				expectedRule:     "",
				expectedSeverity: lint.SeverityInfo,
			},
			{
				name: "valid kebab-case target",
				earthfile: `
VERSION 0.8

test-target:
	FROM ubuntu
	RUN echo hello
`,
				expectedIssues:   0,
				expectedRule:     "",
				expectedSeverity: lint.SeverityInfo,
			},
			{
				name: "multi-word without hyphens",
				earthfile: `
VERSION 0.8

testtargetname:
	FROM ubuntu
	RUN echo hello
`,
				expectedIssues:   1,
				expectedRule:     "target-naming",
				expectedSeverity: lint.SeverityInfo,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ef, err := earthfile.ParseString(tc.earthfile)
				require.NoError(t, err)

				ctx := lint.NewContext(ef)
				issues := rule.Check(ctx)

				assert.Len(t, issues, tc.expectedIssues)
				if tc.expectedIssues > 0 && len(issues) > 0 {
					assert.Equal(t, tc.expectedRule, issues[0].Rule)
					assert.Equal(t, tc.expectedSeverity, issues[0].Severity)
					assert.Contains(t, issues[0].Message, "kebab-case")
				}
			})
		}
	})

	t.Run("accepts valid kebab-case target names", func(t *testing.T) {
		rule := NewTargetNamingRule()

		validEarthfiles := []string{
			`
VERSION 0.8

test-target:
	FROM ubuntu
	RUN echo hello
`,
			`
VERSION 0.8

my-test-target:
	FROM ubuntu
	RUN echo hello
`,
			`
VERSION 0.8

api-server:
	FROM ubuntu
	RUN echo hello
`,
			`
VERSION 0.8

build-image:
	FROM ubuntu
	RUN echo hello
`,
			`
VERSION 0.8

test-123:
	FROM ubuntu
	RUN echo hello
`,
		}

		for i, earthfileContent := range validEarthfiles {
			t.Run(fmt.Sprintf("valid case %d", i), func(t *testing.T) {
				ef, err := earthfile.ParseString(earthfileContent)
				require.NoError(t, err)

				ctx := lint.NewContext(ef)
				issues := rule.Check(ctx)

				assert.Empty(t, issues, "Expected no issues for valid kebab-case target name")
			})
		}
	})

	t.Run("handles multiple targets with mixed validity", func(t *testing.T) {
		rule := NewTargetNamingRule()

		earthfileContent := `
VERSION 0.8

valid-target:
	FROM ubuntu
	RUN echo hello

verylongtargetnamethatshouldbeflagged:
	FROM ubuntu
	RUN echo world

another-valid-target:
	FROM ubuntu
	RUN echo test

alsolongtargetnamewithoutanyhyphens:
	FROM ubuntu
	RUN echo final
`

		ef, err := earthfile.ParseString(earthfileContent)
		require.NoError(t, err)

		ctx := lint.NewContext(ef)
		issues := rule.Check(ctx)

		// Should have 2 issues for the long target names without hyphens
		assert.Len(t, issues, 2)
		for _, issue := range issues {
			assert.Equal(t, "target-naming", issue.Rule)
			assert.Equal(t, lint.SeverityInfo, issue.Severity)
			assert.Contains(t, issue.Message, "long and could benefit")
		}
	})

	t.Run("rule properties", func(t *testing.T) {
		rule := NewTargetNamingRule()

		assert.Equal(t, "target-naming", rule.Name())
		assert.Contains(t, rule.Description(), "kebab-case")
		assert.Contains(t, rule.Description(), "target names")
	})
}
