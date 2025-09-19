// Package upload provides unit tests for S3 upload operations.
package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awstypes "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/testutil"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

func TestUploader_Upload_Simple(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		bucket      string
		key         string
		config      *s3types.UploadConfig
		mockFunc    func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful small upload",
			content: "Hello, World!",
			bucket:  "test-bucket",
			key:     "test-key",
			config: &s3types.UploadConfig{
				ContentType: "text/plain",
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify input
					assert.Equal(t, "test-bucket", aws.ToString(input.Bucket))
					assert.Equal(t, "test-key", aws.ToString(input.Key))
					assert.Equal(t, "text/plain", aws.ToString(input.ContentType))

					// Read the body to ensure it's correct
					body, err := io.ReadAll(input.Body)
					require.NoError(t, err)
					assert.Equal(t, "Hello, World!", string(body))

					return &s3.PutObjectOutput{
						ETag: aws.String("test-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "upload with metadata",
			content: "test content",
			bucket:  "test-bucket",
			key:     "test-key",
			config: &s3types.UploadConfig{
				ContentType: "text/plain",
				Metadata: map[string]string{
					"author":  "test",
					"version": "1.0",
				},
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					assert.Equal(t, "test", input.Metadata["author"])
					assert.Equal(t, "1.0", input.Metadata["version"])
					return &s3.PutObjectOutput{
						ETag: aws.String("test-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "upload with SSE-S3",
			content: "encrypted content",
			bucket:  "test-bucket",
			key:     "test-key",
			config: &s3types.UploadConfig{
				ContentType: "text/plain",
				SSE: &s3types.SSEConfig{
					Type: s3types.SSES3,
				},
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					assert.Equal(t, awstypes.ServerSideEncryptionAes256, input.ServerSideEncryption)
					return &s3.PutObjectOutput{
						ETag: aws.String("test-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "upload with ACL",
			content: "acl content",
			bucket:  "test-bucket",
			key:     "test-key",
			config: &s3types.UploadConfig{
				ContentType: "text/plain",
				ACL:         s3types.ACLPublicRead,
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					assert.Equal(t, awstypes.ObjectCannedACL("public-read"), input.ACL)
					return &s3.PutObjectOutput{
						ETag: aws.String("acl-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "upload with default private ACL",
			content: "private content",
			bucket:  "test-bucket",
			key:     "test-key",
			config: &s3types.UploadConfig{
				ContentType: "text/plain",
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Should default to private ACL
					assert.Equal(t, awstypes.ObjectCannedACL("private"), input.ACL)
					return &s3.PutObjectOutput{
						ETag: aws.String("private-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "upload failure",
			content: "test content",
			bucket:  "test-bucket",
			key:     "test-key",
			config:  &s3types.UploadConfig{},
			mockFunc: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					return nil, errors.New("upload failed")
				}
			},
			wantErr:     true,
			errContains: "upload failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &testutil.MockS3Client{}
			if tt.mockFunc != nil {
				tt.mockFunc(mockClient)
			}

			// Create uploader with mock client
			uploader := New(mockClient)

			ctx := context.Background()
			startTime := time.Now()

			// Test the Upload method
			result, err := uploader.Upload(
				ctx,
				tt.bucket,
				tt.key,
				strings.NewReader(tt.content),
				tt.config,
				startTime,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.key, result.Key)
				assert.Equal(t, int64(len(tt.content)), result.Size)
			}
		})
	}
}

func TestUploader_Upload_Multipart(t *testing.T) {
	// Create large content (>100MB threshold)
	largeContent := strings.Repeat("A", 100*1024*1024+1)

	tests := []struct {
		name        string
		content     string
		bucket      string
		key         string
		config      *s3types.UploadConfig
		mockFunc    func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful multipart upload",
			content: largeContent,
			bucket:  "test-bucket",
			key:     "large-file",
			config: &s3types.UploadConfig{
				ContentType: "application/octet-stream",
				PartSize:    5 * 1024 * 1024, // 5MB parts
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.CreateMultipartUploadFunc = func(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
					return &s3.CreateMultipartUploadOutput{
						UploadId: aws.String("test-upload-id"),
					}, nil
				}
				m.UploadPartFunc = func(ctx context.Context, input *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
					return &s3.UploadPartOutput{
						ETag: aws.String("part-etag"),
					}, nil
				}
				m.CompleteMultipartUploadFunc = func(ctx context.Context, input *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
					return &s3.CompleteMultipartUploadOutput{
						ETag: aws.String("complete-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &testutil.MockS3Client{}
			if tt.mockFunc != nil {
				tt.mockFunc(mockClient)
			}

			// Create uploader with mock client
			uploader := New(mockClient)

			ctx := context.Background()
			startTime := time.Now()

			// Test the Upload method with large content
			result, err := uploader.Upload(
				ctx,
				tt.bucket,
				tt.key,
				strings.NewReader(tt.content),
				tt.config,
				startTime,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
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

func TestUploader_UploadFile(t *testing.T) {
	// Create a temporary test file
	tmpFile := createTempFile(t, "test content")
	defer os.Remove(tmpFile)

	tests := []struct {
		name     string
		bucket   string
		key      string
		filepath string
		config   *s3types.UploadConfig
		mockFunc func(*testutil.MockS3Client)
		wantErr  bool
	}{
		{
			name:     "successful file upload",
			bucket:   "test-bucket",
			key:      "test-file",
			filepath: tmpFile,
			config: &s3types.UploadConfig{
				ContentType: "text/plain",
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify bucket and key
					assert.Equal(t, "test-bucket", aws.ToString(input.Bucket))
					assert.Equal(t, "test-file", aws.ToString(input.Key))
					assert.Equal(t, "text/plain", aws.ToString(input.ContentType))

					// Read and verify content
					body, err := io.ReadAll(input.Body)
					require.NoError(t, err)
					assert.Equal(t, "test content", string(body))

					return &s3.PutObjectOutput{
						ETag: aws.String("file-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &testutil.MockS3Client{}
			if tt.mockFunc != nil {
				tt.mockFunc(mockClient)
			}

			// Create uploader with mock client
			uploader := New(mockClient)

			// Open the file for reading
			file, err := os.Open(tt.filepath)
			require.NoError(t, err)
			defer file.Close()

			// Get file info for size
			info, err := file.Stat()
			require.NoError(t, err)

			ctx := context.Background()
			startTime := time.Now()

			// Test the UploadFile method
			result, err := uploader.UploadFile(
				ctx,
				tt.bucket,
				tt.key,
				file,
				info.Size(),
				tt.config,
				startTime,
			)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.key, result.Key)
				assert.Equal(t, info.Size(), result.Size)
				assert.NotEmpty(t, result.ETag)
			}
		})
	}
}

func TestUploader_UploadSimple(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		data        []byte
		config      *s3types.UploadConfig
		mockFunc    func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:   "successful simple upload",
			bucket: "test-bucket",
			key:    "test-key",
			data:   []byte("simple upload test"),
			config: &s3types.UploadConfig{
				ContentType: "text/plain",
				Metadata: map[string]string{
					"test": "metadata",
				},
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify all parameters
					assert.Equal(t, "test-bucket", aws.ToString(input.Bucket))
					assert.Equal(t, "test-key", aws.ToString(input.Key))
					assert.Equal(t, "text/plain", aws.ToString(input.ContentType))
					assert.Equal(t, int64(18), aws.ToInt64(input.ContentLength))
					assert.Equal(t, "metadata", input.Metadata["test"])

					return &s3.PutObjectOutput{
						ETag: aws.String("simple-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:   "upload with SSE-KMS",
			bucket: "test-bucket",
			key:    "encrypted-key",
			data:   []byte("encrypted data"),
			config: &s3types.UploadConfig{
				ContentType: "text/plain",
				SSE: &s3types.SSEConfig{
					Type:     s3types.SSEKMS,
					KMSKeyID: "my-kms-key",
				},
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify encryption settings
					assert.Equal(t, awstypes.ServerSideEncryptionAwsKms, input.ServerSideEncryption)
					assert.Equal(t, "my-kms-key", aws.ToString(input.SSEKMSKeyId))

					return &s3.PutObjectOutput{
						ETag: aws.String("encrypted-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:   "simple upload failure",
			bucket: "test-bucket",
			key:    "test-key",
			data:   []byte("test data"),
			config: &s3types.UploadConfig{},
			mockFunc: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					return nil, errors.New("access denied")
				}
			},
			wantErr:     true,
			errContains: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &testutil.MockS3Client{}
			if tt.mockFunc != nil {
				tt.mockFunc(mockClient)
			}

			// Create uploader with mock client
			uploader := New(mockClient)

			ctx := context.Background()
			startTime := time.Now()

			// Test the uploadSimple method directly
			result, err := uploader.uploadSimple(
				ctx,
				tt.bucket,
				tt.key,
				tt.data,
				tt.config,
				startTime,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.key, result.Key)
				assert.Equal(t, int64(len(tt.data)), result.Size)
				assert.NotEmpty(t, result.ETag)
				assert.NotZero(t, result.Duration)
			}
		})
	}
}

// Test cases for multipart upload functionality
func TestUploader_Multipart_AutomaticDetection(t *testing.T) {
	tests := []struct {
		name            string
		contentSize     int64
		expectMultipart bool
		partSize        int64
	}{
		{
			name:            "small file uses simple upload",
			contentSize:     10 * 1024 * 1024, // 10MB
			expectMultipart: false,
			partSize:        5 * 1024 * 1024, // 5MB
		},
		{
			name:            "100MB file triggers multipart",
			contentSize:     100 * 1024 * 1024, // 100MB
			expectMultipart: true,
			partSize:        5 * 1024 * 1024, // 5MB
		},
		{
			name:            "large file triggers multipart",
			contentSize:     200 * 1024 * 1024, // 200MB
			expectMultipart: true,
			partSize:        10 * 1024 * 1024, // 10MB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := strings.Repeat("A", int(tt.contentSize))

			config := &s3types.UploadConfig{
				PartSize: tt.partSize,
			}

			mockClient := &testutil.MockS3Client{}
			if tt.expectMultipart {
				// Setup multipart mocks
				partCount := 0
				mockClient.CreateMultipartUploadFunc = func(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
					return &s3.CreateMultipartUploadOutput{
						UploadId: aws.String("test-upload-id"),
					}, nil
				}
				mockClient.UploadPartFunc = func(ctx context.Context, input *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
					partCount++
					return &s3.UploadPartOutput{
						ETag: aws.String(fmt.Sprintf("part-etag-%d", partCount)),
					}, nil
				}
				mockClient.CompleteMultipartUploadFunc = func(ctx context.Context, input *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
					// Verify that multiple parts were uploaded
					assert.Greater(
						t,
						len(input.MultipartUpload.Parts),
						1,
						"Expected multiple parts for multipart upload",
					)
					return &s3.CompleteMultipartUploadOutput{
						ETag: aws.String("multipart-etag"),
					}, nil
				}
			} else {
				// Setup simple upload mock
				mockClient.PutObjectFunc = func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					return &s3.PutObjectOutput{
						ETag: aws.String("simple-etag"),
					}, nil
				}
			}

			uploader := New(mockClient)
			ctx := context.Background()
			startTime := time.Now()

			result, err := uploader.Upload(
				ctx,
				"test-bucket",
				"test-key",
				strings.NewReader(content),
				config,
				startTime,
			)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.contentSize, result.Size)
		})
	}
}

func TestUploader_Multipart_ConcurrentUploads(t *testing.T) {
	// Create a large file that will be split into multiple parts
	contentSize := int64(150 * 1024 * 1024) // 150MB (> 100MB threshold)
	content := strings.Repeat("A", int(contentSize))
	partSize := int64(5 * 1024 * 1024)                            // 5MB parts
	expectedParts := int((contentSize + partSize - 1) / partSize) // Ceiling division

	t.Run("concurrent part uploads", func(t *testing.T) {
		mockClient := &testutil.MockS3Client{}
		uploadCount := 0
		var uploadTimes []time.Time

		mockClient.CreateMultipartUploadFunc = func(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
			return &s3.CreateMultipartUploadOutput{
				UploadId: aws.String("test-upload-id"),
			}, nil
		}

		mockClient.UploadPartFunc = func(ctx context.Context, input *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
			uploadCount++
			uploadTimes = append(uploadTimes, time.Now())

			// Simulate some processing time to test concurrency
			time.Sleep(10 * time.Millisecond)

			return &s3.UploadPartOutput{
				ETag: aws.String(fmt.Sprintf("part-etag-%d", aws.ToInt32(input.PartNumber))),
			}, nil
		}

		mockClient.CompleteMultipartUploadFunc = func(ctx context.Context, input *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
			assert.Equal(t, expectedParts, len(input.MultipartUpload.Parts))
			return &s3.CompleteMultipartUploadOutput{
				ETag: aws.String("multipart-etag"),
			}, nil
		}

		uploader := New(mockClient)
		config := &s3types.UploadConfig{
			PartSize:    partSize,
			Concurrency: 3, // Allow 3 concurrent uploads
		}

		ctx := context.Background()
		startTime := time.Now()

		result, err := uploader.Upload(
			ctx,
			"test-bucket",
			"test-key",
			strings.NewReader(content),
			config,
			startTime,
		)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, expectedParts, uploadCount)

		// Verify concurrency - parts should be uploaded concurrently
		if len(uploadTimes) >= 2 {
			// Check that not all uploads happened sequentially
			totalDuration := uploadTimes[len(uploadTimes)-1].Sub(uploadTimes[0])
			sequentialDuration := time.Duration(len(uploadTimes)-1) * 10 * time.Millisecond
			assert.Less(t, totalDuration, sequentialDuration, "Uploads should be concurrent")
		}
	})
}

func TestUploader_Multipart_ConfigurablePartSize(t *testing.T) {
	tests := []struct {
		name          string
		contentSize   int64
		partSize      int64
		expectedParts int
	}{
		{
			name:          "5MB parts for 15MB file",
			contentSize:   15 * 1024 * 1024,
			partSize:      5 * 1024 * 1024,
			expectedParts: 3,
		},
		{
			name:          "10MB parts for 25MB file",
			contentSize:   25 * 1024 * 1024,
			partSize:      10 * 1024 * 1024,
			expectedParts: 3, // 10MB + 10MB + 5MB
		},
		{
			name:          "8MB parts for 16MB file",
			contentSize:   16 * 1024 * 1024,
			partSize:      8 * 1024 * 1024,
			expectedParts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := strings.Repeat("A", int(tt.contentSize))

			mockClient := &testutil.MockS3Client{}
			uploadedParts := make([]int64, 0)

			mockClient.CreateMultipartUploadFunc = func(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
				return &s3.CreateMultipartUploadOutput{
					UploadId: aws.String("test-upload-id"),
				}, nil
			}

			mockClient.UploadPartFunc = func(ctx context.Context, input *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
				// Read the part data to verify size
				data, err := io.ReadAll(input.Body)
				require.NoError(t, err)
				uploadedParts = append(uploadedParts, int64(len(data)))

				return &s3.UploadPartOutput{
					ETag: aws.String(fmt.Sprintf("part-etag-%d", len(uploadedParts))),
				}, nil
			}

			mockClient.CompleteMultipartUploadFunc = func(ctx context.Context, input *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
				assert.Equal(t, tt.expectedParts, len(input.MultipartUpload.Parts))

				// Verify part sizes
				totalSize := int64(0)
				for i, partSize := range uploadedParts {
					if i < len(uploadedParts)-1 {
						// All parts except the last should be exactly partSize
						assert.Equal(
							t,
							tt.partSize,
							partSize,
							"Part %d should be exactly %d bytes",
							i+1,
							tt.partSize,
						)
					} else {
						// Last part can be smaller
						assert.LessOrEqual(t, partSize, tt.partSize, "Last part should not exceed partSize")
						assert.Greater(t, partSize, int64(0), "Last part should not be empty")
					}
					totalSize += partSize
				}
				assert.Equal(
					t,
					tt.contentSize,
					totalSize,
					"Total uploaded size should match original content",
				)

				return &s3.CompleteMultipartUploadOutput{
					ETag: aws.String("multipart-etag"),
				}, nil
			}

			uploader := New(mockClient)
			config := &s3types.UploadConfig{
				PartSize: tt.partSize,
			}

			ctx := context.Background()
			startTime := time.Now()

			result, err := uploader.Upload(
				ctx,
				"test-bucket",
				"test-key",
				strings.NewReader(content),
				config,
				startTime,
			)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.contentSize, result.Size)
		})
	}
}

func TestUploader_Multipart_ErrorRecovery(t *testing.T) {
	t.Run("abort on create multipart failure", func(t *testing.T) {
		contentSize := int64(150 * 1024 * 1024) // 150MB
		content := strings.Repeat("A", int(contentSize))

		mockClient := &testutil.MockS3Client{}
		mockClient.CreateMultipartUploadFunc = func(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
			return nil, errors.New("create multipart failed")
		}
		// AbortMultipartUpload should not be called since create failed
		mockClient.AbortMultipartUploadFunc = func(ctx context.Context, input *s3.AbortMultipartUploadInput, opts ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
			t.Error("AbortMultipartUpload should not be called when create fails")
			return nil, nil
		}

		uploader := New(mockClient)
		config := &s3types.UploadConfig{}

		ctx := context.Background()
		startTime := time.Now()

		_, err := uploader.Upload(
			ctx,
			"test-bucket",
			"test-key",
			strings.NewReader(content),
			config,
			startTime,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "create multipart failed")
	})

	t.Run("abort on part upload failure", func(t *testing.T) {
		contentSize := int64(150 * 1024 * 1024) // 150MB
		content := strings.Repeat("A", int(contentSize))

		mockClient := &testutil.MockS3Client{}
		partUploadCount := 0

		mockClient.CreateMultipartUploadFunc = func(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
			return &s3.CreateMultipartUploadOutput{
				UploadId: aws.String("test-upload-id"),
			}, nil
		}

		mockClient.UploadPartFunc = func(ctx context.Context, input *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
			partUploadCount++
			if partUploadCount == 3 { // Fail on third part
				return nil, errors.New("part upload failed")
			}
			return &s3.UploadPartOutput{
				ETag: aws.String(fmt.Sprintf("part-etag-%d", partUploadCount)),
			}, nil
		}

		abortCalled := false
		mockClient.AbortMultipartUploadFunc = func(ctx context.Context, input *s3.AbortMultipartUploadInput, opts ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
			abortCalled = true
			assert.Equal(t, "test-upload-id", aws.ToString(input.UploadId))
			return &s3.AbortMultipartUploadOutput{}, nil
		}

		mockClient.CompleteMultipartUploadFunc = func(ctx context.Context, input *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
			t.Error("CompleteMultipartUpload should not be called when part upload fails")
			return nil, nil
		}

		uploader := New(mockClient)
		config := &s3types.UploadConfig{
			PartSize: 5 * 1024 * 1024, // Small parts to trigger multiple uploads
		}

		ctx := context.Background()
		startTime := time.Now()

		_, err := uploader.Upload(
			ctx,
			"test-bucket",
			"test-key",
			strings.NewReader(content),
			config,
			startTime,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "part upload failed")
		assert.True(t, abortCalled, "AbortMultipartUpload should be called on part upload failure")
	})

	t.Run("abort on complete multipart failure", func(t *testing.T) {
		contentSize := int64(150 * 1024 * 1024) // 150MB
		content := strings.Repeat("A", int(contentSize))

		mockClient := &testutil.MockS3Client{}

		mockClient.CreateMultipartUploadFunc = func(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
			return &s3.CreateMultipartUploadOutput{
				UploadId: aws.String("test-upload-id"),
			}, nil
		}

		mockClient.UploadPartFunc = func(ctx context.Context, input *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
			return &s3.UploadPartOutput{
				ETag: aws.String("part-etag"),
			}, nil
		}

		mockClient.CompleteMultipartUploadFunc = func(ctx context.Context, input *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
			return nil, errors.New("complete multipart failed")
		}

		abortCalled := false
		mockClient.AbortMultipartUploadFunc = func(ctx context.Context, input *s3.AbortMultipartUploadInput, opts ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
			abortCalled = true
			assert.Equal(t, "test-upload-id", aws.ToString(input.UploadId))
			return &s3.AbortMultipartUploadOutput{}, nil
		}

		uploader := New(mockClient)
		config := &s3types.UploadConfig{
			PartSize: 5 * 1024 * 1024,
		}

		ctx := context.Background()
		startTime := time.Now()

		_, err := uploader.Upload(
			ctx,
			"test-bucket",
			"test-key",
			strings.NewReader(content),
			config,
			startTime,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "complete multipart failed")
		assert.True(t, abortCalled, "AbortMultipartUpload should be called on complete failure")
	})
}

// Helper function to create a temporary file for testing
func createTempFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	require.NoError(t, err)

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)

	err = tmpFile.Close()
	require.NoError(t, err)

	return tmpFile.Name()
}
