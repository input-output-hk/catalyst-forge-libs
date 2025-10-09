package publishers

import "github.com/input-output-hk/catalyst-forge-libs/schema/common"

// S3Publisher defines an AWS S3 bucket publisher configuration.
// This publisher uploads artifacts to an S3 bucket.
// Discriminated by type!: "s3".
#S3Publisher: {
	type!:        "s3"       // Required literal tag for discriminated union
	bucket:       string     // S3 bucket name
	region?:      string     // AWS region (optional, uses default if not specified)
	credentials?: common.#SecretRef // Optional credentials for S3 authentication
}
