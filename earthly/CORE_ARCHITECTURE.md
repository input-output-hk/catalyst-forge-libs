# Earthfile Core Parser Module Architecture

## Overview

The `earthfile` module provides a high-level Go API for parsing and analyzing Earthfiles. It wraps the low-level Earthbuild AST parser with an ergonomic interface optimized for tooling development.

## Module Information

- **Package**: `github.com/yourdomain/earthfile`
- **Purpose**: Parse, navigate, and analyze Earthfiles
- **Dependencies**:
  - `github.com/EarthBuild/earthbuild/ast` - Core AST parser
  - `github.com/pkg/errors` - Error handling

## Architecture Principles

1. **Simplicity First**: Common operations should be simple; complex operations should be possible
2. **Type Safety**: Leverage Go's type system to prevent runtime errors
3. **Zero Allocation**: Minimize allocations by caching lookups and reusing structures
4. **Fail Fast**: Return errors early with clear, actionable messages
5. **AST Transparency**: Provide escape hatches to access underlying AST when needed

## Core Components

### 1. Parser Layer

Responsible for converting Earthfile text into structured data.

```
┌─────────────────┐
│   Earthfile     │
│     (text)      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  AST Parser     │ ◄── github.com/EarthBuild/earthbuild/ast
│   (ANTLR4)      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Wrapper Layer  │ ◄── This module
│  (earthfile)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Earthfile      │
│   struct        │
└─────────────────┘
```

### 2. Data Model

#### Primary Types

- **Earthfile**: Root container with indexed access to targets and functions
- **Target**: Build target with associated commands and metadata
- **Function**: User-defined function with reusable commands
- **Command**: Individual instruction with type, arguments, and position

#### Type Hierarchy

```
Earthfile
├── Version (string)
├── Targets (map[string]*Target)
│   └── Target
│       ├── Name (string)
│       ├── Docs (string)
│       └── Commands ([]*Command)
├── Functions (map[string]*Function)
│   └── Function
│       ├── Name (string)
│       └── Commands ([]*Command)
└── BaseCommands ([]*Command)
    └── Command
        ├── Name (string)
        ├── Type (CommandType)
        ├── Args ([]string)
        └── Location (SourceLocation)
```

### 3. Parser Functions

Multiple entry points for different use cases:

- `Parse(path)` - Parse from file path
- `ParseContext(ctx, path)` - Parse with cancellation support
- `ParseReader(reader, name)` - Parse from io.Reader
- `ParseString(content)` - Parse from string
- `ParseWithOptions(path, opts)` - Parse with custom configuration

### 4. Query Interface

Efficient lookups using pre-built indices:

```go
type Earthfile struct {
    // Pre-computed maps for O(1) lookups
    targets   map[string]*Target
    functions map[string]*Function

    // Cached computed properties
    dependencies []Dependency  // Lazy-loaded
    version     string         // Cached from AST
}
```

### 5. Traversal System

Two traversal patterns for different use cases:

#### Visitor Pattern (Complex Traversal)
```go
type Visitor interface {
    VisitTarget(*Target) error
    VisitFunction(*Function) error
    VisitCommand(*Command) error
    VisitIfStatement(condition []string, then, else []*Command) error
    // ... other visit methods
}
```

#### Callback Pattern (Simple Traversal)
```go
type WalkFunc func(*Command, depth int) error
```

## Key Algorithms

### 1. Dependency Resolution

```
1. Parse all BUILD, FROM, COPY commands
2. Extract target references
3. Classify as local vs remote
4. Build dependency graph
5. Cache results for repeated queries
```

### 2. Command Indexing

```
1. During parsing, classify commands by type
2. Build type -> []*Command map per target
3. Enable O(1) command type lookups
4. Support pattern matching within command args
```

### 3. Source Mapping

```
1. Preserve ANTLR token positions
2. Map AST nodes to file locations
3. Provide line:column for all elements
4. Enable accurate error reporting
```

## Error Handling

Errors are wrapped with context at each layer:

```
File I/O Error
└── Parse Error (with file path)
    └── AST Error (with line:column)
        └── Validation Error (with rule context)
```

