// Package s3 provides properly mocked tests for upload operations.
package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/testutil"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// TestClient_Upload_WithMock tests the Upload method with mocked S3 client.
func TestClient_Upload_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		content     string
		opts        []s3types.UploadOption
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful small upload",
			bucket:  "test-bucket",
			key:     "test-key",
			content: "Hello, World!",
			setupMock: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify the input parameters
					assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
					assert.Equal(t, "test-key", aws.ToString(params.Key))

					// Read the body to verify content
					body, err := io.ReadAll(params.Body)
					require.NoError(t, err)
					assert.Equal(t, "Hello, World!", string(body))

					return &s3.PutObjectOutput{
						ETag: aws.String("mock-etag-123"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "upload with metadata",
			bucket:  "test-bucket",
			key:     "test-key",
			content: "test content",
			opts: []s3types.UploadOption{
				WithMetadata(map[string]string{
					"author":  "test-author",
					"version": "1.0",
				}),
			},
			setupMock: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify metadata was set
					assert.NotNil(t, params.Metadata)
					assert.Equal(t, "test-author", params.Metadata["author"])
					assert.Equal(t, "1.0", params.Metadata["version"])

					return &s3.PutObjectOutput{
						ETag: aws.String("mock-etag-456"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "upload with content type",
			bucket:  "test-bucket",
			key:     "test.json",
			content: `{"test": "data"}`,
			opts: []s3types.UploadOption{
				WithContentType("application/json"),
			},
			setupMock: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify content type was set
					assert.Equal(t, "application/json", aws.ToString(params.ContentType))

					return &s3.PutObjectOutput{
						ETag: aws.String("mock-etag-789"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "upload failure",
			bucket:  "test-bucket",
			key:     "test-key",
			content: "test content",
			setupMock: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					return nil, errors.New("upload failed: access denied")
				}
			},
			wantErr:     true,
			errContains: "upload failed",
		},
		{
			name:    "empty bucket name",
			bucket:  "",
			key:     "test-key",
			content: "test content",
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantErr:     true,
			errContains: "bucket name cannot be empty",
		},
		{
			name:    "empty key name",
			bucket:  "test-bucket",
			key:     "",
			content: "test content",
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantErr:     true,
			errContains: "object key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock S3 client
			mockClient := &testutil.MockS3Client{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			// Create client with mock
			client := NewWithClient(mockClient)

			// Perform upload
			ctx := context.Background()
			reader := strings.NewReader(tt.content)
			result, err := client.Upload(ctx, tt.bucket, tt.key, reader, tt.opts...)

			// Check results
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.key, result.Key)
				assert.Equal(t, int64(len(tt.content)), result.Size)
				assert.NotEmpty(t, result.ETag)
			}
		})
	}
}

// TestClient_Put_WithMock tests the Put method with mocked S3 client.
func TestClient_Put_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		data        []byte
		opts        []s3types.UploadOption
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:   "successful put",
			bucket: "test-bucket",
			key:    "test-key",
			data:   []byte("test data"),
			setupMock: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify the data
					body, err := io.ReadAll(params.Body)
					require.NoError(t, err)
					assert.Equal(t, "test data", string(body))

					return &s3.PutObjectOutput{
						ETag: aws.String("mock-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:   "put with empty data",
			bucket: "test-bucket",
			key:    "test-key",
			data:   []byte{},
			setupMock: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify empty data is handled
					body, err := io.ReadAll(params.Body)
					require.NoError(t, err)
					assert.Empty(t, body)

					return &s3.PutObjectOutput{
						ETag: aws.String("mock-etag-empty"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:   "put with nil data",
			bucket: "test-bucket",
			key:    "test-key",
			data:   nil,
			setupMock: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify nil data is handled
					body, err := io.ReadAll(params.Body)
					require.NoError(t, err)
					assert.Empty(t, body)

					return &s3.PutObjectOutput{
						ETag: aws.String("mock-etag-nil"),
					}, nil
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock S3 client
			mockClient := &testutil.MockS3Client{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			// Create client with mock
			client := NewWithClient(mockClient)

			// Perform put
			ctx := context.Background()
			err := client.Put(ctx, tt.bucket, tt.key, tt.data, tt.opts...)

			// Check results
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestClient_Upload_LargeFile_WithMock tests multipart upload with mocked S3 client.
func TestClient_Upload_LargeFile_WithMock(t *testing.T) {
	// Create large content (>100MB to trigger multipart)
	largeContent := strings.Repeat("A", 100*1024*1024+1)

	mockClient := &testutil.MockS3Client{
		CreateMultipartUploadFunc: func(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
			assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
			assert.Equal(t, "large-file", aws.ToString(params.Key))
			return &s3.CreateMultipartUploadOutput{
				UploadId: aws.String("test-upload-id"),
			}, nil
		},
		UploadPartFunc: func(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
			assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
			assert.Equal(t, "large-file", aws.ToString(params.Key))
			assert.Equal(t, "test-upload-id", aws.ToString(params.UploadId))
			return &s3.UploadPartOutput{
				ETag: aws.String("part-etag"),
			}, nil
		},
		CompleteMultipartUploadFunc: func(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
			assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
			assert.Equal(t, "large-file", aws.ToString(params.Key))
			assert.Equal(t, "test-upload-id", aws.ToString(params.UploadId))
			return &s3.CompleteMultipartUploadOutput{
				ETag: aws.String("complete-etag"),
			}, nil
		},
	}

	client := NewWithClient(mockClient)
	ctx := context.Background()

	reader := bytes.NewReader([]byte(largeContent))
	result, err := client.Upload(ctx, "test-bucket", "large-file", reader)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "large-file", result.Key)
	assert.Equal(t, int64(len(largeContent)), result.Size)
}

// TestClient_Upload_WithProgressTracker tests upload with progress tracking.
func TestClient_Upload_WithProgressTracker(t *testing.T) {
	tracker := &testutil.MockProgressTracker{}

	mockClient := &testutil.MockS3Client{
		PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			return &s3.PutObjectOutput{
				ETag: aws.String("mock-etag"),
			}, nil
		},
	}

	client := NewWithClient(mockClient)
	ctx := context.Background()

	content := "test content with progress"
	reader := strings.NewReader(content)

	result, err := client.Upload(ctx, "test-bucket", "test-key", reader, WithProgress(tracker))

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify progress tracker was called
	assert.True(t, tracker.UpdateCalled || tracker.CompleteCalled, "Progress tracker should have been called")
}
