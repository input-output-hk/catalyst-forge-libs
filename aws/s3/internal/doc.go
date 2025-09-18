// Package internal contains private implementation details for the S3 module.
// These packages are not intended for external use and may change without notice.
//
// The internal packages are organized as follows:
//   - operations: Core S3 operation implementations
//   - transfer: Complex transfer management (multipart, concurrent)
//   - sync: Directory synchronization functionality
//   - validation: Input validation logic
//   - pool: Memory management optimizations
package internal
