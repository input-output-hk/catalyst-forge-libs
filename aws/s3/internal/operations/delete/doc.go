// Package delete handles S3 object deletion operations.
// This includes single object deletion and batch deletion of multiple objects.
//
// Batch operations use S3's delete objects API to efficiently delete
// up to 1000 objects in a single request.
package delete
