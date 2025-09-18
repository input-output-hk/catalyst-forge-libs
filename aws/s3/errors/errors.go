// Package errors provides error types and handling for AWS S3 operations.
package errors

import (
	"errors"
	"fmt"
)

// Error represents an S3 operation error with context about the operation that failed.
// It wraps the underlying AWS SDK error with additional context for better debugging.
type Error struct {
	// Op is the operation that failed (e.g., "upload", "download", "delete")
	Op string

	// Bucket is the S3 bucket name (if applicable)
	Bucket string

	// Key is the S3 object key (if applicable)
	Key string

	// Err is the underlying error from the AWS SDK or other source
	Err error
}

// Error implements the error interface by providing a formatted error message.
func (e *Error) Error() string {
	if e.Bucket != "" && e.Key != "" {
		return fmt.Sprintf("s3.%s %s/%s: %v", e.Op, e.Bucket, e.Key, e.Err)
	}
	if e.Bucket != "" {
		return fmt.Sprintf("s3.%s bucket %s: %v", e.Op, e.Bucket, e.Err)
	}
	if e.Key != "" {
		return fmt.Sprintf("s3.%s object %s: %v", e.Op, e.Key, e.Err)
	}
	return fmt.Sprintf("s3.%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error for error chaining support.
func (e *Error) Unwrap() error {
	return e.Err
}

// WithBucket adds bucket context to an existing error.
func (e *Error) WithBucket(bucket string) *Error {
	e.Bucket = bucket
	return e
}

// WithKey adds object key context to an existing error.
func (e *Error) WithKey(key string) *Error {
	e.Key = key
	return e
}

// WithMessage wraps the underlying error with a custom message.
func (e *Error) WithMessage(message string) *Error {
	e.Err = fmt.Errorf("%s: %w", message, e.Err)
	return e
}

// NewError creates a new Error with the given operation and underlying error.
func NewError(op string, err error) *Error {
	return &Error{
		Op:  op,
		Err: err,
	}
}

// NewBucketError creates a new Error with bucket context.
func NewBucketError(op, bucket string, err error) *Error {
	return &Error{
		Op:     op,
		Bucket: bucket,
		Err:    err,
	}
}

// NewObjectError creates a new Error with bucket and key context.
func NewObjectError(op, bucket, key string, err error) *Error {
	return &Error{
		Op:     op,
		Bucket: bucket,
		Key:    key,
		Err:    err,
	}
}

// Sentinel errors for common S3 operation failures.
// These can be used with errors.Is() for error checking.
var (
	// ErrObjectNotFound indicates that the requested object does not exist
	ErrObjectNotFound = errors.New("s3: object not found")

	// ErrBucketNotFound indicates that the requested bucket does not exist
	ErrBucketNotFound = errors.New("s3: bucket not found")

	// ErrAccessDenied indicates that access to the resource is denied
	ErrAccessDenied = errors.New("s3: access denied")

	// ErrInvalidInput indicates that the provided input is invalid
	ErrInvalidInput = errors.New("s3: invalid input")

	// ErrBucketAlreadyExists indicates that the bucket already exists
	ErrBucketAlreadyExists = errors.New("s3: bucket already exists")

	// ErrBucketNotEmpty indicates that the bucket is not empty and cannot be deleted
	ErrBucketNotEmpty = errors.New("s3: bucket not empty")

	// ErrInvalidBucketName indicates that the bucket name is invalid
	ErrInvalidBucketName = errors.New("s3: invalid bucket name")

	// ErrInvalidObjectKey indicates that the object key is invalid
	ErrInvalidObjectKey = errors.New("s3: invalid object key")

	// ErrTooManyRequests indicates that the request rate is too high
	ErrTooManyRequests = errors.New("s3: too many requests")

	// ErrTimeout indicates that the operation timed out
	ErrTimeout = errors.New("s3: operation timeout")

	// ErrConnection indicates a connection error
	ErrConnection = errors.New("s3: connection error")

	// ErrChecksumMismatch indicates that checksums don't match
	ErrChecksumMismatch = errors.New("s3: checksum mismatch")

	// ErrInvalidRange indicates that the requested range is invalid
	ErrInvalidRange = errors.New("s3: invalid range")

	// ErrNotImplemented indicates that the requested feature is not implemented
	ErrNotImplemented = errors.New("s3: not implemented")

	// ErrInvalidCredentials indicates that the AWS credentials are invalid
	ErrInvalidCredentials = errors.New("s3: invalid credentials")

	// ErrRegionMismatch indicates that the bucket is in a different region
	ErrRegionMismatch = errors.New("s3: region mismatch")
)

// IsObjectNotFound checks if an error indicates that an object was not found.
// This is a convenience function that handles both sentinel errors and wrapped errors.
func IsObjectNotFound(err error) bool {
	return errors.Is(err, ErrObjectNotFound)
}

// IsBucketNotFound checks if an error indicates that a bucket was not found.
// This is a convenience function that handles both sentinel errors and wrapped errors.
func IsBucketNotFound(err error) bool {
	return errors.Is(err, ErrBucketNotFound)
}

// IsAccessDenied checks if an error indicates access was denied.
// This is a convenience function that handles both sentinel errors and wrapped errors.
func IsAccessDenied(err error) bool {
	return errors.Is(err, ErrAccessDenied)
}

// IsInvalidInput checks if an error indicates invalid input.
// This is a convenience function that handles both sentinel errors and wrapped errors.
func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}
