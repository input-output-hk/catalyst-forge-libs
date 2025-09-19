// Package testutil provides test utilities and mocks for S3 operations.
// This package is internal and should only be used for testing within the S3 module.
package testutil

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/s3api"
)

// MockS3Client is a mock implementation of the S3API interface for testing.
// It allows customization of each S3 operation through function fields.
type MockS3Client struct {
	PutObjectFunc               func(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObjectFunc               func(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObjectFunc            func(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	DeleteObjectsFunc           func(context.Context, *s3.DeleteObjectsInput, ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	ListObjectsV2Func           func(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	HeadObjectFunc              func(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	CopyObjectFunc              func(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	CreateMultipartUploadFunc   func(context.Context, *s3.CreateMultipartUploadInput, ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPartFunc              func(context.Context, *s3.UploadPartInput, ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	UploadPartCopyFunc          func(context.Context, *s3.UploadPartCopyInput, ...func(*s3.Options)) (*s3.UploadPartCopyOutput, error)
	CompleteMultipartUploadFunc func(context.Context, *s3.CompleteMultipartUploadInput, ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUploadFunc    func(context.Context, *s3.AbortMultipartUploadInput, ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
	CreateBucketFunc            func(context.Context, *s3.CreateBucketInput, ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	DeleteBucketFunc            func(context.Context, *s3.DeleteBucketInput, ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
}

// PutObject mocks the S3 PutObject operation.
func (m *MockS3Client) PutObject(
	ctx context.Context,
	params *s3.PutObjectInput,
	optFns ...func(*s3.Options),
) (*s3.PutObjectOutput, error) {
	if m.PutObjectFunc != nil {
		return m.PutObjectFunc(ctx, params, optFns...)
	}
	return &s3.PutObjectOutput{}, nil
}

// GetObject mocks the S3 GetObject operation.
func (m *MockS3Client) GetObject(
	ctx context.Context,
	params *s3.GetObjectInput,
	optFns ...func(*s3.Options),
) (*s3.GetObjectOutput, error) {
	if m.GetObjectFunc != nil {
		return m.GetObjectFunc(ctx, params, optFns...)
	}
	return &s3.GetObjectOutput{}, nil
}

// DeleteObject mocks the S3 DeleteObject operation.
func (m *MockS3Client) DeleteObject(
	ctx context.Context,
	params *s3.DeleteObjectInput,
	optFns ...func(*s3.Options),
) (*s3.DeleteObjectOutput, error) {
	if m.DeleteObjectFunc != nil {
		return m.DeleteObjectFunc(ctx, params, optFns...)
	}
	return &s3.DeleteObjectOutput{}, nil
}

// DeleteObjects mocks the S3 DeleteObjects operation.
func (m *MockS3Client) DeleteObjects(
	ctx context.Context,
	params *s3.DeleteObjectsInput,
	optFns ...func(*s3.Options),
) (*s3.DeleteObjectsOutput, error) {
	if m.DeleteObjectsFunc != nil {
		return m.DeleteObjectsFunc(ctx, params, optFns...)
	}
	return &s3.DeleteObjectsOutput{}, nil
}

// ListObjectsV2 mocks the S3 ListObjectsV2 operation.
func (m *MockS3Client) ListObjectsV2(
	ctx context.Context,
	params *s3.ListObjectsV2Input,
	optFns ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	if m.ListObjectsV2Func != nil {
		return m.ListObjectsV2Func(ctx, params, optFns...)
	}
	return &s3.ListObjectsV2Output{}, nil
}

// HeadObject mocks the S3 HeadObject operation.
func (m *MockS3Client) HeadObject(
	ctx context.Context,
	params *s3.HeadObjectInput,
	optFns ...func(*s3.Options),
) (*s3.HeadObjectOutput, error) {
	if m.HeadObjectFunc != nil {
		return m.HeadObjectFunc(ctx, params, optFns...)
	}
	return &s3.HeadObjectOutput{}, nil
}

// CopyObject mocks the S3 CopyObject operation.
func (m *MockS3Client) CopyObject(
	ctx context.Context,
	params *s3.CopyObjectInput,
	optFns ...func(*s3.Options),
) (*s3.CopyObjectOutput, error) {
	if m.CopyObjectFunc != nil {
		return m.CopyObjectFunc(ctx, params, optFns...)
	}
	return &s3.CopyObjectOutput{}, nil
}

// CreateMultipartUpload mocks the S3 CreateMultipartUpload operation.
func (m *MockS3Client) CreateMultipartUpload(
	ctx context.Context,
	params *s3.CreateMultipartUploadInput,
	optFns ...func(*s3.Options),
) (*s3.CreateMultipartUploadOutput, error) {
	if m.CreateMultipartUploadFunc != nil {
		return m.CreateMultipartUploadFunc(ctx, params, optFns...)
	}
	return &s3.CreateMultipartUploadOutput{}, nil
}

// UploadPart mocks the S3 UploadPart operation.
func (m *MockS3Client) UploadPart(
	ctx context.Context,
	params *s3.UploadPartInput,
	optFns ...func(*s3.Options),
) (*s3.UploadPartOutput, error) {
	if m.UploadPartFunc != nil {
		return m.UploadPartFunc(ctx, params, optFns...)
	}
	return &s3.UploadPartOutput{}, nil
}

// UploadPartCopy mocks the S3 UploadPartCopy operation.
func (m *MockS3Client) UploadPartCopy(
	ctx context.Context,
	params *s3.UploadPartCopyInput,
	optFns ...func(*s3.Options),
) (*s3.UploadPartCopyOutput, error) {
	if m.UploadPartCopyFunc != nil {
		return m.UploadPartCopyFunc(ctx, params, optFns...)
	}
	return &s3.UploadPartCopyOutput{}, nil
}

// CompleteMultipartUpload mocks the S3 CompleteMultipartUpload operation.
func (m *MockS3Client) CompleteMultipartUpload(
	ctx context.Context,
	params *s3.CompleteMultipartUploadInput,
	optFns ...func(*s3.Options),
) (*s3.CompleteMultipartUploadOutput, error) {
	if m.CompleteMultipartUploadFunc != nil {
		return m.CompleteMultipartUploadFunc(ctx, params, optFns...)
	}
	return &s3.CompleteMultipartUploadOutput{}, nil
}

// AbortMultipartUpload mocks the S3 AbortMultipartUpload operation.
func (m *MockS3Client) AbortMultipartUpload(
	ctx context.Context,
	params *s3.AbortMultipartUploadInput,
	optFns ...func(*s3.Options),
) (*s3.AbortMultipartUploadOutput, error) {
	if m.AbortMultipartUploadFunc != nil {
		return m.AbortMultipartUploadFunc(ctx, params, optFns...)
	}
	return &s3.AbortMultipartUploadOutput{}, nil
}

// CreateBucket mocks the S3 CreateBucket operation.
func (m *MockS3Client) CreateBucket(
	ctx context.Context,
	params *s3.CreateBucketInput,
	optFns ...func(*s3.Options),
) (*s3.CreateBucketOutput, error) {
	if m.CreateBucketFunc != nil {
		return m.CreateBucketFunc(ctx, params, optFns...)
	}
	return &s3.CreateBucketOutput{}, nil
}

// DeleteBucket mocks the S3 DeleteBucket operation.
func (m *MockS3Client) DeleteBucket(
	ctx context.Context,
	params *s3.DeleteBucketInput,
	optFns ...func(*s3.Options),
) (*s3.DeleteBucketOutput, error) {
	if m.DeleteBucketFunc != nil {
		return m.DeleteBucketFunc(ctx, params, optFns...)
	}
	return &s3.DeleteBucketOutput{}, nil
}

// Ensure MockS3Client implements s3api.S3API interface
var _ s3api.S3API = (*MockS3Client)(nil)
