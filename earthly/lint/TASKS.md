# Earthlint Module Implementation Tasks

## Preamble

### Development Standards
- **MANDATORY**: All code MUST conform to `/docs/guides/go/style.md`
- **MANDATORY**: All code MUST conform to `/docs/CONSTITUTION.md`
- **MANDATORY**: Run `golangci-lint run` after EVERY task - it MUST pass with zero violations
- **MANDATORY**: Run `go test ./...` after EVERY task - all tests MUST pass
- **MANDATORY**: Follow Test-Driven Development (TDD) - write failing tests first, then implement

### Module Information
- **Package Path**: `github.com/input-output-hk/catalyst-forge-libs/earthly/lint`
- **Location**: `/earthly/lint/`
- **Dependency**: `github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile`

---

## Phase 1: Core Foundation

### Task 1: Define Core Types and Interfaces
- [ ] Create `types.go` with core types
  - **Success Criteria**:
    - `Severity` type defined (Error, Warning, Info)
    - `Issue` struct defined with all required fields
    - `SourceLocation` struct defined
    - `Fix` struct defined for auto-fixes
    - All types have proper godoc comments
    - Unit tests written for type methods (String(), validation)

### Task 2: Define Rule Interface and Context System
- [ ] Create `rule.go` with Rule interface
- [ ] Create `context.go` with LintContext implementation
  - **Success Criteria**:
    - `Rule` interface defined with Name(), Description(), Check() methods
    - `LintContext` struct with file/target/command/parent fields
    - Context hierarchy support (parent/child contexts)
    - Cache system for rule-specific data
    - Tests verify context creation and traversal

### Task 3: Implement Linter Engine
- [ ] Create `linter.go` with main Linter struct
  - **Success Criteria**:
    - `Linter` struct with rules slice
    - `Check()` method that processes an Earthfile
    - Issue aggregation and deduplication
    - Sort issues by location
    - Tests verify linter execution flow

---

## Phase 2: Rule System Implementation

### Task 4: Create Rule Builder Functions
- [ ] Create `builders.go` with helper functions
  - **Success Criteria**:
    - `SimpleRule()` function for basic rules
    - `CommandRule()` for command-specific rules
    - `PatternRule()` for regex-based rules
    - `RequireRule()` for requirement checks
    - `ForbidRule()` for forbidden patterns
    - Tests for each builder function

### Task 5: Implement Issue Reporting and Formatting
- [ ] Create `reporter.go` with output formatters
  - **Success Criteria**:
    - Text formatter (default)
    - JSON formatter
    - SARIF formatter
    - Configurable output streams
    - Tests verify all output formats

---

## Phase 3: Built-in Security Rules

### Task 6: Implement No-Sudo Rule
- [ ] Create `rules/security/no_sudo.go`
  - **Success Criteria**:
    - Detects sudo usage in RUN commands
    - Returns Warning severity issues
    - Tests with various sudo patterns

### Task 7: Implement No-Root-User Rule
- [ ] Create `rules/security/no_root_user.go`
  - **Success Criteria**:
    - Detects USER root commands
    - Returns Error severity issues
    - Tests for direct and indirect root usage

### Task 8: Implement No-Curl-Pipe Rule
- [ ] Create `rules/security/no_curl_pipe.go`
  - **Success Criteria**:
    - Detects curl | sh patterns
    - Detects wget | sh patterns
    - Returns Error severity issues
    - Tests for various pipe patterns

### Task 9: Implement No-Secrets Rule
- [ ] Create `rules/security/no_secrets.go`
  - **Success Criteria**:
    - Detects hardcoded API keys
    - Detects hardcoded passwords
    - Pattern matching for common secret formats
    - Returns Error severity issues
    - Tests with various secret patterns

---

## Phase 4: Best Practice Rules

### Task 10: Implement Require-Version Rule
- [ ] Create `rules/best_practice/require_version.go`
  - **Success Criteria**:
    - Checks VERSION command exists
    - Returns Error if missing
    - Tests for presence and absence

### Task 11: Implement No-Latest-Tags Rule
- [ ] Create `rules/best_practice/no_latest_tags.go`
  - **Success Criteria**:
    - Detects :latest tags in FROM commands
    - Detects :latest in BUILD commands
    - Returns Warning severity issues
    - Tests for various tag formats

### Task 12: Implement Require-From-First Rule
- [ ] Create `rules/best_practice/require_from_first.go`
  - **Success Criteria**:
    - Verifies FROM is first command in targets
    - Returns Warning if not first
    - Tests for various command orderings

