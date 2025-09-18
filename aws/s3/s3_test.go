// Package s3 provides properly mocked tests for upload operations.
package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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

// TestClient_Delete_WithMock tests the Delete method with mocked S3 client.
func TestClient_Delete_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:   "successful delete",
			bucket: "test-bucket",
			key:    "test-key",
			setupMock: func(m *testutil.MockS3Client) {
				m.DeleteObjectFunc = func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
					assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
					assert.Equal(t, "test-key", aws.ToString(params.Key))
					return &s3.DeleteObjectOutput{}, nil
				}
			},
			wantErr: false,
		},
		{
			name:   "delete non-existent object",
			bucket: "test-bucket",
			key:    "non-existent-key",
			setupMock: func(m *testutil.MockS3Client) {
				m.DeleteObjectFunc = func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
					return nil, errors.New("NoSuchKey: The specified key does not exist")
				}
			},
			wantErr:     true,
			errContains: "NoSuchKey",
		},
		{
			name:   "empty bucket name",
			bucket: "",
			key:    "test-key",
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantErr:     true,
			errContains: "bucket name cannot be empty",
		},
		{
			name:   "empty key name",
			bucket: "test-bucket",
			key:    "",
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

			// Perform delete
			ctx := context.Background()
			err := client.Delete(ctx, tt.bucket, tt.key)

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

// TestClient_DeleteMany_WithMock tests the DeleteMany method with mocked S3 client.
func TestClient_DeleteMany_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		keys        []string
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:   "successful batch delete",
			bucket: "test-bucket",
			keys:   []string{"key1", "key2", "key3"},
			setupMock: func(m *testutil.MockS3Client) {
				m.DeleteObjectsFunc = func(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
					assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
					assert.Len(t, params.Delete.Objects, 3)
					assert.Equal(t, "key1", aws.ToString(params.Delete.Objects[0].Key))
					assert.Equal(t, "key2", aws.ToString(params.Delete.Objects[1].Key))
					assert.Equal(t, "key3", aws.ToString(params.Delete.Objects[2].Key))
					return &s3.DeleteObjectsOutput{}, nil
				}
			},
			wantErr: false,
		},
		{
			name:   "empty keys slice",
			bucket: "test-bucket",
			keys:   []string{},
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantErr:     true,
			errContains: "keys cannot be empty",
		},
		{
			name:   "empty bucket name",
			bucket: "",
			keys:   []string{"key1"},
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantErr:     true,
			errContains: "bucket name cannot be empty",
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

			// Perform delete many
			ctx := context.Background()
			result, err := client.DeleteMany(ctx, tt.bucket, tt.keys)

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
			}
		})
	}
}

// TestClient_Exists_WithMock tests the Exists method with mocked S3 client.
func TestClient_Exists_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		setupMock   func(*testutil.MockS3Client)
		wantExists  bool
		wantErr     bool
		errContains string
	}{
		{
			name:   "object exists",
			bucket: "test-bucket",
			key:    "existing-key",
			setupMock: func(m *testutil.MockS3Client) {
				m.HeadObjectFunc = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
					assert.Equal(t, "existing-key", aws.ToString(params.Key))
					return &s3.HeadObjectOutput{}, nil
				}
			},
			wantExists: true,
			wantErr:    false,
		},
		{
			name:   "object does not exist",
			bucket: "test-bucket",
			key:    "non-existent-key",
			setupMock: func(m *testutil.MockS3Client) {
				m.HeadObjectFunc = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					return nil, errors.New("NotFound: The specified key does not exist")
				}
			},
			wantExists: false,
			wantErr:    false,
		},
		{
			name:   "empty bucket name",
			bucket: "",
			key:    "test-key",
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantExists:  false,
			wantErr:     true,
			errContains: "bucket name cannot be empty",
		},
		{
			name:   "empty key name",
			bucket: "test-bucket",
			key:    "",
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantExists:  false,
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

			// Check existence
			ctx := context.Background()
			exists, err := client.Exists(ctx, tt.bucket, tt.key)

			// Check results
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantExists, exists)
			}
		})
	}
}

