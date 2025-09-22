# Earthlint - Linting Framework Architecture

## Overview

The `earthlint` module provides a flexible, rule-based linting framework for Earthfiles. It enables developers to enforce coding standards, security policies, and best practices through composable linting rules.

## Module Information

- **Package**: `github.com/yourdomain/earthlint`
- **Purpose**: Lint, validate, and enforce standards for Earthfiles
- **Dependencies**:
  - `github.com/yourdomain/earthfile` - Core parser module
  - `github.com/pkg/errors` - Error handling
  - `regexp` - Pattern matching

## Architecture Principles

1. **Rule Composability**: Rules should be independent and composable
2. **Context Awareness**: Rules have access to full context (file, target, command)
3. **Performance**: Rules should be fast; expensive operations should be cached
4. **Extensibility**: Easy to add custom rules without modifying core
5. **Fix Suggestions**: Support automated fixes where possible
6. **Configuration**: Flexible configuration at multiple levels

## Core Components

### 1. Rule Engine Architecture

```
┌─────────────────┐
│   Earthfile     │
│    (parsed)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│     Linter      │
│                 │
│  ┌───────────┐  │
│  │   Rules   │  │
│  │    [ ]    │  │
│  └─────┬─────┘  │
└────────┼────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌────────┐ ┌────────┐
│ Rule 1 │ │ Rule 2 │ ...
└───┬────┘ └───┬────┘
    │          │
    ▼          ▼
┌─────────────────┐
│     Issues      │
│       [ ]       │
└─────────────────┘
```

### 2. Rule System

#### Rule Interface

```go
type Rule interface {
    Name() string           // Unique identifier
    Description() string    // Human-readable description
    Check(*LintContext) []Issue
}
```

#### Rule Categories

1. **Security Rules**: Detect security anti-patterns
2. **Best Practice Rules**: Enforce Earthfile best practices
3. **Style Rules**: Ensure consistent formatting
4. **Dependency Rules**: Validate dependency management
5. **Documentation Rules**: Enforce documentation standards

#### Rule Execution Model

```
For each rule:
  1. Create LintContext (file-level)
  2. For each target:
     a. Create target context
     b. For each command:
        - Create command context
        - Execute rule.Check()
        - Collect issues
  3. Aggregate and deduplicate issues
  4. Apply severity overrides
  5. Sort by location
```

### 3. Context System

The `LintContext` provides rule implementations with necessary information:

```go
type LintContext struct {
    File      *earthfile.Earthfile  // Current file
    Target    *earthfile.Target      // Current target (nil for global)
    Command   *earthfile.Command     // Current command (nil for target-level)
    Parent    *LintContext           // Parent context for nested structures

    // Cached data for performance
    cache     map[string]interface{} // Rule-specific cache
    visited   map[string]bool        // Track visited nodes
}
```

#### Context Hierarchy

```
File Context
├── Global Commands Context
├── Target Context
│   ├── Command Context
│   ├── If Statement Context
│   │   └── Command Context
│   └── For Statement Context
│       └── Command Context
└── Function Context
    └── Command Context
```

### 4. Issue Reporting

```go
type Issue struct {
    Rule     string         // Rule that found the issue
    Severity Severity       // Error, Warning, Info
    Message  string         // Human-readable message
    Location SourceLocation // File position
    Fix      *Fix          // Optional auto-fix
    Context  map[string]interface{} // Additional metadata
}
```

#### Severity Levels

- **Error**: Must fix, blocks build
- **Warning**: Should fix, potential issue
- **Info**: Consider fixing, suggestion

### 5. Rule Builders

Helper functions to create common rule patterns:

```go
// Simple function-based rule
SimpleRule(name, description string, check CheckFunc) Rule

// Command-specific rule
CommandRule(name, description string, cmdType CommandType, check CommandCheckFunc) Rule

// Pattern-matching rule
PatternRule(name, description, pattern string, severity Severity) Rule

// Requirement rule (something must exist)
RequireRule(name, description string, requirement RequirementFunc) Rule

// Forbid rule (something must not exist)
ForbidRule(name, description string, forbidden ForbiddenFunc) Rule
```

## Rule Implementation Patterns

### 1. Stateless Rules

Simple rules that check individual elements:

