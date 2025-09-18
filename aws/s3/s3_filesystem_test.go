// Package s3 provides tests for filesystem integration.
package s3

import (
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/testutil"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// TestClient_UploadFile_WithMemoryFS tests UploadFile with an in-memory filesystem.
func TestClient_UploadFile_WithMemoryFS(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		filepath    string
		fileContent string
		setupFS     func(*billy.FS) error
		setupMock   func(*testutil.MockS3Client)
		opts        []s3types.UploadOption
		wantErr     bool
		errContains string
	}{
		{
			name:        "successful file upload from memory fs",
			bucket:      "test-bucket",
			key:         "test-key",
			filepath:    "/test/file.txt",
			fileContent: "Hello from memory filesystem!",
			setupFS: func(fs *billy.FS) error {
				// Create directory structure
				if err := fs.MkdirAll("/test", 0o755); err != nil {
					return err
				}
				// Write file
				return fs.WriteFile("/test/file.txt", []byte("Hello from memory filesystem!"), 0o644)
			},
			setupMock: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify the input parameters
					assert.Equal(t, "test-bucket", aws.ToString(params.Bucket))
					assert.Equal(t, "test-key", aws.ToString(params.Key))

					// Read the body to verify content
					body, err := io.ReadAll(params.Body)
					require.NoError(t, err)
					assert.Equal(t, "Hello from memory filesystem!", string(body))

					return &s3.PutObjectOutput{
						ETag:      aws.String("mock-etag-memory"),
						VersionId: aws.String("v1"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:        "upload with JSON file and auto content type detection",
			bucket:      "test-bucket",
			key:         "data.json",
			filepath:    "/data.json",
			fileContent: `{"name": "test", "value": 123}`,
			setupFS: func(fs *billy.FS) error {
				return fs.WriteFile("/data.json", []byte(`{"name": "test", "value": 123}`), 0o644)
			},
			setupMock: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify content type was auto-detected
					contentType := aws.ToString(params.ContentType)
					// Should be application/json or fall back to extension-based detection
					assert.Contains(t, contentType, "json")

					return &s3.PutObjectOutput{
						ETag: aws.String("mock-etag-json"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:        "file not found in memory fs",
			bucket:      "test-bucket",
			key:         "test-key",
			filepath:    "/nonexistent.txt",
			fileContent: "",
			setupFS: func(fs *billy.FS) error {
				// Don't create the file
				return nil
			},
			setupMock: func(m *testutil.MockS3Client) {
				// Should not be called
			},
			wantErr:     true,
			errContains: "file does not exist",
		},
		{
			name:        "upload directory instead of file",
			bucket:      "test-bucket",
			key:         "test-key",
			filepath:    "/testdir",
			fileContent: "",
			setupFS: func(fs *billy.FS) error {
				return fs.MkdirAll("/testdir", 0o755)
			},
			setupMock: func(m *testutil.MockS3Client) {
				// Should not be called
			},
			wantErr:     true,
			errContains: "points to a directory",
		},
		{
			name:        "upload with custom metadata",
			bucket:      "test-bucket",
			key:         "test-key",
			filepath:    "/metadata.txt",
			fileContent: "file with metadata",
			setupFS: func(fs *billy.FS) error {
				return fs.WriteFile("/metadata.txt", []byte("file with metadata"), 0o644)
			},
			setupMock: func(m *testutil.MockS3Client) {
				m.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					// Verify metadata
					assert.NotNil(t, params.Metadata)
					assert.Equal(t, "test-user", params.Metadata["uploaded-by"])
					assert.Equal(t, "memory-fs", params.Metadata["source"])

					return &s3.PutObjectOutput{
						ETag: aws.String("mock-etag-metadata"),
					}, nil
				}
			},
			opts: []s3types.UploadOption{
				WithMetadata(map[string]string{
					"uploaded-by": "test-user",
					"source":      "memory-fs",
				}),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create in-memory filesystem
			memFS := billy.NewInMemoryFS()

			// Setup filesystem
			if tt.setupFS != nil {
				err := tt.setupFS(memFS)
				require.NoError(t, err, "Failed to setup filesystem")
			}

			// Create mock S3 client
			mockClient := &testutil.MockS3Client{}
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			// Create S3 client with memory filesystem
			client := NewWithClient(mockClient)
			client.SetFilesystem(memFS)

			// Perform upload
			ctx := context.Background()
			result, err := client.UploadFile(ctx, tt.bucket, tt.key, tt.filepath, tt.opts...)

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
				assert.Equal(t, int64(len(tt.fileContent)), result.Size)
				assert.NotEmpty(t, result.ETag)
			}
		})
	}
}

// TestClient_ContentTypeDetection_WithMemoryFS tests content type detection with memory filesystem.
func TestClient_ContentTypeDetection_WithMemoryFS(t *testing.T) {
	tests := []struct {
		name            string
		filepath        string
		fileContent     []byte
		expectedType    string
		expectedPartial string // For partial matching when exact type may vary
	}{
		{
			name:         "detect JSON from content",
			filepath:     "/test.json",
			fileContent:  []byte(`{"valid": "json"}`),
			expectedType: "application/json",
		},
		{
			name:         "detect text from content",
			filepath:     "/readme.txt",
			fileContent:  []byte("This is plain text content"),
			expectedType: "text/plain; charset=utf-8",
		},
		{
			name:            "detect HTML from content",
			filepath:        "/index.html",
			fileContent:     []byte("<!DOCTYPE html><html><body>Hello</body></html>"),
			expectedPartial: "html",
		},
		{
			name:         "fallback to extension for unknown content",
			filepath:     "/script.js",
			fileContent:  []byte("console.log('test');"),
			expectedType: "text/plain; charset=utf-8",
		},
		{
			name:            "detect PDF from magic bytes",
			filepath:        "/document.pdf",
			fileContent:     []byte("%PDF-1.5\n%����\n"),
			expectedPartial: "pdf",
		},
		{
			name:         "default to octet-stream for unknown",
			filepath:     "/unknown.xyz",
			fileContent:  []byte{0x00, 0x01, 0x02, 0x03},
			expectedType: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create in-memory filesystem
			memFS := billy.NewInMemoryFS()

			// Create directory if needed
			dir := "/test"
			if tt.filepath == "/test.json" || tt.filepath == "/readme.txt" {
				err := memFS.MkdirAll(dir, 0o755)
				require.NoError(t, err)
			}

			// Write file
			err := memFS.WriteFile(tt.filepath, tt.fileContent, 0o644)
			require.NoError(t, err)

			// Create mock S3 client
			mockClient := &testutil.MockS3Client{}

			// Create S3 client with memory filesystem
			client := NewWithClient(mockClient)
			client.SetFilesystem(memFS)

			// Detect content type
			contentType := client.detectContentType(tt.filepath)

			// Verify
			if tt.expectedType != "" {
				assert.Equal(t, tt.expectedType, contentType)
			} else if tt.expectedPartial != "" {
				assert.Contains(t, contentType, tt.expectedPartial)
			}
		})
	}
}

// TestClient_WithCustomFilesystemOption tests the WithFilesystem option.
func TestClient_WithCustomFilesystemOption(t *testing.T) {
	// Create in-memory filesystem
	memFS := billy.NewInMemoryFS()

	// Create a test file
	err := memFS.WriteFile("/test.txt", []byte("custom fs content"), 0o644)
	require.NoError(t, err)

	// Create mock S3 client
	mockClient := &testutil.MockS3Client{
		PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			// Verify the content
			body, readErr := io.ReadAll(params.Body)
			require.NoError(t, readErr)
			assert.Equal(t, "custom fs content", string(body))

			return &s3.PutObjectOutput{
				ETag: aws.String("mock-etag"),
			}, nil
		},
	}

	// Create client with filesystem option
	client, err := New(
		WithFilesystem(memFS),
		WithRegion("us-west-2"),
	)
	require.NoError(t, err)

	// Replace the s3Client with our mock
	client.s3Client = mockClient

	// Upload file
	ctx := context.Background()
	result, err := client.UploadFile(ctx, "test-bucket", "test-key", "/test.txt")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-key", result.Key)
	assert.Equal(t, int64(17), result.Size) // "custom fs content" = 17 bytes
}
