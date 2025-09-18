// Package copy handles S3 object copy and move operations.
// This includes server-side copying between buckets and multipart copy
// operations for large objects.
//
// Copy operations are optimized to use S3's server-side copy when possible
// to minimize data transfer and improve performance.
package copy
