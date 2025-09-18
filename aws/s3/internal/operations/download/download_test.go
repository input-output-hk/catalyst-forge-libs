// Package download provides unit tests for S3 download operations.
package download

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
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

// mockProgressTracker is a test implementation of ProgressTracker
type mockProgressTracker struct {
	updates   []progressUpdate
	completed bool
	error     error
}

type progressUpdate struct {
	bytesTransferred, totalBytes int64
}

func (m *mockProgressTracker) Update(bytesTransferred, totalBytes int64) {
	m.updates = append(m.updates, progressUpdate{bytesTransferred, totalBytes})
}

func (m *mockProgressTracker) Complete() {
	m.completed = true
}

func (m *mockProgressTracker) Error(err error) {
	m.error = err
}

func TestDownloader_Download_Stream(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		content     string
		config      *s3types.DownloadConfig
		mockFunc    func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful stream download",
			bucket:  "test-bucket",
			key:     "test-key",
			content: "Hello, World!",
			config:  &s3types.DownloadConfig{},
			mockFunc: func(m *testutil.MockS3Client) {
				m.GetObjectFunc = func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					// Verify input
					assert.Equal(t, "test-bucket", aws.ToString(input.Bucket))
					assert.Equal(t, "test-key", aws.ToString(input.Key))

					// Return mock response
					return &s3.GetObjectOutput{
						Body:          io.NopCloser(strings.NewReader("Hello, World!")),
						ContentLength: aws.Int64(int64(len("Hello, World!"))),
						ETag:          aws.String("test-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "download with progress tracking",
			bucket:  "test-bucket",
			key:     "test-key",
			content: "test content",
			config: &s3types.DownloadConfig{
				ProgressTracker: &mockProgressTracker{},
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.GetObjectFunc = func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					return &s3.GetObjectOutput{
						Body:          io.NopCloser(strings.NewReader("test content")),
						ContentLength: aws.Int64(int64(len("test content"))),
						ETag:          aws.String("test-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "download with range",
			bucket:  "test-bucket",
			key:     "test-key",
			content: "Hello", // Range "bytes=0-4" returns first 5 bytes
			config: &s3types.DownloadConfig{
				RangeSpec: "bytes=0-4",
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.GetObjectFunc = func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					assert.Equal(t, "bytes=0-4", aws.ToString(input.Range))
					return &s3.GetObjectOutput{
						Body:          io.NopCloser(strings.NewReader("Hello")),
						ContentLength: aws.Int64(5),
						ETag:          aws.String("test-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "download object not found",
			bucket:  "test-bucket",
			key:     "non-existent-key",
			content: "",
			config:  &s3types.DownloadConfig{},
			mockFunc: func(m *testutil.MockS3Client) {
				m.GetObjectFunc = func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					return nil, errors.New("NoSuchKey: The specified key does not exist")
				}
			},
			wantErr:     true,
			errContains: "object not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock client
			mockClient := &testutil.MockS3Client{}
			if tt.mockFunc != nil {
				tt.mockFunc(mockClient)
			}

			// Create downloader
			downloader := New(mockClient)

			// Setup writer
			var buf bytes.Buffer

			// Perform download
			startTime := time.Now()
			result, err := downloader.Download(context.Background(), tt.bucket, tt.key, &buf, tt.config, startTime)

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.key, result.Key)
			assert.Equal(t, tt.content, buf.String())

			// Check progress tracking if enabled
			if tt.config.ProgressTracker != nil {
				tracker := tt.config.ProgressTracker.(*mockProgressTracker)
				assert.True(t, tracker.completed)
				assert.NotEmpty(t, tracker.updates)
			}
		})
	}
}

func TestDownloader_DownloadFile(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		filepath    string
		content     string
		config      *s3types.DownloadConfig
		mockFunc    func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:     "successful file download",
			bucket:   "test-bucket",
			key:      "test-key",
			filepath: "/tmp/test-download.txt",
			content:  "Hello, World!",
			config:   &s3types.DownloadConfig{},
			mockFunc: func(m *testutil.MockS3Client) {
				m.GetObjectFunc = func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					return &s3.GetObjectOutput{
						Body:          io.NopCloser(strings.NewReader("Hello, World!")),
						ContentLength: aws.Int64(int64(len("Hello, World!"))),
						ETag:          aws.String("test-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:     "file download with progress",
			bucket:   "test-bucket",
			key:      "test-key",
			filepath: "/tmp/test-download-progress.txt",
			content:  "progress test content",
			config: &s3types.DownloadConfig{
				ProgressTracker: &mockProgressTracker{},
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.GetObjectFunc = func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					return &s3.GetObjectOutput{
						Body:          io.NopCloser(strings.NewReader("progress test content")),
						ContentLength: aws.Int64(int64(len("progress test content"))),
						ETag:          aws.String("test-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock client
			mockClient := &testutil.MockS3Client{}
			if tt.mockFunc != nil {
				tt.mockFunc(mockClient)
			}

			// Create downloader
			downloader := New(mockClient)

			// Perform download
			startTime := time.Now()
			result, err := downloader.DownloadFile(
				context.Background(),
				tt.bucket,
				tt.key,
				tt.filepath,
				tt.config,
				startTime,
			)

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.key, result.Key)

			// Verify file content
			content, err := os.ReadFile(tt.filepath)
			require.NoError(t, err)
			assert.Equal(t, tt.content, string(content))

			// Clean up
			os.Remove(tt.filepath)

			// Check progress tracking if enabled
			if tt.config.ProgressTracker != nil {
				tracker := tt.config.ProgressTracker.(*mockProgressTracker)
				assert.True(t, tracker.completed)
			}
		})
	}
}

func TestDownloader_Get(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		key         string
		content     string
		config      *s3types.DownloadConfig
		mockFunc    func(*testutil.MockS3Client)
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful get operation",
			bucket:  "test-bucket",
			key:     "test-key",
			content: "Hello, World!",
			config:  &s3types.DownloadConfig{},
			mockFunc: func(m *testutil.MockS3Client) {
				m.GetObjectFunc = func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					return &s3.GetObjectOutput{
						Body:          io.NopCloser(strings.NewReader("Hello, World!")),
						ContentLength: aws.Int64(int64(len("Hello, World!"))),
						ETag:          aws.String("test-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "get with progress tracking",
			bucket:  "test-bucket",
			key:     "test-key",
			content: "progress content",
			config: &s3types.DownloadConfig{
				ProgressTracker: &mockProgressTracker{},
			},
			mockFunc: func(m *testutil.MockS3Client) {
				m.GetObjectFunc = func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					return &s3.GetObjectOutput{
						Body:          io.NopCloser(strings.NewReader("progress content")),
						ContentLength: aws.Int64(int64(len("progress content"))),
						ETag:          aws.String("test-etag"),
					}, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "get object not found",
			bucket:  "test-bucket",
			key:     "non-existent-key",
			content: "",
			config:  &s3types.DownloadConfig{},
			mockFunc: func(m *testutil.MockS3Client) {
				m.GetObjectFunc = func(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
					return nil, errors.New("NoSuchKey: The specified key does not exist")
				}
			},
			wantErr:     true,
			errContains: "object not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock client
			mockClient := &testutil.MockS3Client{}
			if tt.mockFunc != nil {
				tt.mockFunc(mockClient)
			}

			// Create downloader
			downloader := New(mockClient)

			// Perform get operation
			startTime := time.Now()
			data, err := downloader.Get(context.Background(), tt.bucket, tt.key, tt.config, startTime)

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.content, string(data))

			// Check progress tracking if enabled
			if tt.config.ProgressTracker != nil {
				tracker := tt.config.ProgressTracker.(*mockProgressTracker)
				assert.True(t, tracker.completed)
			}
		})
	}
}