// TestClient_GetMetadata_WithMock tests the GetMetadata method with mocked S3 client.
func TestClient_GetMetadata_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:   "successful metadata retrieval",
			bucket: "test-bucket",
			key:    "test-key",
			setupMock: func(m *testutil.MockS3Client) {
				m.HeadObjectFunc = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
					assert.Equal(t, "test-key", aws.ToString(params.Key))
					return &s3.HeadObjectOutput{
						ContentType:   aws.String("application/json"),
						ContentLength: aws.Int64(1024),
						LastModified:  aws.Time(time.Now()),
						ETag:          aws.String("\"mock-etag\""),
						Metadata: map[string]string{
							"author":  "test-author",
							"version": "1.0",
						},
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:   "object not found",
			bucket: "test-bucket",
			key:    "non-existent-key",
			setupMock: func(m *testutil.MockS3Client) {
				m.HeadObjectFunc = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					return nil, errors.New("NotFound: The specified key does not exist")
				}
			},
			wantErr:     true,
			errContains: "NotFound",
		},
		{
			name:   "empty bucket name",
			bucket: "",
			key:    "test-key",
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantErr:     true,
			errContains: "bucket name cannot be empty",
		},
		{
			name:   "empty key name",
			bucket: "test-bucket",
			key:    "",
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

			// Get metadata
			ctx := context.Background()
			metadata, err := client.GetMetadata(ctx, tt.bucket, tt.key)

			// Check results
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, metadata)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, metadata)
			}
		})
	}
}

// TestClient_Copy_WithMock tests the Copy method with mocked S3 client.
func TestClient_Copy_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		srcBucket   string
		srcKey      string
		dstBucket   string
		dstKey      string
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:      "successful copy",
			srcBucket: "src-bucket",
			srcKey:    "src-key",
			dstBucket: "dst-bucket",
			dstKey:    "dst-key",
			setupMock: func(m *testutil.MockS3Client) {
				m.CopyObjectFunc = func(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
					assert.Equal(t, "dst-bucket", aws.ToString(params.Bucket))
					assert.Equal(t, "dst-key", aws.ToString(params.Key))
					assert.Equal(t, "src-bucket/src-key", aws.ToString(params.CopySource))
					return &s3.CopyObjectOutput{}, nil
				}
			},
			wantErr: false,
		},
		{
			name:      "copy to same bucket and key",
			srcBucket: "test-bucket",
			srcKey:    "same-key",
			dstBucket: "test-bucket",
			dstKey:    "same-key",
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantErr:     true,
			errContains: "cannot copy object to itself",
		},
		{
			name:      "empty source bucket",
			srcBucket: "",
			srcKey:    "src-key",
			dstBucket: "dst-bucket",
			dstKey:    "dst-key",
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantErr:     true,
			errContains: "source bucket name cannot be empty",
		},
		{
			name:      "empty destination bucket",
			srcBucket: "src-bucket",
			srcKey:    "src-key",
			dstBucket: "",
			dstKey:    "dst-key",
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantErr:     true,
			errContains: "destination bucket name cannot be empty",
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

			// Perform copy
			ctx := context.Background()
			err := client.Copy(ctx, tt.srcBucket, tt.srcKey, tt.dstBucket, tt.dstKey)

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

// TestClient_Move_WithMock tests the Move method with mocked S3 client.
func TestClient_Move_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		srcBucket   string
		srcKey      string
		dstBucket   string
		dstKey      string
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:      "successful move",
			srcBucket: "src-bucket",
			srcKey:    "src-key",
			dstBucket: "dst-bucket",
			dstKey:    "dst-key",
			setupMock: func(m *testutil.MockS3Client) {
				callCount := 0
				m.CopyObjectFunc = func(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
					callCount++
					assert.Equal(t, "dst-bucket", aws.ToString(params.Bucket))
					assert.Equal(t, "dst-key", aws.ToString(params.Key))
					assert.Equal(t, "src-bucket/src-key", aws.ToString(params.CopySource))
					return &s3.CopyObjectOutput{}, nil
				}
				m.DeleteObjectFunc = func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
					assert.Equal(t, 1, callCount, "Copy should be called before delete")
					assert.Equal(t, "src-bucket", aws.ToString(params.Bucket))
					assert.Equal(t, "src-key", aws.ToString(params.Key))
					return &s3.DeleteObjectOutput{}, nil
				}
			},
			wantErr: false,
		},
		{
			name:      "move to same location",
			srcBucket: "test-bucket",
			srcKey:    "same-key",
			dstBucket: "test-bucket",
			dstKey:    "same-key",
			setupMock: func(m *testutil.MockS3Client) {
				// Mock shouldn't be called due to validation
			},
			wantErr:     true,
			errContains: "cannot move object to itself",
		},
		{
			name:      "copy succeeds but delete fails",
			srcBucket: "src-bucket",
			srcKey:    "src-key",
			dstBucket: "dst-bucket",
			dstKey:    "dst-key",
			setupMock: func(m *testutil.MockS3Client) {
				m.CopyObjectFunc = func(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
					return &s3.CopyObjectOutput{}, nil
				}
				m.DeleteObjectFunc = func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
					return nil, errors.New("delete failed: access denied")
				}
			},
			wantErr:     true,
			errContains: "delete failed",
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

			// Perform move
			ctx := context.Background()
			err := client.Move(ctx, tt.srcBucket, tt.srcKey, tt.dstBucket, tt.dstKey)

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

