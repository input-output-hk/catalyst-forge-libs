package schema

import "embed"

// CueModule contains the embedded CUE schema definitions.
// This embed.FS includes the entire CUE module directory structure:
//   - cue.mod/: CUE module configuration
//   - repo.cue: Repository configuration schema
//   - project.cue: Project configuration schema
//   - common/: Common type definitions (secrets, etc.)
//   - publishers/: Publisher type definitions (Docker, GitHub, S3)
//   - phases/: Pipeline phase definitions
//   - artifacts/: Artifact and producer type definitions
//
// The cue library (Layer 2) uses this embedded filesystem to load
// and validate user configurations at runtime.
//
//go:embed cue.mod repo.cue project.cue common publishers phases artifacts
var CueModule embed.FS
