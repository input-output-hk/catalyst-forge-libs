package publishers

import "github.com/input-output-hk/catalyst-forge-libs/schema/common"

// DockerPublisher defines a Docker registry publisher configuration.
// This publisher pushes container images to a Docker registry.
// Discriminated by type!: "docker".
#DockerPublisher: {
	// Required literal tag for discriminated union
	type!: "docker"
	// Docker registry URL (e.g., "docker.io", "ghcr.io")
	registry: string
	// Registry namespace (e.g., "myorg", "username")
	namespace: string
	// Optional credentials for registry authentication
	credentials?: common.#SecretRef
}