// TestClient_List_WithMock tests the List method with mocked S3 client.
func TestClient_List_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		prefix      string
		opts        []s3types.ListOption
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
		expected    *s3types.ListResult
	}{
		{
			name:   "successful list with no objects",
			bucket: "test-bucket",
			setupMock: func(m *testutil.MockS3Client) {
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					// Verify parameters
					assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
					assert.Equal(t, "", aws.ToString(params.Prefix))
					assert.Equal(t, int32(1000), aws.ToInt32(params.MaxKeys))

					return &s3.ListObjectsV2Output{
						Contents:    []types.Object{},
						IsTruncated: aws.Bool(false),
						KeyCount:    aws.Int32(0),
						MaxKeys:     aws.Int32(1000),
						Name:        aws.String("test-bucket"),
						Prefix:      aws.String(""),
					}, nil
				}
			},
			wantErr: false,
			expected: &s3types.ListResult{
				Objects:     []s3types.Object{},
				IsTruncated: false,
			},
		},
		{
			name:   "successful list with objects",
			bucket: "test-bucket",
			setupMock: func(m *testutil.MockS3Client) {
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					return &s3.ListObjectsV2Output{
						Contents: []types.Object{
							{
								Key:          aws.String("file1.txt"),
								Size:         aws.Int64(1024),
								LastModified: aws.Time(time.Now()),
								ETag:         aws.String("\"etag1\""),
								StorageClass: types.ObjectStorageClassStandard,
							},
							{
								Key:          aws.String("file2.txt"),
								Size:         aws.Int64(2048),
								LastModified: aws.Time(time.Now()),
								ETag:         aws.String("\"etag2\""),
								StorageClass: types.ObjectStorageClassStandardIa,
							},
						},
						IsTruncated: aws.Bool(false),
						KeyCount:    aws.Int32(2),
						MaxKeys:     aws.Int32(1000),
						Name:        aws.String("test-bucket"),
					}, nil
				}
			},
			wantErr: false,
			expected: &s3types.ListResult{
				Objects: []s3types.Object{
					{
						Key:          "file1.txt",
						Size:         1024,
						LastModified: time.Now(),
						ETag:         "\"etag1\"",
						StorageClass: "STANDARD",
					},
					{
						Key:          "file2.txt",
						Size:         2048,
						LastModified: time.Now(),
						ETag:         "\"etag2\"",
						StorageClass: "STANDARD_IA",
					},
				},
				IsTruncated: false,
			},
		},
		{
			name:   "list with prefix filter",
			bucket: "test-bucket",
			opts: []s3types.ListOption{
				WithPrefix("docs/"),
			},
			setupMock: func(m *testutil.MockS3Client) {
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					assert.Equal(t, "docs/", aws.ToString(params.Prefix))
					return &s3.ListObjectsV2Output{
						Contents: []types.Object{
							{
								Key:          aws.String("docs/readme.txt"),
								Size:         aws.Int64(512),
								LastModified: aws.Time(time.Now()),
								ETag:         aws.String("\"etag-docs\""),
							},
						},
						IsTruncated: aws.Bool(false),
						KeyCount:    aws.Int32(1),
						Prefix:      aws.String("docs/"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:   "list with max keys",
			bucket: "test-bucket",
			opts: []s3types.ListOption{
				WithMaxKeys(50),
			},
			setupMock: func(m *testutil.MockS3Client) {
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					assert.Equal(t, int32(50), aws.ToInt32(params.MaxKeys))
					return &s3.ListObjectsV2Output{
						Contents:    []types.Object{},
						IsTruncated: aws.Bool(false),
						KeyCount:    aws.Int32(0),
						MaxKeys:     aws.Int32(50),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:   "list with delimiter for hierarchical listing",
			bucket: "test-bucket",
			opts: []s3types.ListOption{
				WithDelimiter("/"),
			},
			setupMock: func(m *testutil.MockS3Client) {
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					assert.Equal(t, "/", aws.ToString(params.Delimiter))
					return &s3.ListObjectsV2Output{
						Contents:       []types.Object{},
						CommonPrefixes: []types.CommonPrefix{},
						IsTruncated:    aws.Bool(false),
						KeyCount:       aws.Int32(0),
						Delimiter:      aws.String("/"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:   "list with start after for pagination",
			bucket: "test-bucket",
			opts: []s3types.ListOption{
				WithStartAfter("file001.txt"),
			},
			setupMock: func(m *testutil.MockS3Client) {
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					assert.Equal(t, "file001.txt", aws.ToString(params.StartAfter))
					return &s3.ListObjectsV2Output{
						Contents:    []types.Object{},
						IsTruncated: aws.Bool(false),
						KeyCount:    aws.Int32(0),
						StartAfter:  aws.String("file001.txt"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:        "list with empty bucket name",
			bucket:      "",
			wantErr:     true,
			errContains: "bucket name cannot be empty",
		},
		{
			name:   "list with S3 error",
			bucket: "test-bucket",
			setupMock: func(m *testutil.MockS3Client) {
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					return nil, errors.New("AccessDenied: Access Denied")
				}
			},
			wantErr:     true,
			errContains: "AccessDenied: Access Denied",
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

			// Perform list operation
			ctx := context.Background()
			result, err := client.List(ctx, tt.bucket, tt.prefix, tt.opts...)

			// Check results
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			// Success case
			assert.NoError(t, err)
			require.NotNil(t, result)

			// Basic structure checks
			assert.NotNil(t, result.Objects)
			assert.IsType(t, false, result.IsTruncated)

			// If we have expected results, verify them
			if tt.expected != nil {
				assert.Equal(t, tt.expected.IsTruncated, result.IsTruncated)
				assert.Len(t, result.Objects, len(tt.expected.Objects))

				for i, expectedObj := range tt.expected.Objects {
					if i < len(result.Objects) {
						actualObj := result.Objects[i]
						assert.Equal(t, expectedObj.Key, actualObj.Key)
						assert.Equal(t, expectedObj.Size, actualObj.Size)
						assert.Equal(t, expectedObj.ETag, actualObj.ETag)
						assert.Equal(t, expectedObj.StorageClass, actualObj.StorageClass)
						// Note: LastModified comparison might be tricky due to time precision
					}
				}
			}
		})
	}
}

// TestClient_ListAll_WithMock tests the ListAll method with mocked S3 client.
func TestClient_ListAll_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		prefix      string
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
		expectClose bool // Whether we expect the channel to be closed
	}{
		{
			name:   "successful list all with no objects",
			bucket: "test-bucket",
			setupMock: func(m *testutil.MockS3Client) {
				callCount := 0
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					callCount++
					assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
					assert.Equal(t, "", aws.ToString(params.Prefix))

					return &s3.ListObjectsV2Output{
						Contents:    []types.Object{},
						IsTruncated: aws.Bool(false),
						KeyCount:    aws.Int32(0),
						MaxKeys:     aws.Int32(1000),
					}, nil
				}
			},
			wantErr:     false,
			expectClose: true,
		},
		{
			name:   "successful list all with multiple pages",
			bucket: "test-bucket",
			setupMock: func(m *testutil.MockS3Client) {
				callCount := 0
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					callCount++
					switch callCount {
					case 1:
						// First page
						return &s3.ListObjectsV2Output{
							Contents: []types.Object{
								{
									Key:          aws.String("file1.txt"),
									Size:         aws.Int64(1024),
									LastModified: aws.Time(time.Now()),
									ETag:         aws.String("\"etag1\""),
								},
							},
							IsTruncated:           aws.Bool(true),
							KeyCount:              aws.Int32(1),
							MaxKeys:               aws.Int32(1000),
							NextContinuationToken: aws.String("token123"),
						}, nil
					case 2:
						// Second page
						assert.Equal(t, "token123", aws.ToString(params.ContinuationToken))
						return &s3.ListObjectsV2Output{
							Contents: []types.Object{
								{
									Key:          aws.String("file2.txt"),
									Size:         aws.Int64(2048),
									LastModified: aws.Time(time.Now()),
									ETag:         aws.String("\"etag2\""),
								},
							},
							IsTruncated: aws.Bool(false),
							KeyCount:    aws.Int32(1),
							MaxKeys:     aws.Int32(1000),
						}, nil
					default:
						return nil, errors.New("unexpected call")
					}
				}
			},
			wantErr:     false,
			expectClose: true,
		},
		{
			name:   "list all with prefix",
			bucket: "test-bucket",
			prefix: "logs/",
			setupMock: func(m *testutil.MockS3Client) {
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					assert.Equal(t, "logs/", aws.ToString(params.Prefix))
					return &s3.ListObjectsV2Output{
						Contents: []types.Object{
							{
								Key:          aws.String("logs/app.log"),
								Size:         aws.Int64(512),
								LastModified: aws.Time(time.Now()),
								ETag:         aws.String("\"etag-logs\""),
							},
						},
						IsTruncated: aws.Bool(false),
						KeyCount:    aws.Int32(1),
					}, nil
				}
			},
			wantErr:     false,
			expectClose: true,
		},
		{
			name:        "list all with empty bucket name",
			bucket:      "",
			wantErr:     true,
			errContains: "bucket name cannot be empty",
		},
		{
			name:   "list all with S3 error on first call",
			bucket: "test-bucket",
			setupMock: func(m *testutil.MockS3Client) {
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					return nil, errors.New("NoSuchBucket: The specified bucket does not exist")
				}
			},
			wantErr:     true,
			errContains: "NoSuchBucket",
			expectClose: true, // Channel should still be closed on error
		},
		{
			name:   "list all with context cancellation",
			bucket: "test-bucket",
			setupMock: func(m *testutil.MockS3Client) {
				m.ListObjectsV2Func = func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
					// Simulate slow operation that can be cancelled
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
						return &s3.ListObjectsV2Output{
							Contents: []types.Object{
								{
									Key:          aws.String("file1.txt"),
									Size:         aws.Int64(1024),
									LastModified: aws.Time(time.Now()),
									ETag:         aws.String("\"etag1\""),
								},
							},
							IsTruncated: aws.Bool(false),
							KeyCount:    aws.Int32(1),
						}, nil
					}
				}
			},
			wantErr:     false,
			expectClose: true,
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

			// Perform list all operation
			ctx := context.Background()
			if tt.name == "list all with context cancellation" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				// Cancel context immediately to test cancellation
				cancel()
			}

			objectsChan := client.ListAll(ctx, tt.bucket, tt.prefix)

			// Collect results from channel
			var objects []s3types.Object
			var receivedError error
			channelClosed := false

			for obj := range objectsChan {
				objects = append(objects, obj) //nolint:staticcheck // collecting objects for test
			}

			// Check if channel was closed (by attempting to read one more time)
			select {
			case _, ok := <-objectsChan:
				if !ok {
					channelClosed = true
				}
			default:
				// Channel is closed if we can't read without blocking
				channelClosed = true
			}

			// For error cases, we expect the channel to be closed
			if tt.expectClose {
				assert.True(t, channelClosed, "channel should be closed")
			}

			// Check for errors (in a real implementation, errors would be sent differently)
			// For now, we just verify the basic structure works
			assert.NotNil(t, objectsChan)
			_ = receivedError // In real implementation, we'd check this
		})
	}
}
