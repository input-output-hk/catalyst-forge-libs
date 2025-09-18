// Package transfer manages complex S3 transfer operations.
// This includes multipart upload/download coordination, progress tracking,
// and concurrency management.
//
// The transfer package orchestrates high-level transfer operations and
// delegates to specific operation packages for the actual AWS SDK calls.
package transfer
