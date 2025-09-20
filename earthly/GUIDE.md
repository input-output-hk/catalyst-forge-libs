# Quick Guide: Using the Earthbuild AST Module

This guide provides concrete examples of using the Earthbuild AST module (`github.com/EarthBuild/earthbuild/ast`) to avoid common mistakes when implementing the wrapper module.

## Critical Information

- **Module Path**: `github.com/EarthBuild/earthbuild/ast`
- **Go Version Required**: 1.21+
- **Main Import**: Uses ANTLR4 for parsing (handled internally)

## Essential Imports

```go
import (
    "context"

    "github.com/EarthBuild/earthbuild/ast"
    "github.com/EarthBuild/earthbuild/ast/spec"
    "github.com/EarthBuild/earthbuild/ast/command"
    "github.com/EarthBuild/earthbuild/ast/commandflag"
)
```

## Core Parsing Functions

### Method 1: Parse from File Path

```go
ctx := context.Background()
ef, err := ast.Parse(ctx, filePath, enableSourceMap)
// ef is type spec.Earthfile
```

### Method 2: Parse with Options

```go
ctx := context.Background()
ef, err := ast.ParseOpts(ctx, ast.FromPath(filePath), opts...)
```

### Method 3: Parse from Reader

```go
// IMPORTANT: Reader must implement ast.NamedReader interface
type NamedReader interface {
    Name() string
    Seek(offset int64, whence int) (int64, error)
    Read(buff []byte) (n int, err error)
}

// Example implementation:
type myReader struct {
    *strings.Reader
    name string
}

func (r *myReader) Name() string { return r.name }

reader := &myReader{
    Reader: strings.NewReader(content),
    name:   "Earthfile",
}

ctx := context.Background()
ef, err := ast.ParseOpts(ctx, ast.FromReader(reader))
```

## AST Structure (spec.Earthfile)

```go
type Earthfile struct {
    Version        *Version        // Can be nil if no VERSION command
    BaseRecipe     Block          // Commands before first target
    Targets        []Target       // All targets in file
    Functions      []Function     // User-defined functions
    SourceLocation *SourceLocation // Only if enableSourceMap=true
}

type Target struct {
    Name           string
    Docs           string          // Comment docs before target
    Recipe         Block          // []Statement
    SourceLocation *SourceLocation
}

type Block []Statement

type Statement struct {
    // Only ONE of these will be non-nil per statement
    Command        *Command
    With           *WithStatement
    If             *IfStatement
    Try            *TryStatement
    For            *ForStatement
    Wait           *WaitStatement
    SourceLocation *SourceLocation
}

type Command struct {
    Name           string   // e.g., "RUN", "COPY", "FROM"
    Docs           string
    Args           []string // Raw arguments as strings
    ExecMode       bool     // True if in shell execution mode
    SourceLocation *SourceLocation
}
```

## Walking the AST

### Iterate Through Targets

```go
for _, target := range ef.Targets {
    fmt.Printf("Target: %s\n", target.Name)

    // Walk statements in target
    for _, stmt := range target.Recipe {
        if stmt.Command != nil {
            fmt.Printf("  Command: %s %v\n",
                stmt.Command.Name, stmt.Command.Args)
        }

        if stmt.If != nil {
            // Handle if statement
            fmt.Printf("  IF %v\n", stmt.If.Expression)
            // stmt.If.IfBody is another Block ([]Statement)
        }

        if stmt.For != nil {
            // Handle for loop
            fmt.Printf("  FOR %v\n", stmt.For.Args)
            // stmt.For.Body is another Block ([]Statement)
        }
    }
}
```

### Handle Nested Blocks

