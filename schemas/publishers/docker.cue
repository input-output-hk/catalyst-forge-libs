package publishers

import "github.com/input-output-hk/catalyst-forge-libs/schema/common"

// DockerPublisher defines a Docker registry publisher configuration.
// This publisher pushes container images to a Docker registry.
// Discriminated by type!: "docker".
#DockerPublisher: {
	type!:        "docker"   // Required literal tag for discriminated union
	registry:     string     // Docker registry URL (e.g., "docker.io", "ghcr.io")
	namespace:    string     // Registry namespace (e.g., "myorg", "username")
	credentials?: common.#SecretRef // Optional credentials for registry authentication
}
