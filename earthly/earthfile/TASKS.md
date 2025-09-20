# Earthfile Parser Module Implementation Tasks

## Preamble

This implementation SHALL conform to:
- **Go Style Guide** (`docs/guides/go/style.md`): All code must follow the established Go coding standards
- **The Constitution** (`docs/CONSTITUTION.md`): Test-first development, single responsibility, fail fast principles
- **Linting**: `golangci-lint run` must pass after EVERY task completion
- **Testing**: Write failing tests BEFORE implementation (TDD)

## Implementation Tasks

### Phase 1: Foundation Setup

#### Task 1: Module Structure and Dependencies
- [x] Create module directory structure (`earthfile/`)
- [x] Initialize go.mod with required dependencies
- [x] Add github.com/EarthBuild/earthbuild/ast dependency
- [x] Create internal package directories
- [x] Run `golangci-lint run` and ensure it passes
**Success Criteria**: Module compiles, dependencies resolve, linting passes

#### Task 2: Core Type Definitions
- [x] Write failing tests for core types (TestEarthfileStruct, TestTargetStruct, etc.)
- [x] Define Earthfile struct with maps for targets/functions
- [x] Define Target struct with name, docs, commands
- [x] Define Function struct
- [x] Define Command struct with type, args, location
- [x] Define CommandType enumeration
- [x] Define SourceLocation struct
- [x] Run tests (should pass) and `golangci-lint run`
**Success Criteria**: All core types defined, tests pass, no lint errors

### Phase 2: Parser Implementation

#### Task 3: Basic Parser Functions
- [x] Write failing tests for Parse, ParseString, ParseReader functions
- [x] Implement Parse(path) function wrapping ast.Parse
- [x] Implement ParseString(content) using NamedReader
- [x] Implement ParseReader(reader, name)
- [x] Implement ParseContext(ctx, path) for cancellation support
- [x] Handle error wrapping with fmt.Errorf and %w verb
- [x] Run tests and `golangci-lint run`
**Success Criteria**: All parser entry points work, proper error handling with stdlib, tests pass

#### Task 4: ParseOptions Support
- [x] Write failing tests for ParseWithOptions
- [x] Define ParseOptions struct (EnableSourceMap, StrictMode)
- [x] Implement ParseWithOptions(path, opts)
- [x] Implement option propagation to AST parser
- [x] Test source map enable/disable scenarios
- [x] Run `golangci-lint run`
**Success Criteria**: Options correctly passed to AST parser, tests pass

#### Task 5: AST to Domain Model Conversion
- [x] Write failing tests for AST conversion
- [x] Implement conversion from spec.Earthfile to domain Earthfile
- [x] Build target map during conversion for O(1) lookups
- [x] Build function map during conversion
- [x] Convert spec.Block to Command slices
- [x] Handle nested statements (IF, FOR, WITH, etc.)
- [x] Preserve source locations when enabled
- [x] Run tests and `golangci-lint run`
**Success Criteria**: Complete AST transformation, indices built, tests pass

### Phase 3: Query Interface

#### Task 6: Earthfile Methods - Basic Access
- [x] Write failing tests for Version(), HasVersion()
- [x] Write failing tests for Target(), Targets(), TargetNames(), HasTarget()
- [x] Write failing tests for Function(), Functions()
- [x] Write failing tests for BaseCommands()
- [x] Implement all accessor methods
- [x] Ensure O(1) lookups using pre-built maps
- [x] Run tests and `golangci-lint run`
**Success Criteria**: All query methods work with O(1) performance where applicable

#### Task 7: Target Query Methods
- [x] Write failing tests for FindCommands(CommandType)
- [x] Write failing tests for GetFromBase(), GetArgs(), GetBuilds()
- [x] Write failing tests for GetArtifacts(), GetImages()
- [x] Write failing tests for HasCommand(CommandType)
- [x] Implement command filtering by type
- [x] Cache command type lookups for performance
- [x] Run tests and `golangci-lint run`
**Success Criteria**: Target queries work correctly, command filtering accurate

#### Task 8: Command Analysis Methods
- [x] Write failing tests for GetFlag(name), GetPositionalArgs()
- [x] Write failing tests for IsRemoteReference(), GetReference()
- [x] Implement argument parsing logic
- [x] Implement reference detection (local vs remote)
- [x] Define Reference struct if needed
- [x] Run tests and `golangci-lint run`
**Success Criteria**: Command arguments parsed correctly, references identified

### Phase 4: Traversal System

#### Task 9: Visitor Pattern Implementation
- [ ] Write failing tests for Visitor interface
- [ ] Define Visitor interface with all Visit methods
- [ ] Implement Walk(Visitor) on Earthfile
- [ ] Implement Walk(Visitor) on Target
- [ ] Handle nested statement traversal (IF/FOR/WITH blocks)
- [ ] Test visitor receives all nodes in correct order
- [ ] Run tests and `golangci-lint run`
**Success Criteria**: Visitor pattern traverses entire AST correctly