## Performance Considerations

### Optimizations

1. **Lazy Loading**: Dependencies computed only when requested
2. **Index Caching**: Target/function maps built once during parse
3. **String Interning**: Command names use predefined constants
4. **Minimal Allocations**: Reuse visitor instances across walks

### Benchmarks (Target)

```
Parse small Earthfile (10 targets):     ~1ms
Parse large Earthfile (100 targets):    ~10ms
Parse huge Earthfile (1000 targets):    ~100ms
Target lookup (after parse):            ~100ns
Command type search:                    ~1µs
Full traversal (100 targets):          ~5ms
```

## Public API

### Core Functions
```go
// Parsing
func Parse(path string) (*Earthfile, error)
func ParseString(content string) (*Earthfile, error)
func ParseReader(r io.Reader, name string) (*Earthfile, error)

// Options
type ParseOptions struct {
    EnableSourceMap bool
    StrictMode      bool
}
```

### Earthfile Methods
```go
// Version info
Version() string
HasVersion() bool

// Target access
Target(name string) *Target
Targets() []*Target
TargetNames() []string
HasTarget(name string) bool

// Function access
Function(name string) *Function
Functions() []*Function

// Commands
BaseCommands() []*Command

// Analysis
Dependencies() []Dependency

// Traversal
Walk(Visitor) error
WalkCommands(WalkFunc) error

// Low-level access
AST() *spec.Earthfile
```

### Target Methods
```go
// Command queries
FindCommands(CommandType) []*Command
GetFromBase() *Command
GetArgs() []*Command
GetBuilds() []*Command
GetArtifacts() []*Command
GetImages() []*Command
HasCommand(CommandType) bool

// Traversal
Walk(WalkFunc) error
```

### Command Methods
```go
// Argument parsing
GetFlag(name string) (string, bool)
GetPositionalArgs() []string

// Reference analysis
IsRemoteReference() bool
GetReference() (*Reference, error)

// Source info
SourceLocation() SourceLocation
```

## Extension Points

The module is designed for extension:

1. **Custom Visitors**: Implement Visitor interface for custom traversals
2. **Command Processors**: Build specialized command analyzers
3. **AST Access**: Use AST() method for advanced operations
4. **Metadata Extraction**: Add custom methods via embedding

## Usage Examples

### Basic Parsing
```go
ef, err := earthfile.Parse("./Earthfile")
if err != nil {
    return err
}
fmt.Printf("Version: %s\n", ef.Version())
fmt.Printf("Targets: %v\n", ef.TargetNames())
```

### Dependency Analysis
```go
deps := ef.Dependencies()
for _, dep := range deps {
    if dep.Local {
        fmt.Printf("Local: %s\n", dep.Target)
    } else {
        fmt.Printf("Remote: %s\n", dep.Target)
    }
}
```

### Custom Visitor
```go
type ImageCollector struct {
    Images []string
}

func (ic *ImageCollector) VisitCommand(cmd *Command) error {
    if cmd.Type == CommandSaveImage {
        ic.Images = append(ic.Images, cmd.Args[0])
    }
    return nil
}

collector := &ImageCollector{}
ef.Walk(collector)
fmt.Printf("Images: %v\n", collector.Images)
```

## Testing Strategy

1. **Unit Tests**: Each component tested in isolation
2. **Integration Tests**: Full parse -> query workflows
3. **Fuzzing**: Random Earthfile generation for parser robustness
4. **Benchmarks**: Performance regression tests
5. **Example Tests**: Validate all documentation examples

## Future Enhancements

1. **Earthfile Builder**: Programmatically construct Earthfiles
2. **Diff Support**: Compare two Earthfiles structurally
3. **Partial Parsing**: Parse only specific targets for performance
4. **Streaming Parser**: Handle very large Earthfiles
5. **Schema Validation**: Validate against Earthfile schema versions

## Compatibility

- **Go Version**: 1.21+
- **Earthfile Versions**: 0.6, 0.7, 0.8
- **Platforms**: All platforms supported by Go

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

[License details here]