package publishers

import "github.com/input-output-hk/catalyst-forge-libs/schema/common"

// GitHubPublisher defines a GitHub Releases publisher configuration.
// This publisher publishes artifacts to GitHub Releases.
// Discriminated by type!: "github".
#GitHubPublisher: {
	// Required literal tag for discriminated union
	type!: "github"
	// GitHub repository (e.g., "owner/repo")
	repository: string
	// Optional credentials for GitHub authentication
	credentials?: common.#SecretRef
}
