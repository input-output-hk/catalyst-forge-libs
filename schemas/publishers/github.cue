package publishers

import "github.com/input-output-hk/catalyst-forge-libs/schema/common"

// GitHubPublisher defines a GitHub Releases publisher configuration.
// This publisher publishes artifacts to GitHub Releases.
// Discriminated by type!: "github".
#GitHubPublisher: {
	type!:        "github"   // Required literal tag for discriminated union
	repository:   string     // GitHub repository (e.g., "owner/repo")
	credentials?: common.#SecretRef // Optional credentials for GitHub authentication
}
