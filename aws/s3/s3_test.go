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
