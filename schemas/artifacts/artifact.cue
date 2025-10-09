package artifacts

// ContainerArtifact defines a container image artifact.
// This artifact type represents a container image that can be pushed to registries.
// Discriminated by type!: "container".
#ContainerArtifact: {
	type!:    "container"       // Required literal tag for discriminated union
	ref:      string            // Container image reference (e.g., "myapp:v1.0.0")
	producer: #ArtifactProducer // Producer that builds this artifact
	publishers: [...string] // References to publisher names from repo.cue
}

// BinaryArtifact defines a binary executable artifact.
// This artifact type represents a standalone binary executable.
// Discriminated by type!: "binary".
#BinaryArtifact: {
	type!:    "binary"          // Required literal tag for discriminated union
	name:     string            // Name of the binary (as produced by producer)
	producer: #ArtifactProducer // Producer that builds this artifact
	publishers: [...string] // References to publisher names from repo.cue
}

// ArchiveArtifact defines an archive artifact.
// This artifact type represents a compressed archive file.
// Discriminated by type!: "archive".
#ArchiveArtifact: {
	type!:       "archive"         // Required literal tag for discriminated union
	compression: *"gzip" | "zip"   // Compression format (default: gzip)
	producer:    #ArtifactProducer // Producer that builds this artifact
	publishers: [...string] // References to publisher names from repo.cue
}

// ArtifactSpec is a discriminated union of all artifact types.
// MVP: container, binary, archive
// Future: can extend with | #HelmChartArtifact | #TerraformModuleArtifact, etc.
#ArtifactSpec: #ContainerArtifact | #BinaryArtifact | #ArchiveArtifact