```go
type NoSudoRule struct{}

func (r NoSudoRule) Check(ctx *LintContext) []Issue {
    if ctx.Command != nil && ctx.Command.Type == CommandRun {
        if strings.Contains(ctx.Command.Args[0], "sudo") {
            return []Issue{{
                Rule:     "no-sudo",
                Severity: SeverityWarning,
                Message:  "Avoid using sudo in RUN commands",
                Location: ctx.Command.SourceLocation(),
            }}
        }
    }
    return nil
}
```

### 2. Stateful Rules

Rules that accumulate state across checks:

```go
type UnusedTargetsRule struct {
    defined  map[string]bool
    used     map[string]bool
}

func (r *UnusedTargetsRule) Check(ctx *LintContext) []Issue {
    // Track defined targets
    if ctx.Target != nil {
        r.defined[ctx.Target.Name] = true
    }

    // Track used targets
    if ctx.Command != nil && ctx.Command.Type == CommandBuild {
        ref, _ := ctx.Command.GetReference()
        r.used[ref.Target] = true
    }

    // Report unused at file level
    if ctx.Target == nil && ctx.Command == nil {
        var issues []Issue
        for name := range r.defined {
            if !r.used[name] {
                issues = append(issues, Issue{
                    Rule:    "unused-targets",
                    Message: fmt.Sprintf("Target '%s' is never used", name),
                })
            }
        }
        return issues
    }

    return nil
}
```

### 3. Cross-Reference Rules

Rules that check relationships between elements:

```go
type CyclicDependencyRule struct{}

func (r CyclicDependencyRule) Check(ctx *LintContext) []Issue {
    graph := buildDependencyGraph(ctx.File)
    if cycle := findCycle(graph); cycle != nil {
        return []Issue{{
            Rule:    "no-cycles",
            Message: fmt.Sprintf("Cyclic dependency: %s", strings.Join(cycle, " -> ")),
        }}
    }
    return nil
}
```

## Built-in Rules

### Security Rules

| Rule | Description | Severity |
|------|-------------|----------|
| `no-sudo` | Forbid sudo usage | Warning |
| `no-root-user` | Forbid USER root | Error |
| `no-curl-pipe` | Forbid curl \| sh pattern | Error |
| `no-secrets` | Detect hardcoded secrets | Error |
| `no-privileged` | Forbid --privileged flag | Error |

### Best Practice Rules

| Rule | Description | Severity |
|------|-------------|----------|
| `require-version` | VERSION must be specified | Error |
| `no-latest-tags` | Forbid :latest tags | Warning |
| `require-from-first` | FROM must be first command | Warning |
| `no-empty-targets` | Targets must have commands | Warning |
| `unique-targets` | Target names must be unique | Error |

### Style Rules

| Rule | Description | Severity |
|------|-------------|----------|
| `target-naming` | Enforce kebab-case | Info |
| `max-line-length` | Lines under 120 chars | Info |
| `sorted-args` | ARGs in alphabetical order | Info |

## Configuration System

### Configuration Levels

1. **Default**: Built-in defaults
2. **File**: `.earthlint.yaml` in project root
3. **Inline**: Comments in Earthfile
4. **Runtime**: Programmatic configuration

### Configuration Schema

```yaml
# .earthlint.yaml
version: "1.0"

# Global settings
severity_overrides:
  no-latest-tags: error  # Upgrade from warning
  target-naming: ignore  # Disable

disabled_rules:
  - sorted-args
  - max-line-length

# Rule-specific configuration
rules:
  max-line-length:
    max: 100

  target-naming:
    pattern: "^[a-z][a-z0-9-]*$"

  max-dependencies:
    max: 10

# Custom rules
custom_rules:
  - path: ./rules/custom.go
    name: company-standards

# Ignore patterns
ignore:
  - "**/vendor/**"
  - "test/**"
```

### Inline Configuration

```earthfile
# earthlint:disable no-sudo
RUN sudo apt-get update

# earthlint:disable-next-line no-latest-tags
FROM node:latest

# earthlint:disable-file no-documentation
```

## Performance Optimization

### Caching Strategy

1. **Parse Once**: Parse Earthfile once, share across all rules
2. **Index Commands**: Pre-index commands by type for O(1) lookup
3. **Cache Patterns**: Compile regex patterns once
4. **Memoize Results**: Cache expensive computations

### Parallel Execution

