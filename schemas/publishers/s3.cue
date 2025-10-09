package publishers

import "github.com/input-output-hk/catalyst-forge-libs/schema/common"

// S3Publisher defines an AWS S3 bucket publisher configuration.
// This publisher uploads artifacts to an S3 bucket.
// Discriminated by type!: "s3".
#S3Publisher: {
	// Required literal tag for discriminated union
	type!: "s3"
	// S3 bucket name
	bucket: string
	// AWS region (optional, uses default if not specified)
	region?: string
	// Optional credentials for S3 authentication
	credentials?: common.#SecretRef
}
