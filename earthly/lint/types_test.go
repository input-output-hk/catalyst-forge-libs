package lint

import (
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeverityString(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     string
	}{
		{
			name:     "error severity",
			severity: SeverityError,
			want:     "error",
		},
		{
			name:     "warning severity",
			severity: SeverityWarning,
			want:     "warning",
		},
		{
			name:     "info severity",
			severity: SeverityInfo,
			want:     "info",
		},
		{
			name:     "unknown severity",
			severity: Severity(999),
			want:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.severity.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssueString(t *testing.T) {
	tests := []struct {
		name  string
		issue Issue
		want  string
	}{
		{
			name: "issue with location",
			issue: Issue{
				Rule:     "test-rule",
				Severity: SeverityError,
				Message:  "test message",
				Location: &earthfile.SourceLocation{
					File:        "Earthfile",
					StartLine:   10,
					StartColumn: 5,
					EndLine:     10,
					EndColumn:   15,
				},
			},
			want: "Earthfile:10:5 [test-rule] test message",
		},
		{
			name: "issue without location",
			issue: Issue{
				Rule:     "test-rule",
				Severity: SeverityWarning,
				Message:  "test message without location",
			},
			want: "[test-rule] test message without location",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.issue.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssueIsValid(t *testing.T) {
	tests := []struct {
		name  string
		issue Issue
		want  bool
	}{
		{
			name: "valid issue",
			issue: Issue{
				Rule:    "test-rule",
				Message: "test message",
			},
			want: true,
		},
		{
			name: "invalid issue - missing rule",
			issue: Issue{
				Message: "test message",
			},
			want: false,
		},
		{
			name: "invalid issue - missing message",
			issue: Issue{
				Rule: "test-rule",
			},
			want: false,
		},
		{
			name:  "invalid issue - missing both",
			issue: Issue{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.issue.IsValid()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewIssue(t *testing.T) {
	location := &earthfile.SourceLocation{
		File:        "Earthfile",
		StartLine:   1,
		StartColumn: 0,
		EndLine:     1,
		EndColumn:   5,
	}

	issue := NewIssue("test-rule", SeverityError, "test message", location)

	assert.Equal(t, "test-rule", issue.Rule)
	assert.Equal(t, SeverityError, issue.Severity)
	assert.Equal(t, "test message", issue.Message)
	assert.Equal(t, location, issue.Location)
	assert.NotNil(t, issue.Context)
	assert.Empty(t, issue.Context)
	assert.Nil(t, issue.Fix)
}

func TestIssueWithFix(t *testing.T) {
	location := &earthfile.SourceLocation{
		File:        "Earthfile",
		StartLine:   1,
		StartColumn: 0,
		EndLine:     1,
		EndColumn:   5,
	}

	fixLocation := &earthfile.SourceLocation{
		File:        "Earthfile",
		StartLine:   1,
		StartColumn: 0,
		EndLine:     1,
		EndColumn:   10,
	}

	issue := NewIssue("test-rule", SeverityWarning, "test message", location).
		WithFix("fix description", "before", "after", fixLocation)

	require.NotNil(t, issue.Fix)
	assert.Equal(t, "fix description", issue.Fix.Description)
	assert.Equal(t, "before", issue.Fix.Before)
	assert.Equal(t, "after", issue.Fix.After)
	assert.Equal(t, fixLocation, issue.Fix.Location)
}

func TestIssueWithContext(t *testing.T) {
	issue := NewIssue("test-rule", SeverityInfo, "test message", nil).
		WithContext("key1", "value1").
		WithContext("key2", 42)

	assert.NotNil(t, issue.Context)
	assert.Equal(t, "value1", issue.Context["key1"])
	assert.Equal(t, 42, issue.Context["key2"])
}

func TestIssueWithContextMultiple(t *testing.T) {
	// Test that WithContext doesn't overwrite existing context
	issue := NewIssue("test-rule", SeverityInfo, "test message", nil).
		WithContext("key1", "value1").
		WithContext("key2", 42)

	// Add more context
	issue = issue.WithContext("key3", true)

	assert.NotNil(t, issue.Context)
	assert.Equal(t, "value1", issue.Context["key1"])
	assert.Equal(t, 42, issue.Context["key2"])
	assert.Equal(t, true, issue.Context["key3"])
}

func TestSourceLocationReuse(t *testing.T) {
	// Test that SourceLocation is correctly aliased from earthfile package
	location := &SourceLocation{
		File:        "test.earth",
		StartLine:   5,
		StartColumn: 10,
		EndLine:     5,
		EndColumn:   20,
	}

	// Should be assignable to earthfile.SourceLocation
	efLocation := location
	assert.Equal(t, "test.earth", efLocation.File)
	assert.Equal(t, 5, efLocation.StartLine)
	assert.Equal(t, 10, efLocation.StartColumn)
	assert.Equal(t, 5, efLocation.EndLine)
	assert.Equal(t, 20, efLocation.EndColumn)
}
