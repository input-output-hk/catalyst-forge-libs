package artifacts

// ContainerArtifact defines a container image artifact.
// This artifact type represents a container image that can be pushed to registries.
// Discriminated by type!: "container".
#ContainerArtifact: {
	// Required literal tag for discriminated union
	type!: "container"
	// Container image reference (e.g., "myapp:v1.0.0")
	ref: string
	// Producer that builds this artifact
	producer: #ArtifactProducer
	// References to publisher names from repo.cue
	publishers: [...string]
}

// BinaryArtifact defines a binary executable artifact.
// This artifact type represents a standalone binary executable.
// Discriminated by type!: "binary".
#BinaryArtifact: {
	// Required literal tag for discriminated union
	type!: "binary"
	// Name of the binary (as produced by producer)
	name: string
	// Producer that builds this artifact
	producer: #ArtifactProducer
	// References to publisher names from repo.cue
	publishers: [...string]
}

// ArchiveArtifact defines an archive artifact.
// This artifact type represents a compressed archive file.
// Discriminated by type!: "archive".
#ArchiveArtifact: {
	// Required literal tag for discriminated union
	type!: "archive"
	// Compression format (default: gzip)
	compression: *"gzip" | "zip"
	// Producer that builds this artifact
	producer: #ArtifactProducer
	// References to publisher names from repo.cue
	publishers: [...string]
}

// ArtifactSpec is a discriminated union of all artifact types.
// MVP: container, binary, archive
// Future: can extend with | #HelmChartArtifact | #TerraformModuleArtifact, etc.
#ArtifactSpec: #ContainerArtifact | #BinaryArtifact | #ArchiveArtifact