```go
func walkBlock(block spec.Block, depth int) {
    indent := strings.Repeat("  ", depth)

    for _, stmt := range block {
        if stmt.Command != nil {
            fmt.Printf("%sCommand: %s\n", indent, stmt.Command.Name)
        }

        if stmt.If != nil {
            fmt.Printf("%sIF %v\n", indent, stmt.If.Expression)
            walkBlock(stmt.If.IfBody, depth+1)

            for _, elseIf := range stmt.If.ElseIf {
                fmt.Printf("%sELSE IF %v\n", indent, elseIf.Expression)
                walkBlock(elseIf.Body, depth+1)
            }

            if stmt.If.ElseBody != nil {
                fmt.Printf("%sELSE\n", indent)
                walkBlock(*stmt.If.ElseBody, depth+1)
            }
        }

        if stmt.With != nil {
            fmt.Printf("%sWITH %s\n", indent, stmt.With.Command.Name)
            walkBlock(stmt.With.Body, depth+1)
        }

        // Similar for For, Try, Wait statements
    }
}
```

## Command Types

The `command` package defines command type constants:

```go
import "github.com/EarthBuild/earthbuild/ast/command"

// Command types (partial list)
const (
    AddCmd            Type = iota + 1
    ArgCmd
    BuildCmd
    CacheCmd
    CmdCmd
    CopyCmd
    DoCmd
    FromCmd
    FromDockerfileCmd
    GitCloneCmd
    RunCmd
    SaveArtifactCmd
    SaveImageCmd
    // ... and more
)
```

To check command type, compare the command name string:

```go
if stmt.Command != nil {
    switch stmt.Command.Name {
    case "FROM":
        // Handle FROM command
    case "RUN":
        // Handle RUN command
    case "COPY":
        // Handle COPY command
    case "BUILD":
        // Handle BUILD command
    case "SAVE ARTIFACT":
        // Handle SAVE ARTIFACT command
    case "SAVE IMAGE":
        // Handle SAVE IMAGE command
    }
}
```

## Version Parsing

### Parse Only VERSION Command (Lightweight)

```go
version, err := ast.ParseVersion(filePath, enableSourceMap)
// version is *spec.Version (can be nil if no VERSION)

if version != nil {
    // version.Args contains the arguments
    // Last arg is typically the version number
    versionStr := version.Args[len(version.Args)-1]
}
```

## Source Locations

When `enableSourceMap` is true, source locations are available:

```go
type SourceLocation struct {
    File        string
    StartLine   int    // 1-based line number
    StartColumn int    // 0-based column
    EndLine     int
    EndColumn   int
}

// Example usage
if stmt.Command != nil && stmt.Command.SourceLocation != nil {
    loc := stmt.Command.SourceLocation
    fmt.Printf("Command at %s:%d:%d\n",
        loc.File, loc.StartLine, loc.StartColumn)
}
```

## Common Patterns

### Find All Commands of a Type

```go
func findCommands(target *spec.Target, commandName string) []*spec.Command {
    var commands []*spec.Command

    for _, stmt := range target.Recipe {
        if stmt.Command != nil && stmt.Command.Name == commandName {
            commands = append(commands, stmt.Command)
        }
    }

    return commands
}

// Usage
runCommands := findCommands(target, "RUN")
```

### Extract FROM Base Image

```go
func getBaseImage(target *spec.Target) string {
    for _, stmt := range target.Recipe {
        if stmt.Command != nil && stmt.Command.Name == "FROM" {
            if len(stmt.Command.Args) > 0 {
                return stmt.Command.Args[0]
            }
        }
    }
    return ""
}
```

### Find BUILD Dependencies

```go
func getBuildDependencies(ef *spec.Earthfile) []string {
    var deps []string

    // Check base recipe
    for _, stmt := range ef.BaseRecipe {
        if stmt.Command != nil && stmt.Command.Name == "BUILD" {
            if len(stmt.Command.Args) > 0 {
                deps = append(deps, stmt.Command.Args[0])
            }
        }
    }

    // Check all targets
    for _, target := range ef.Targets {
        for _, stmt := range target.Recipe {
            if stmt.Command != nil && stmt.Command.Name == "BUILD" {
                if len(stmt.Command.Args) > 0 {
                    deps = append(deps, stmt.Command.Args[0])
                }
            }
        }
    }

    return deps
}
```

