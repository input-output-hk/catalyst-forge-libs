package artifacts

// EarthlyProducer defines an Earthly-based artifact producer.
// This producer uses Earthly targets to build artifacts.
// Discriminated by type!: "earthly".
#EarthlyProducer: {
	type!:     "earthly" // Required literal tag for discriminated union
	target:    string    // Earthly target to build (e.g., "+build")
	artifact?: string    // Artifact output reference (e.g., "+build/output")
}

// ArtifactProducer is a discriminated union of all producer types.
// MVP: earthly only
// Future: can extend with | #DockerProducer | #CustomProducer, etc.
#ArtifactProducer: #EarthlyProducer