#### Task 10: Callback Pattern Implementation
- [ ] Write failing tests for WalkCommands
- [ ] Define WalkFunc type signature
- [ ] Implement WalkCommands(WalkFunc) on Earthfile
- [ ] Implement Walk(WalkFunc) on Target
- [ ] Support depth tracking in callbacks
- [ ] Handle early termination via error return
- [ ] Run tests and `golangci-lint run`
**Success Criteria**: Simple traversal works, depth tracked, early exit supported

### Phase 5: Analysis Features

#### Task 11: Dependency Resolution
- [ ] Write failing tests for Dependencies() method
- [ ] Define Dependency struct (Target, Local, Source fields)
- [ ] Parse BUILD, FROM, COPY commands for dependencies
- [ ] Classify dependencies as local vs remote
- [ ] Implement lazy loading with caching
- [ ] Build dependency graph structure
- [ ] Run tests and `golangci-lint run`
**Success Criteria**: All dependencies identified and classified correctly

#### Task 12: Version Parsing Optimization
- [ ] Write failing tests for lightweight version parsing
- [ ] Implement ParseVersion(path) for VERSION-only parsing
- [ ] Validate version format (0.6, 0.7, 0.8)
- [ ] Handle missing VERSION gracefully (return nil)
- [ ] Run tests and `golangci-lint run`
**Success Criteria**: Fast version extraction without full parse

### Phase 6: Error Handling & Validation

#### Task 13: Comprehensive Error Handling
- [ ] Write tests for parse error scenarios
- [ ] Test invalid Earthfile syntax handling
- [ ] Test file not found errors
- [ ] Test reader errors
- [ ] Ensure errors include file:line:column when available
- [ ] Validate error wrapping maintains context chain
- [ ] Run tests and `golangci-lint run`
**Success Criteria**: All errors properly wrapped with context, actionable messages

#### Task 14: Validation Rules
- [ ] Write failing tests for validation rules
- [ ] Validate no duplicate target names
- [ ] Validate no targets named "base" (reserved)
- [ ] Validate VERSION format if present
- [ ] Return clear validation errors
- [ ] Run tests and `golangci-lint run`
**Success Criteria**: All validation rules enforced, clear error messages

### Phase 7: Performance & Optimization

#### Task 15: Performance Optimizations
- [ ] Write benchmarks for Parse operations
- [ ] Write benchmarks for Target lookups
- [ ] Write benchmarks for traversal operations
- [ ] Implement string interning for command names
- [ ] Minimize allocations in hot paths
- [ ] Reuse visitor instances where possible
- [ ] Validate performance targets met (see CORE_ARCHITECTURE.md benchmarks)
- [ ] Run benchmarks and `golangci-lint run`
**Success Criteria**: Meet or exceed target benchmarks, minimal allocations

#### Task 16: AST Access & Extension Points
- [ ] Write tests for AST() method
- [ ] Implement AST() to return underlying spec.Earthfile
- [ ] Document extension patterns in comments
- [ ] Create example custom visitor in tests
- [ ] Create example command processor in tests
- [ ] Run tests and `golangci-lint run`
**Success Criteria**: Raw AST accessible, extension examples work

### Phase 8: Integration & Documentation

#### Task 17: Integration Tests
- [ ] Create test Earthfiles covering all features
- [ ] Test empty Earthfile parsing
- [ ] Test complex nesting scenarios
- [ ] Test special commands (SAVE ARTIFACT, etc.)
- [ ] Test global commands in BaseRecipe
- [ ] Test user-defined functions
- [ ] Test against real-world Earthfiles
- [ ] Run all tests and `golangci-lint run`
**Success Criteria**: All integration scenarios pass, edge cases handled

#### Task 18: Example Programs
- [ ] Create basic parsing example
- [ ] Create dependency analysis example
- [ ] Create custom visitor example
- [ ] Create image collector example
- [ ] Validate all examples compile and run
- [ ] Run `golangci-lint run` on examples
**Success Criteria**: All examples from CORE_ARCHITECTURE.md work

#### Task 19: Fuzzing & Robustness
- [ ] Set up fuzzing infrastructure
- [ ] Create Earthfile generator for fuzzing
- [ ] Run fuzzing for parser robustness
- [ ] Fix any panics or crashes found
- [ ] Add regression tests for fuzzing findings
- [ ] Run tests and `golangci-lint run`
**Success Criteria**: No panics on malformed input, graceful error handling

#### Task 20: Final Validation
- [ ] Verify all public API matches CORE_ARCHITECTURE.md
- [ ] Confirm all benchmarks meet targets
- [ ] Validate Go 1.21+ compatibility
- [ ] Ensure 100% test coverage of public API
- [ ] Run full test suite
- [ ] Run `golangci-lint run` on entire module
- [ ] Create module README with usage instructions
**Success Criteria**: Module ready for production use, all requirements met

## Notes

- Each task must have tests written FIRST (TDD)
- Run `golangci-lint run` after EVERY task
- Commit after each completed task
- If any task reveals issues in previous tasks, fix immediately
- Performance benchmarks should be run regularly to catch regressions