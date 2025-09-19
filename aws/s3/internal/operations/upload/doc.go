// Package upload handles S3 object upload operations.
// This includes simple uploads, multipart uploads, and stream-based uploads.
//
// The package automatically detects when to use multipart upload based on
// file size thresholds and handles concurrent part uploads for optimal performance.
package upload