## Important Gotchas

### 1. Version Can Be Nil
```go
// ALWAYS check if Version is nil
if ef.Version != nil {
    version := ef.Version.Args[len(ef.Version.Args)-1]
}
```

### 2. SourceLocation Is Optional
```go
// Only available when enableSourceMap=true
if stmt.Command.SourceLocation != nil {
    // Safe to use location
}
```

### 3. Statement Has One Active Field
```go
// Only ONE of these will be non-nil
if stmt.Command != nil {
    // It's a command
} else if stmt.If != nil {
    // It's an if statement
} else if stmt.For != nil {
    // It's a for loop
}
// etc.
```

### 4. Command Names Include Spaces
```go
// Some commands have spaces in their names
"SAVE ARTIFACT"  // Not "SAVE" with "ARTIFACT" as first arg
"SAVE IMAGE"
"GIT CLONE"
"FROM DOCKERFILE"
```

### 5. Args Are Raw Strings
```go
// Command.Args are unparsed strings
// For "RUN echo hello world":
cmd.Name = "RUN"
cmd.Args = []string{"echo hello world"}  // Single string, not split

// For "COPY src/ dest/":
cmd.Name = "COPY"
cmd.Args = []string{"src/", "dest/"}  // Two separate args
```

### 6. ExecMode Is Important
```go
// ExecMode indicates shell execution context
if stmt.Command.ExecMode {
    // Command is in shell mode (e.g., inside RUN)
}
```

## Error Handling

The AST parser returns wrapped errors with context:

```go
ef, err := ast.Parse(ctx, filePath, true)
if err != nil {
    // Error will include file path and line/column if available
    // e.g., "Earthfile:10:5 unexpected token"
    return err
}
```

## Validation

The AST automatically validates:
- VERSION command format (must be "0.6", "0.7", or "0.8")
- No duplicate target names
- No targets named "base" (reserved)

These validations happen during parsing and return errors.

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/EarthBuild/earthbuild/ast"
    "github.com/EarthBuild/earthbuild/ast/spec"
)

func analyzeEarthfile(filePath string) error {
    ctx := context.Background()

    // Parse with source mapping enabled
    ef, err := ast.Parse(ctx, filePath, true)
    if err != nil {
        return fmt.Errorf("parse error: %w", err)
    }

    // Check version
    if ef.Version != nil {
        version := ef.Version.Args[len(ef.Version.Args)-1]
        fmt.Printf("Version: %s\n", version)
    }

    // Analyze targets
    for _, target := range ef.Targets {
        fmt.Printf("\nTarget: %s\n", target.Name)

        // Find base image
        for _, stmt := range target.Recipe {
            if stmt.Command != nil && stmt.Command.Name == "FROM" {
                if len(stmt.Command.Args) > 0 {
                    fmt.Printf("  Base: %s\n", stmt.Command.Args[0])
                }
                break
            }
        }

        // Count commands
        commandCount := 0
        for _, stmt := range target.Recipe {
            if stmt.Command != nil {
                commandCount++
            }
        }
        fmt.Printf("  Commands: %d\n", commandCount)
    }

    return nil
}
```

## Testing Your Implementation

When implementing the wrapper module, test against these cases:

1. **Empty Earthfile** - Should parse but have empty targets
2. **No VERSION** - ef.Version should be nil
3. **Complex nesting** - IF/FOR/WITH statements with nested commands
4. **Special commands** - "SAVE ARTIFACT", "SAVE IMAGE", etc.
5. **Global commands** - Commands in BaseRecipe before any target
6. **Functions** - User-defined functions in ef.Functions

## Need More?

Refer to these files in the Earthbuild repository:
- `ast/ast.go` - Main parsing functions
- `ast/spec/earthfile.go` - AST type definitions
- `ast/ast_test.go` - Examples of parsing various Earthfiles
- `ast/listener.go` - Internal walker implementation