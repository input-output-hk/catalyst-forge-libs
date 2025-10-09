# Schema Package

[![Go Reference](https://pkg.go.dev/badge/github.com/input-output-hk/catalyst-forge-libs/schema.svg)](https://pkg.go.dev/github.com/input-output-hk/catalyst-forge-libs/schema)

CUE schema definitions and generated Go types for Catalyst Forge configuration files.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Schema Structure](#schema-structure)
  - [Repository Configuration](#repository-configuration-repoconfig)
  - [Project Configuration](#project-configuration-projectconfig)
  - [Phases](#phases)
  - [Publishers](#publishers)
  - [Artifacts](#artifacts)
  - [Secret References](#secret-references)
- [Working with Discriminated Unions](#working-with-discriminated-unions)
  - [Publisher Helpers](#publisher-helpers)
  - [Artifact Helpers](#artifact-helpers)
  - [Secret Reference Helpers](#secret-reference-helpers)
- [Version Compatibility](#version-compatibility)
- [Embedded CUE Module](#embedded-cue-module)
- [Regenerating Types](#regenerating-types)
- [Example Usage](#example-usage)
- [Package Structure](#package-structure)
- [Development](#development)
  - [Running Tests](#running-tests)
  - [Linting](#linting)
  - [Building](#building)
- [Contributing](#contributing)
- [License](#license)
- [Related Packages](#related-packages)
- [References](#references)

## Overview

The `schema` package is Layer 1 in the Catalyst Forge architecture, providing the foundation for configuration validation and type safety. It includes:

- **CUE Schema Definitions**: Declarative schemas for repository and project configurations
- **Generated Go Types**: Strongly-typed Go structs auto-generated from CUE schemas
- **Embedded CUE Module**: Complete CUE schema embedded at build time for runtime validation
- **Version Management**: Semantic versioning compatibility checking
- **Helper Functions**: Type-safe methods for working with discriminated unions

## Installation

```bash
go get github.com/input-output-hk/catalyst-forge-libs/schema
```

## Schema Structure

### Repository Configuration (`RepoConfig`)

Repository-level configuration defines settings that apply to an entire repository:

```go
type RepoConfig struct {
    ForgeVersion string                           // Schema version (e.g., "0.1.0")
    Tagging      TaggingStrategy                  // Git tagging strategy
    Phases       map[string]PhaseDefinition       // Phase definitions
    Publishers   map[string]PublisherConfig       // Publisher configurations
}
```

**Tagging Strategies:**
- `monorepo`: Individual tags per project
- `tag-all`: Single tag for entire repository

### Project Configuration (`ProjectConfig`)

Project-level configuration defines settings for individual projects within a repository:

```go
type ProjectConfig struct {
    Name      string                          // Project name
    Phases    map[string]PhaseParticipation   // Phase participation
    Artifacts map[string]ArtifactSpec         // Artifact specifications
    Release   *ReleaseConfig                  // Optional release configuration
    Deploy    *DeploymentConfig               // Optional deployment configuration
}
```

### Phases

Phases define pipeline stages with execution groups:

```go
type PhaseDefinition struct {
    Group       int     // Execution group (same group = parallel execution)
    Description string  // Optional description
    Timeout     string  // Optional timeout (e.g., "30m", "1h")
    Required    bool    // Whether phase is required
}
```

**Phase participation** defines the steps a project executes during a phase:

```go
type PhaseParticipation struct {
    Steps []Step  // List of steps to execute
}
```

Currently, only Earthly steps are supported:

```go
type Step struct {
    Name    string  // Step name
    Action  string  // "earthly"
    Target  string  // Earthly target (e.g., "+test")
    Timeout string  // Optional timeout
}
```

### Publishers

Publishers define where artifacts are published. The package supports three publisher types:

#### Docker Publisher

```go
type DockerPublisher struct {
    Type        string      // "docker"
    Registry    string      // Registry URL (e.g., "docker.io", "ghcr.io")
    Namespace   string      // Registry namespace
    Credentials *SecretRef  // Optional credentials
}
```

#### GitHub Publisher

```go
type GitHubPublisher struct {
    Type        string      // "github"
    Repository  string      // GitHub repository (e.g., "owner/repo")
    Credentials *SecretRef  // Optional credentials
}
```

#### S3 Publisher

```go
type S3Publisher struct {
    Type        string      // "s3"
    Bucket      string      // S3 bucket name
    Region      string      // AWS region
    Credentials *SecretRef  // Optional credentials
}
```

### Artifacts

Artifacts define build outputs. The package supports three artifact types:

#### Container Artifact

```go
type ContainerArtifact struct {
    Type       string           // "container"
    Ref        string           // Image reference (e.g., "myapp:v1.0.0")
    Producer   ArtifactProducer // Build producer
    Publishers []string         // Publisher references
}
```

#### Binary Artifact

```go
type BinaryArtifact struct {
    Type       string           // "binary"
    Name       string           // Binary name
    Producer   ArtifactProducer // Build producer
    Publishers []string         // Publisher references
}
```

#### Archive Artifact

```go
type ArchiveArtifact struct {
    Type        string           // "archive"
    Compression string           // "gzip" or "zip"
    Producer    ArtifactProducer // Build producer
    Publishers  []string         // Publisher references
}
```

Currently, only Earthly producers are supported:

```go
type ArtifactProducer struct {
    Type     string  // "earthly"
    Target   string  // Earthly target (e.g., "+build")
    Artifact string  // Optional artifact path (e.g., "+build/output")
}
```

### Secret References

Secret references support multiple providers:

#### AWS Secrets Manager

```go
type AWSSecretRef struct {
    Provider string  // "aws"
    Name     string  // ARN or secret name
    Key      string  // Optional key within secret
    Region   string  // Optional AWS region
}
```

#### HashiCorp Vault

```go
type VaultSecretRef struct {
    Provider string  // "vault"
    Path     string  // Secret path (e.g., "secret/data/myapp/api-key")
    Key      string  // Optional key (for KV v2)
}
```

## Working with Discriminated Unions

Several types use discriminated unions (tagged unions) that are represented as `map[string]any` in Go. The package provides helper functions for type-safe access to these unions.

### Publisher Helpers

```go
import "github.com/input-output-hk/catalyst-forge-libs/schema/publishers"

var config publishers.PublisherConfig
// ... unmarshal from JSON/YAML ...

// Get the discriminator value
typ := config.Type()  // Returns "docker", "github", "s3", or ""

// Type assertion with type safety
if docker, ok := config.AsDocker(); ok {
    fmt.Printf("Registry: %s/%s\n", docker.Registry, docker.Namespace)
}

// Or use a switch statement
switch config.Type() {
case "docker":
    docker, _ := config.AsDocker()
    // Work with docker publisher
case "github":
    github, _ := config.AsGitHub()
    // Work with GitHub publisher
case "s3":
    s3, _ := config.AsS3()
    // Work with S3 publisher
}

// Validate the discriminator
if err := config.Validate(); err != nil {
    log.Fatal(err)  // Invalid or missing type
}
```

### Artifact Helpers

```go
import "github.com/input-output-hk/catalyst-forge-libs/schema/artifacts"

var spec artifacts.ArtifactSpec
// ... unmarshal from JSON/YAML ...

// Get the discriminator value
typ := spec.Type()  // Returns "container", "binary", "archive", or ""

// Type assertion
if container, ok := spec.AsContainer(); ok {
    fmt.Printf("Image: %s\n", container.Ref)
    fmt.Printf("Publishers: %v\n", container.Publishers)
}

// Validate the discriminator
if err := spec.Validate(); err != nil {
    log.Fatal(err)
}
```

### Secret Reference Helpers

```go
import "github.com/input-output-hk/catalyst-forge-libs/schema/common"

var ref common.SecretRef
// ... unmarshal from JSON/YAML ...

// Get the discriminator value
provider := ref.Provider()  // Returns "aws", "vault", or ""

// Type assertion
if aws, ok := ref.AsAWS(); ok {
    fmt.Printf("Secret: %s in %s\n", aws.Name, aws.Region)
}

if vault, ok := ref.AsVault(); ok {
    fmt.Printf("Path: %s\n", vault.Path)
}

// Validate the discriminator
if err := ref.Validate(); err != nil {
    log.Fatal(err)
}
```

## Version Compatibility

The package provides semantic version compatibility checking:

```go
import "github.com/input-output-hk/catalyst-forge-libs/schema"

userVersion := "0.1.5"
compatible, err := schema.IsCompatible(userVersion)
if err != nil {
    log.Fatalf("Invalid version: %v", err)
}
if !compatible {
    log.Fatalf("Incompatible version: user has %s, requires compatible with %s",
        userVersion, schema.SchemaVersion)
}
```

**Compatibility Rules:**

For version `0.x.y`, the caret constraint (`^`) allows only patch version changes:
- ✅ `0.1.0` → `0.1.1` (patch bump)
- ✅ `0.1.0` → `0.1.5` (patch bump)
- ❌ `0.1.0` → `0.2.0` (minor bump - potentially breaking)
- ❌ `0.1.0` → `1.0.0` (major bump - breaking)

Current schema version: **`0.1.0`**

## Embedded CUE Module

The package embeds the complete CUE module at build time for use by validation libraries:

```go
import "github.com/input-output-hk/catalyst-forge-libs/schema"

// Access the embedded CUE filesystem
_ = schema.CueModule  // embed.FS containing all CUE schemas
```

The embedded module includes:
- `cue.mod/`: CUE module configuration
- `repo.cue`: Repository configuration schema
- `project.cue`: Project configuration schema
- `common/`: Common type definitions
- `publishers/`: Publisher type definitions
- `phases/`: Phase definitions
- `artifacts/`: Artifact and producer definitions

## Regenerating Types

The Go types are generated from CUE schemas. To regenerate after modifying CUE schemas:

```bash
go generate ./...
```

This runs:
```go
//go:generate go run cuelang.org/go/cmd/cue@v0.12.0 exp gengotypes ./...
```

**Note:** Helper functions in `*_helpers.go` files are hand-written and will not be overwritten during generation.

## Example Usage

### Complete Configuration Example

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/input-output-hk/catalyst-forge-libs/schema"
    "github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

func main() {
    // Parse a repository configuration
    jsonData := `{
        "forgeVersion": "0.1.0",
        "tagging": {"strategy": "monorepo"},
        "phases": {
            "test": {
                "group": 1,
                "description": "Run tests",
                "required": true
            }
        },
        "publishers": {
            "dockerhub": {
                "type": "docker",
                "registry": "docker.io",
                "namespace": "myorg"
            }
        }
    }`

    var config schema.RepoConfig
    if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
        log.Fatal(err)
    }

    // Check version compatibility
    compatible, err := schema.IsCompatible(config.ForgeVersion)
    if err != nil {
        log.Fatalf("Invalid version: %v", err)
    }
    if !compatible {
        log.Fatalf("Incompatible schema version: %s", config.ForgeVersion)
    }

    // Access publisher configuration using helpers
    if pubConfig, exists := config.Publishers["dockerhub"]; exists {
        if docker, ok := pubConfig.AsDocker(); ok {
            fmt.Printf("Docker registry: %s/%s\n",
                docker.Registry, docker.Namespace)
        }
    }
}
```

## Package Structure

```
schema/
├── README.md                    # This file
├── doc.go                       # Package documentation
├── go.mod                       # Go module definition
├── embed.go                     # CUE module embedding
├── version.go                   # Version compatibility checking
├── cue_types_gen.go             # Generated types (repo, project, etc.)
├── repo.cue                     # Repository schema
├── project.cue                  # Project schema
├── artifacts/
│   ├── artifact.cue             # Artifact schemas
│   ├── producer.cue             # Producer schemas
│   ├── cue_types_gen.go         # Generated artifact types
│   ├── helpers.go               # Artifact helper functions
│   └── helpers_test.go          # Helper tests
├── common/
│   ├── secret.cue               # Secret reference schemas
│   ├── cue_types_gen.go         # Generated common types
│   ├── helpers.go               # SecretRef helper functions
│   └── helpers_test.go          # Helper tests
├── phases/
│   ├── phase.cue                # Phase schemas
│   └── cue_types_gen.go         # Generated phase types
└── publishers/
    ├── publisher.cue            # Publisher union schema
    ├── docker.cue               # Docker publisher schema
    ├── github.cue               # GitHub publisher schema
    ├── s3.cue                   # S3 publisher schema
    ├── cue_types_gen.go         # Generated publisher types
    ├── helpers.go               # Publisher helper functions
    └── helpers_test.go          # Helper tests
```

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run tests for a specific package
go test ./publishers -v
```

### Linting

```bash
# Run linter
golangci-lint run ./...

# Auto-fix issues
golangci-lint run ./... --fix
```

### Building

```bash
# Build all packages
go build ./...
```

## Contributing

When adding new schema types or modifying existing ones:

1. Update the relevant `.cue` files
2. Run `go generate ./...` to regenerate Go types
3. Add helper functions if the new type is a discriminated union
4. Add comprehensive tests for helpers
5. Update this README with the new types and examples
6. Run tests and linting to ensure everything passes

## License

See the repository root for license information.

## Related Packages

- **Layer 2**: `cue` package - CUE validation and loading
- **Layer 3**: `domain` package - Domain models and business logic

## References

- [CUE Language](https://cuelang.org/)
- [Semantic Versioning](https://semver.org/)
- [Catalyst Forge Documentation](https://github.com/input-output-hk/catalyst-forge)
