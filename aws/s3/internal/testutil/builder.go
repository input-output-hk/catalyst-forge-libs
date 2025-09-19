// Package testutil provides a builder for creating mock S3 clients.
package testutil

import (
	"context"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// MockBuilder provides a fluent interface for building MockS3Client instances.
type MockBuilder struct {
	client *MockS3Client
}

// NewMockBuilder creates a new MockBuilder.
func NewMockBuilder() *MockBuilder {
	return &MockBuilder{
		client: &MockS3Client{},
	}
}

// Build returns the configured MockS3Client.
func (b *MockBuilder) Build() *MockS3Client {
	return b.client
}

// WithPutObject configures the PutObject behavior.
func (b *MockBuilder) WithPutObject(
	fn func(context.Context, *s3.PutObjectInput) (*s3.PutObjectOutput, error),
) *MockBuilder {
	b.client.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
		return fn(ctx, params)
	}
	return b
}

// WithGetObject configures the GetObject behavior.
func (b *MockBuilder) WithGetObject(
	fn func(context.Context, *s3.GetObjectInput) (*s3.GetObjectOutput, error),
) *MockBuilder {
	b.client.GetObjectFunc = func(ctx context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
		return fn(ctx, params)
	}
	return b
}

// WithDeleteObject configures the DeleteObject behavior.
func (b *MockBuilder) WithDeleteObject(
	fn func(context.Context, *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error),
) *MockBuilder {
	b.client.DeleteObjectFunc = func(ctx context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
		return fn(ctx, params)
	}
	return b
}

// WithListObjectsV2 configures the ListObjectsV2 behavior.
func (b *MockBuilder) WithListObjectsV2(
	fn func(context.Context, *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error),
) *MockBuilder {
	b.client.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
		return fn(ctx, params)
	}
	return b
}

// WithHeadObject configures the HeadObject behavior.
func (b *MockBuilder) WithHeadObject(
	fn func(context.Context, *s3.HeadObjectInput) (*s3.HeadObjectOutput, error),
) *MockBuilder {
	b.client.HeadObjectFunc = func(ctx context.Context, params *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
		return fn(ctx, params)
	}
	return b
}

// WithCopyObject configures the CopyObject behavior.
func (b *MockBuilder) WithCopyObject(
	fn func(context.Context, *s3.CopyObjectInput) (*s3.CopyObjectOutput, error),
) *MockBuilder {
	b.client.CopyObjectFunc = func(ctx context.Context, params *s3.CopyObjectInput, _ ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
		return fn(ctx, params)
	}
	return b
}

// WithCreateBucket configures the CreateBucket behavior.
func (b *MockBuilder) WithCreateBucket(
	fn func(context.Context, *s3.CreateBucketInput) (*s3.CreateBucketOutput, error),
) *MockBuilder {
	b.client.CreateBucketFunc = func(ctx context.Context, params *s3.CreateBucketInput, _ ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
		return fn(ctx, params)
	}
	return b
}

// WithDeleteBucket configures the DeleteBucket behavior.
func (b *MockBuilder) WithDeleteBucket(
	fn func(context.Context, *s3.DeleteBucketInput) (*s3.DeleteBucketOutput, error),
) *MockBuilder {
	b.client.DeleteBucketFunc = func(ctx context.Context, params *s3.DeleteBucketInput, _ ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
		return fn(ctx, params)
	}
	return b
}

// WithSuccessfulUpload configures the mock to always return successful uploads.
func (b *MockBuilder) WithSuccessfulUpload() *MockBuilder {
	b.client.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
		// Consume the body if provided
		if params.Body != nil {
			_, _ = io.Copy(io.Discard, params.Body)
		}
		return &s3.PutObjectOutput{
			ETag: StringPtr(`"test-etag"`),
		}, nil
	}
	return b
}

// WithFailedUpload configures the mock to always return upload failures.
func (b *MockBuilder) WithFailedUpload(err error) *MockBuilder {
	b.client.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
		return nil, err
	}
	return b
}

// WithObjectNotFound configures the mock to return object not found errors.
func (b *MockBuilder) WithObjectNotFound() *MockBuilder {
	notFoundErr := &types.NoSuchKey{
		Message: StringPtr("The specified key does not exist."),
	}

	b.client.GetObjectFunc = func(ctx context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
		return nil, notFoundErr
	}
	b.client.HeadObjectFunc = func(ctx context.Context, params *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
		return nil, notFoundErr
	}
	return b
}

// WithEmptyBucket configures the mock to return an empty bucket listing.
func (b *MockBuilder) WithEmptyBucket() *MockBuilder {
	b.client.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
		return &s3.ListObjectsV2Output{
			Name:        params.Bucket,
			Prefix:      params.Prefix,
			Delimiter:   params.Delimiter,
			MaxKeys:     params.MaxKeys,
			IsTruncated: BoolPtr(false),
			KeyCount:    Int32Ptr(0),
		}, nil
	}
	return b
}

// WithMultipartUpload configures the mock for multipart upload operations.
func (b *MockBuilder) WithMultipartUpload() *MockBuilder {
	uploadID := "test-upload-id"

	b.client.CreateMultipartUploadFunc = func(ctx context.Context, params *s3.CreateMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
		return &s3.CreateMultipartUploadOutput{
			UploadId: StringPtr(uploadID),
			Bucket:   params.Bucket,
			Key:      params.Key,
		}, nil
	}

	b.client.UploadPartFunc = func(ctx context.Context, params *s3.UploadPartInput, _ ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
		// Consume the body if provided
		if params.Body != nil {
			_, _ = io.Copy(io.Discard, params.Body)
		}
		return &s3.UploadPartOutput{
			ETag: StringPtr(`"part-etag"`),
		}, nil
	}

	b.client.CompleteMultipartUploadFunc = func(ctx context.Context, params *s3.CompleteMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
		return &s3.CompleteMultipartUploadOutput{
			ETag:   StringPtr(`"multipart-etag"`),
			Bucket: params.Bucket,
			Key:    params.Key,
		}, nil
	}

	b.client.AbortMultipartUploadFunc = func(ctx context.Context, params *s3.AbortMultipartUploadInput, _ ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
		return &s3.AbortMultipartUploadOutput{}, nil
	}

	return b
}

// WithAccessDenied configures the mock to return access denied errors.
func (b *MockBuilder) WithAccessDenied() *MockBuilder {
	accessDeniedErr := errors.New("access denied")

	b.client.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
		return nil, accessDeniedErr
	}
	b.client.GetObjectFunc = func(ctx context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
		return nil, accessDeniedErr
	}
	b.client.DeleteObjectFunc = func(ctx context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
		return nil, accessDeniedErr
	}
	b.client.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
		return nil, accessDeniedErr
	}

	return b
}
