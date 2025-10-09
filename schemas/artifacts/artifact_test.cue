package artifacts

import "strings"

// Test: Valid ContainerArtifact
_validContainerArtifact: #ContainerArtifact & {
	type: "container"
	ref:  "myapp:v1.0.0"
	producer: {
		type:   "earthly"
		target: "+build-container"
	}
	publishers: ["docker-hub", "ghcr"]
}

// Test: Valid ContainerArtifact with artifact path
_validContainerArtifactWithPath: #ContainerArtifact & {
	type: "container"
	ref:  "myapp:latest"
	producer: {
		type:     "earthly"
		target:   "+build"
		artifact: "+build/image"
	}
	publishers: ["docker-hub"]
}

// Test: Valid BinaryArtifact
_validBinaryArtifact: #BinaryArtifact & {
	type: "binary"
	name: "myapp"
	producer: {
		type:   "earthly"
		target: "+build-binary"
	}
	publishers: ["github-releases", "s3-binaries"]
}

// Test: Valid ArchiveArtifact with default compression
_validArchiveArtifactDefault: #ArchiveArtifact & {
	type: "archive"
	producer: {
		type:   "earthly"
		target: "+package"
	}
	publishers: ["s3-archives"]
}

// Validate that compression defaults to "gzip"
_compressionDefaultTest: _validArchiveArtifactDefault.compression
_compressionDefaultTest: "gzip"

// Test: Valid ArchiveArtifact with explicit gzip
_validArchiveArtifactGzip: #ArchiveArtifact & {
	type:        "archive"
	compression: "gzip"
	producer: {
		type:   "earthly"
		target: "+package"
	}
	publishers: ["s3-archives"]
}

// Test: Valid ArchiveArtifact with zip
_validArchiveArtifactZip: #ArchiveArtifact & {
	type:        "archive"
	compression: "zip"
	producer: {
		type:   "earthly"
		target: "+package-zip"
	}
	publishers: ["s3-archives"]
}

// Test: ArtifactSpec union accepts ContainerArtifact
_unionAcceptsContainer: #ArtifactSpec & _validContainerArtifact

// Test: ArtifactSpec union accepts BinaryArtifact
_unionAcceptsBinary: #ArtifactSpec & _validBinaryArtifact

// Test: ArtifactSpec union accepts ArchiveArtifact
_unionAcceptsArchive: #ArtifactSpec & _validArchiveArtifactDefault

// Test: Empty publishers list is valid
_emptyPublishers: #ContainerArtifact & {
	type: "container"
	ref:  "myapp:test"
	producer: {
		type:   "earthly"
		target: "+build"
	}
	publishers: []
}
