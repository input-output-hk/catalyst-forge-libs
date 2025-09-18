// Package s3api defines interfaces for S3 operations to enable testing and mocking.
package s3api

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3API defines the interface for S3 operations used by this module.
// This interface allows for mocking in tests and potential future implementations.
type S3API interface {
	// PutObject uploads an object to S3
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)

	// GetObject retrieves an object from S3
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)

	// DeleteObject deletes an object from S3
	DeleteObject(
		ctx context.Context,
		params *s3.DeleteObjectInput,
		optFns ...func(*s3.Options),
	) (*s3.DeleteObjectOutput, error)

	// DeleteObjects deletes multiple objects from S3
	DeleteObjects(
		ctx context.Context,
		params *s3.DeleteObjectsInput,
		optFns ...func(*s3.Options),
	) (*s3.DeleteObjectsOutput, error)

	// ListObjectsV2 lists objects in an S3 bucket
	ListObjectsV2(
		ctx context.Context,
		params *s3.ListObjectsV2Input,
		optFns ...func(*s3.Options),
	) (*s3.ListObjectsV2Output, error)

	// HeadObject retrieves metadata about an object without retrieving the object itself
	HeadObject(
		ctx context.Context,
		params *s3.HeadObjectInput,
		optFns ...func(*s3.Options),
	) (*s3.HeadObjectOutput, error)

	// CopyObject copies an object within S3
	CopyObject(
		ctx context.Context,
		params *s3.CopyObjectInput,
		optFns ...func(*s3.Options),
	) (*s3.CopyObjectOutput, error)

	// CreateMultipartUpload initiates a multipart upload
	CreateMultipartUpload(
		ctx context.Context,
		params *s3.CreateMultipartUploadInput,
		optFns ...func(*s3.Options),
	) (*s3.CreateMultipartUploadOutput, error)

	// UploadPart uploads a part in a multipart upload
	UploadPart(
		ctx context.Context,
		params *s3.UploadPartInput,
		optFns ...func(*s3.Options),
	) (*s3.UploadPartOutput, error)

	// CompleteMultipartUpload completes a multipart upload
	CompleteMultipartUpload(
		ctx context.Context,
		params *s3.CompleteMultipartUploadInput,
		optFns ...func(*s3.Options),
	) (*s3.CompleteMultipartUploadOutput, error)

	// AbortMultipartUpload aborts a multipart upload
	AbortMultipartUpload(
		ctx context.Context,
		params *s3.AbortMultipartUploadInput,
		optFns ...func(*s3.Options),
	) (*s3.AbortMultipartUploadOutput, error)

	// CreateBucket creates a new S3 bucket
	CreateBucket(
		ctx context.Context,
		params *s3.CreateBucketInput,
		optFns ...func(*s3.Options),
	) (*s3.CreateBucketOutput, error)

	// DeleteBucket deletes an S3 bucket
	DeleteBucket(
		ctx context.Context,
		params *s3.DeleteBucketInput,
		optFns ...func(*s3.Options),
	) (*s3.DeleteBucketOutput, error)
}

// Verify that the AWS S3 client implements our interface
var _ S3API = (*s3.Client)(nil)
