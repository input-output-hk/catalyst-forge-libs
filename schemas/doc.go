// Package schema provides CUE schema definitions and generated Go types for
// Catalyst Forge configuration files.
//
// This package includes:
//   - Embedded CUE module (CueModule) for runtime validation
//   - Generated Go types from CUE schemas (RepoConfig, ProjectConfig, etc.)
//   - Version management utilities for schema compatibility checking
//
// # CUE Module
//
// The CueModule variable embeds the complete CUE schema definitions at build time.
// It includes:
//   - repo.cue: Repository-level configuration schema
//   - project.cue: Project-level configuration schema
//   - common/: Common type definitions (secrets, etc.)
//   - publishers/: Publisher type definitions (Docker, GitHub, S3)
//   - phases/: Pipeline phase definitions
//   - artifacts/: Artifact and producer type definitions
//
// The cue library (Layer 2) uses CueModule for runtime validation of user configurations.
//
// # Generated Types
//
// Go types are generated from CUE schemas using:
//
//	//go:generate go run cuelang.org/go/cmd/cue@v0.12.0 exp gengotypes ./...
//
// Run `go generate ./...` to regenerate types after modifying CUE schemas.
//
// Generated files include:
//   - repo_gen.go: RepoConfig and related types
//   - project_gen.go: ProjectConfig and related types
//   - publishers/publishers_gen.go: Publisher types
//   - phases/phases_gen.go: Phase types
//   - artifacts/artifacts_gen.go: Artifact types
//
// # Usage Example
//
//	package main
//
//	import (
//	    "fmt"
//	    "log"
//
//	    "github.com/input-output-hk/catalyst-forge-libs/schema"
//	)
//
//	func main() {
//	    // Check version compatibility
//	    userVersion := "0.1.5"
//	    compatible, err := schema.IsCompatible(userVersion)
//	    if err != nil {
//	        log.Fatalf("Invalid version: %v", err)
//	    }
//	    if !compatible {
//	        log.Fatalf("Incompatible version: user has %s, requires compatible with %s",
//	            userVersion, schema.SchemaVersion)
//	    }
//
//	    // Access embedded CUE module (used by cue library for validation)
//	    _ = schema.CueModule
//	}
package schema

//go:generate go run cuelang.org/go/cmd/cue@v0.12.0 exp gengotypes ./...
