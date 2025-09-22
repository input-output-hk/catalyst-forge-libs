// Package lint provides a flexible, rule-based linting framework for Earthfiles.
// It enables developers to enforce coding standards, security policies, and best practices
// through composable linting rules.
package lint

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Format represents the output format for reporting issues.
type Format int

const (
	// FormatText outputs issues in a human-readable text format.
	FormatText Format = iota
	// FormatJSON outputs issues in JSON format.
	FormatJSON
	// FormatSARIF outputs issues in SARIF (Static Analysis Results Interchange Format).
	FormatSARIF
)

// String returns the string representation of the format.
func (f Format) String() string {
	switch f {
	case FormatText:
		return "text"
	case FormatJSON:
		return "json"
	case FormatSARIF:
		return "sarif"
	default:
		return "unknown"
	}
}

// Reporter handles formatting and outputting linting issues.
type Reporter struct {
	writer io.Writer
	format Format
}

// NewReporter creates a new Reporter with the specified output writer and format.
func NewReporter(writer io.Writer, format Format) *Reporter {
	return &Reporter{
		writer: writer,
		format: format,
	}
}

// Report writes the issues to the output writer in the specified format.
// Issues are sorted by location before reporting.
func (r *Reporter) Report(issues []Issue) error {
	if len(issues) == 0 {
		return nil
	}

	// Sort issues by location for consistent output
	sortedIssues := make([]Issue, len(issues))
	copy(sortedIssues, issues)
	sort.Slice(sortedIssues, func(i, j int) bool {
		return compareIssuesByLocation(sortedIssues[i], sortedIssues[j])
	})

	switch r.format {
	case FormatText:
		return r.reportText(sortedIssues)
	case FormatJSON:
		return r.reportJSON(sortedIssues)
	case FormatSARIF:
		return r.reportSARIF(sortedIssues)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// reportText outputs issues in human-readable text format.
func (r *Reporter) reportText(issues []Issue) error {
	for _, issue := range issues {
		line := issue.String()
		if _, err := fmt.Fprintln(r.writer, line); err != nil {
			return fmt.Errorf("failed to write text output: %w", err)
		}
	}
	return nil
}

// reportJSON outputs issues in JSON format.
func (r *Reporter) reportJSON(issues []Issue) error {
	output := struct {
		Issues []Issue `json:"issues"`
	}{
		Issues: issues,
	}

	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON output: %w", err)
	}
	return nil
}

// reportSARIF outputs issues in SARIF (Static Analysis Results Interchange Format).
func (r *Reporter) reportSARIF(issues []Issue) error {
	// Group issues by rule for SARIF rules section
	ruleMap := make(map[string][]Issue)
	for _, issue := range issues {
		ruleMap[issue.Rule] = append(ruleMap[issue.Rule], issue)
	}

	// Create SARIF rules
	var rules []map[string]interface{}
	for ruleName, ruleIssues := range ruleMap {
		if len(ruleIssues) > 0 {
			rules = append(rules, map[string]interface{}{
				"id":   ruleName,
				"name": ruleName,
				"help": map[string]interface{}{
					"text": ruleIssues[0].Message, // Use first issue's message as help text
				},
			})
		}
	}

	// Create SARIF results
	var results []map[string]interface{}
	for _, issue := range issues {
		result := map[string]interface{}{
			"ruleId":  issue.Rule,
			"level":   issue.Severity.String(),
			"message": map[string]interface{}{"text": issue.Message},
			"locations": []map[string]interface{}{
				{
					"physicalLocation": map[string]interface{}{
						"artifactLocation": map[string]interface{}{
							"uri": getFileURI(issue.Location),
						},
						"region": map[string]interface{}{
							"startLine":   getStartLine(issue.Location),
							"startColumn": getStartColumn(issue.Location),
							"endLine":     getEndLine(issue.Location),
							"endColumn":   getEndColumn(issue.Location),
						},
					},
				},
			},
		}
		results = append(results, result)
	}

	// Create SARIF output
	sarif := map[string]interface{}{
		"version": "2.1.0",
		"$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		"runs": []map[string]interface{}{
			{
				"tool": map[string]interface{}{
					"driver": map[string]interface{}{
						"name":           "earthlint",
						"informationUri": "https://github.com/input-output-hk/catalyst-forge-libs",
						"rules":          rules,
					},
				},
				"results": results,
			},
		},
	}

	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(sarif); err != nil {
		return fmt.Errorf("failed to encode SARIF output: %w", err)
	}
	return nil
}

// Helper functions for SARIF formatting

// getFileURI returns the file URI for SARIF output.
func getFileURI(loc *SourceLocation) string {
	if loc == nil {
		return ""
	}
	return fmt.Sprintf("file://%s", strings.TrimPrefix(loc.File, "/"))
}

// getStartLine returns the start line number for SARIF output.
func getStartLine(loc *SourceLocation) int {
	if loc == nil {
		return 0
	}
	return loc.StartLine
}

// getStartColumn returns the start column number for SARIF output.
func getStartColumn(loc *SourceLocation) int {
	if loc == nil {
		return 0
	}
	return loc.StartColumn
}

// getEndLine returns the end line number for SARIF output.
func getEndLine(loc *SourceLocation) int {
	if loc == nil {
		return 0
	}
	return loc.EndLine
}

// getEndColumn returns the end column number for SARIF output.
func getEndColumn(loc *SourceLocation) int {
	if loc == nil {
		return 0
	}
	return loc.EndColumn
}

// compareIssuesByLocation compares two issues by their location for sorting.
func compareIssuesByLocation(a, b Issue) bool {
	// Handle nil locations
	if a.Location == nil && b.Location == nil {
		return a.Rule < b.Rule
	}
	if a.Location == nil {
		return true
	}
	if b.Location == nil {
		return false
	}

	// Compare by file
	if a.Location.File != b.Location.File {
		return a.Location.File < b.Location.File
	}

	// Compare by start line
	if a.Location.StartLine != b.Location.StartLine {
		return a.Location.StartLine < b.Location.StartLine
	}

	// Compare by start column
	if a.Location.StartColumn != b.Location.StartColumn {
		return a.Location.StartColumn < b.Location.StartColumn
	}

	// Compare by rule name as tiebreaker
	return a.Rule < b.Rule
}
