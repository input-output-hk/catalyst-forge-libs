package artifacts

// EarthlyProducer defines an Earthly-based artifact producer.
// This producer uses Earthly targets to build artifacts.
// Discriminated by type!: "earthly".
#EarthlyProducer: {
	// Required literal tag for discriminated union
	type!: "earthly"
	// Earthly target to build (e.g., "+build")
	target: string
	// Artifact output reference (e.g., "+build/output")
	artifact?: string
}

// ArtifactProducer is a discriminated union of all producer types.
// MVP: earthly only
// Future: can extend with | #DockerProducer | #CustomProducer, etc.
#ArtifactProducer: #EarthlyProducer