```go
type parallelLinter struct {
    rules []Rule
    workers int
}

func (l *parallelLinter) Check(ef *Earthfile) []Issue {
    ctx := createContext(ef)
    issueChan := make(chan []Issue, len(l.rules))

    for _, rule := range l.rules {
        go func(r Rule) {
            issueChan <- r.Check(ctx)
        }(rule)
    }

    // Aggregate results
    var allIssues []Issue
    for i := 0; i < len(l.rules); i++ {
        allIssues = append(allIssues, <-issueChan...)
    }

    return allIssues
}
```

## Integration Points

### CI/CD Integration

```bash
# GitHub Actions
- name: Lint Earthfiles
  run: earthlint ./Earthfile

# GitLab CI
lint:
  script:
    - earthlint --format=gitlab ./Earthfile

# Jenkins
stage('Lint') {
    sh 'earthlint --format=junit > lint-results.xml'
}
```

### IDE Integration

```json
// VS Code settings.json
{
  "earthfile.linter": "earthlint",
  "earthfile.lintOnSave": true,
  "earthfile.lintRules": {
    "no-sudo": "error",
    "no-latest-tags": "warning"
  }
}
```

### Pre-commit Hook

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/yourdomain/earthlint
    rev: v1.0.0
    hooks:
      - id: earthlint
        files: Earthfile$
```

## Output Formats

### Text (Default)
```
Earthfile:10:5 [no-sudo] Avoid using sudo in RUN commands
Earthfile:15:1 [no-latest-tags] Avoid using :latest tags
```

### JSON
```json
{
  "issues": [
    {
      "rule": "no-sudo",
      "severity": "warning",
      "message": "Avoid using sudo in RUN commands",
      "location": {
        "file": "Earthfile",
        "line": 10,
        "column": 5
      }
    }
  ]
}
```

### SARIF (Static Analysis Results Interchange Format)
```json
{
  "version": "2.1.0",
  "runs": [{
    "tool": {
      "driver": {
        "name": "earthlint",
        "rules": [...]
      }
    },
    "results": [...]
  }]
}
```

## Testing Strategy

### Rule Testing

```go
func TestNoSudoRule(t *testing.T) {
    tests := []struct {
        name     string
        earthfile string
        want     []string // Expected issue messages
    }{
        {
            name: "detects sudo",
            earthfile: `
                VERSION 0.7
                FROM ubuntu
                RUN sudo apt-get update
            `,
            want: []string{"Avoid using sudo"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ef, _ := earthfile.ParseString(tt.earthfile)
            rule := &NoSudoRule{}
            issues := rule.Check(createContext(ef))
            // Assert issues match expected
        })
    }
}
```

### Integration Testing

1. **Golden Files**: Compare output against expected results
2. **Regression Tests**: Ensure fixes don't break existing rules
3. **Performance Tests**: Ensure rules complete within time limits
4. **Compatibility Tests**: Test against different Earthfile versions

## Extensibility

### Custom Rule Development

```go
// my_rules.go
package myrules

import "github.com/yourdomain/earthlint"

type MyCustomRule struct {
    config map[string]interface{}
}

func (r MyCustomRule) Name() string {
    return "my-custom-rule"
}

func (r MyCustomRule) Description() string {
    return "Enforces company-specific standards"
}

func (r MyCustomRule) Check(ctx *earthlint.LintContext) []earthlint.Issue {
    // Custom logic here
    return nil
}

// Register rule
func init() {
    earthlint.RegisterRule(MyCustomRule{})
}
```

### Plugin System (Future)

```go
type Plugin interface {
    Name() string
    Version() string
    Rules() []Rule
}

func LoadPlugin(path string) (Plugin, error) {
    // Dynamic loading of rule plugins
}
```

## Performance Benchmarks

```
Small Earthfile (10 targets, 50 commands):
  - Parse: 1ms
  - Lint (10 rules): 5ms
  - Total: 6ms

Medium Earthfile (50 targets, 500 commands):
  - Parse: 10ms
  - Lint (10 rules): 25ms
  - Total: 35ms

Large Earthfile (200 targets, 2000 commands):
  - Parse: 50ms
  - Lint (10 rules): 100ms
  - Total: 150ms

Memory usage:
  - Base: ~5MB
  - Per rule: ~500KB
  - Large file: ~20MB peak
```

## Future Enhancements

1. **Auto-fix System**: Automatically apply fixes for common issues
2. **Rule Composition**: Combine simple rules into complex ones
3. **Machine Learning**: Learn patterns from codebase
4. **Incremental Linting**: Only lint changed portions
5. **Rule Marketplace**: Share custom rules across teams
6. **Visual Rule Editor**: GUI for creating custom rules

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

[License details here]