### Task 13: Implement No-Empty-Targets Rule
- [ ] Create `rules/best_practice/no_empty_targets.go`
  - **Success Criteria**:
    - Detects targets without commands
    - Returns Warning severity issues
    - Tests for empty and non-empty targets

### Task 14: Implement Unique-Targets Rule
- [ ] Create `rules/best_practice/unique_targets.go`
  - **Success Criteria**:
    - Detects duplicate target names
    - Returns Error severity issues
    - Stateful rule tracking all targets
    - Tests with duplicate scenarios

---

## Phase 5: Style Rules

### Task 15: Implement Target-Naming Rule
- [ ] Create `rules/style/target_naming.go`
  - **Success Criteria**:
    - Enforces kebab-case for target names
    - Returns Info severity issues
    - Configurable pattern support
    - Tests for various naming patterns

### Task 16: Implement Max-Line-Length Rule
- [ ] Create `rules/style/max_line_length.go`
  - **Success Criteria**:
    - Checks lines under configured length (default 120)
    - Returns Info severity issues
    - Tests with various line lengths

---

## Phase 6: Configuration System

### Task 17: Implement Configuration Schema
- [ ] Create `config/config.go` with configuration types
  - **Success Criteria**:
    - Config struct with all fields
    - Severity override support
    - Disabled rules list
    - Rule-specific configuration
    - Tests for configuration merging

### Task 18: Implement Configuration Loading
- [ ] Create `config/loader.go` for loading configs
  - **Success Criteria**:
    - Load from `.earthlint.yaml`
    - Support multiple config levels
    - Default configuration
    - Tests for various config files

### Task 19: Implement Inline Directives
- [ ] Create `directives.go` for inline comments
  - **Success Criteria**:
    - Parse `earthlint:disable` comments
    - Parse `earthlint:disable-next-line`
    - Parse `earthlint:disable-file`
    - Tests for directive parsing

---

## Phase 7: Performance Optimization

### Task 20: Implement Caching System
- [ ] Create `cache.go` with caching infrastructure
  - **Success Criteria**:
    - Parse-once strategy
    - Command indexing by type
    - Pattern compilation cache
    - Result memoization
    - Benchmark tests showing improvement

### Task 21: Implement Parallel Rule Execution
- [ ] Update `linter.go` with parallel execution
  - **Success Criteria**:
    - Configurable worker pool
    - Thread-safe issue collection
    - No race conditions
    - Performance tests

---

## Phase 8: CLI and Integration

### Task 22: Create CLI Command
- [ ] Create `cmd/earthlint/main.go`
  - **Success Criteria**:
    - Parse command-line flags
    - Support format selection
    - Support config file override
    - Exit codes for CI integration
    - Integration tests

### Task 23: Add Rule Registration System
- [ ] Create `registry.go` for dynamic rule loading
  - **Success Criteria**:
    - Register built-in rules
    - Support custom rule registration
    - Rule discovery mechanism
    - Tests for registration

---

## Phase 9: Testing Infrastructure

### Task 24: Create Test Utilities
- [ ] Create `testutil/helpers.go`
  - **Success Criteria**:
    - Helper to parse Earthfile strings
    - Helper to create test contexts
    - Helper to assert issues
    - Golden file testing support

### Task 25: Add Integration Tests
- [ ] Create `integration_test.go`
  - **Success Criteria**:
    - Test all rules together
    - Test with real Earthfiles
    - Test configuration loading
    - Test CLI functionality
    - Performance benchmarks

---

## Phase 10: Documentation and Examples

### Task 26: Create Usage Examples
- [ ] Create `examples/` directory with sample configurations
  - **Success Criteria**:
    - Basic `.earthlint.yaml` example
    - Custom rule example
    - CI integration examples
    - Pre-commit hook setup

### Task 27: Generate Rule Documentation
- [ ] Create `docs/rules.md` with all built-in rules
  - **Success Criteria**:
    - Auto-generated from rule metadata
    - Examples for each rule
    - Configuration options documented
    - Severity levels explained

---

## Completion Checklist

### Final Validation
- [ ] All tests passing (`go test -race -cover ./...`)
- [ ] No golangci-lint violations
- [ ] 80%+ code coverage
- [ ] Benchmarks show acceptable performance
- [ ] Documentation complete
- [ ] Examples working
- [ ] CI integration tested

### Deliverables
- [ ] Fully functional earthlint module
- [ ] Comprehensive test suite
- [ ] Performance benchmarks
- [ ] Documentation and examples
- [ ] CI/CD integration guides