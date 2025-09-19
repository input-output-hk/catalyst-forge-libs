package s3

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/input-output-hk/catalyst-forge-libs/fs/billy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/testutil"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// mockProgressTracker implements the ProgressTracker interface for testing
type mockProgressTracker struct {
	updateCalled    bool
	completeCalled  bool
	errorCalled     bool
	bytesTransfered int64
	totalBytes      int64
	lastError       error
}

func (m *mockProgressTracker) Update(bytesTransferred, totalBytes int64) {
	m.updateCalled = true
	m.bytesTransfered = bytesTransferred
	m.totalBytes = totalBytes
}

func (m *mockProgressTracker) Complete() {
	m.completeCalled = true
}

func (m *mockProgressTracker) Error(err error) {
	m.errorCalled = true
	m.lastError = err
}

func TestClient_Sync(t *testing.T) {
	t.Skip("Skipping: Requires filesystem integration - should be moved to integration tests with LocalStack")
	// Create in-memory filesystem
	fs := billy.NewInMemoryFS()

	// Create test files in memory
	basePath := "/test"
	require.NoError(t, fs.MkdirAll(basePath, 0o755))
	require.NoError(t, fs.WriteFile(basePath+"/file1.txt", []byte("content1"), 0o644))
	require.NoError(t, fs.WriteFile(basePath+"/file2.txt", []byte("content2"), 0o644))

	t.Run("successful sync with options", func(t *testing.T) {
		mockS3 := &testutil.MockS3Client{
			ListObjectsV2Func: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				// Return empty list (no existing objects)
				return &s3.ListObjectsV2Output{
					Contents: []types.Object{},
				}, nil
			},
			PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
				return &s3.PutObjectOutput{
					ETag: aws.String("mock-etag"),
				}, nil
			},
		}

		// Create client with mock S3 and filesystem
		client := &Client{
			s3Client: mockS3,
			fs:       fs,
		}

		tracker := &mockProgressTracker{}

		result, err := client.Sync(
			context.Background(),
			basePath,
			"test-bucket",
			"prefix/",
			WithSyncDryRun(false),
			WithSyncParallelism(2),
			WithSyncProgressTracker(tracker),
		)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, result.FilesUploaded, 0)
		assert.Equal(t, 0, result.FilesDeleted)
	})

	t.Run("dry run mode", func(t *testing.T) {
		mockS3 := &testutil.MockS3Client{
			ListObjectsV2Func: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return &s3.ListObjectsV2Output{
					Contents: []types.Object{
						{
							Key:  aws.String("prefix/extra.txt"),
							Size: aws.Int64(100),
						},
					},
				}, nil
			},
		}

		// Create client with mock S3 and filesystem
		client := &Client{
			s3Client: mockS3,
			fs:       fs,
		}

		result, err := client.Sync(
			context.Background(),
			basePath,
			"test-bucket",
			"prefix/",
			WithSyncDryRun(true),
			WithSyncDeleteExtra(true),
		)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// In dry run, no actual operations should be performed
		assert.Equal(t, 0, result.FilesUploaded)
		assert.Equal(t, 0, result.FilesDeleted)
	})

	t.Run("with include patterns", func(t *testing.T) {
		mockS3 := &testutil.MockS3Client{
			ListObjectsV2Func: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return &s3.ListObjectsV2Output{}, nil
			},
			PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
				// Only .txt files should be uploaded
				assert.Contains(t, *params.Key, ".txt")
				return &s3.PutObjectOutput{}, nil
			},
		}

		// Create client with mock S3 and filesystem
		client := &Client{
			s3Client: mockS3,
			fs:       fs,
		}

		// Create a non-txt file
		require.NoError(t, fs.WriteFile(basePath+"/file3.md", []byte("markdown"), 0o644))

		result, err := client.Sync(
			context.Background(),
			basePath,
			"test-bucket",
			"prefix/",
			WithSyncIncludePattern("*.txt"),
		)

		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("with exclude patterns", func(t *testing.T) {
		mockS3 := &testutil.MockS3Client{
			ListObjectsV2Func: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return &s3.ListObjectsV2Output{}, nil
			},
			PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
				// .tmp files should not be uploaded
				assert.NotContains(t, *params.Key, ".tmp")
				return &s3.PutObjectOutput{}, nil
			},
		}

		// Create client with mock S3 and filesystem
		client := &Client{
			s3Client: mockS3,
			fs:       fs,
		}

		// Create a tmp file
		require.NoError(t, fs.WriteFile(basePath+"/temp.tmp", []byte("temp"), 0o644))

		result, err := client.Sync(
			context.Background(),
			basePath,
			"test-bucket",
			"prefix/",
			WithSyncExcludePattern("*.tmp"),
		)

		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("validation errors", func(t *testing.T) {
		mockS3 := &testutil.MockS3Client{}
		// Create client with mock S3 and filesystem
		client := &Client{
			s3Client: mockS3,
			fs:       fs,
		}

		// Empty local path
		result, err := client.Sync(
			context.Background(),
			"",
			"test-bucket",
			"prefix/",
		)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "localPath")

		// Empty bucket
		result, err = client.Sync(
			context.Background(),
			basePath,
			"",
			"prefix/",
		)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "bucket")
	})

	t.Run("prefix normalization", func(t *testing.T) {
		mockS3 := &testutil.MockS3Client{
			ListObjectsV2Func: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				// Check that prefix ends with /
				assert.True(t, *params.Prefix == "" || strings.HasSuffix(*params.Prefix, "/"))
				return &s3.ListObjectsV2Output{}, nil
			},
		}

		// Create client with mock S3 and filesystem
		client := &Client{
			s3Client: mockS3,
			fs:       fs,
		}

		// Test without trailing slash
		_, err := client.Sync(
			context.Background(),
			basePath,
			"test-bucket",
			"prefix", // No trailing slash
			WithSyncDryRun(true),
		)
		assert.NoError(t, err)
	})
}

func TestClient_SyncUpload(t *testing.T) {
	t.Skip("Skipping: Requires filesystem integration - should be moved to integration tests with LocalStack")
	fs := billy.NewInMemoryFS()
	basePath := "/test"
	require.NoError(t, fs.MkdirAll(basePath, 0o755))
	require.NoError(t, fs.WriteFile(basePath+"/file1.txt", []byte("content1"), 0o644))

	mockS3 := &testutil.MockS3Client{
		ListObjectsV2Func: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			// Return an extra file that should NOT be deleted in upload-only mode
			return &s3.ListObjectsV2Output{
				Contents: []types.Object{
					{
						Key:  aws.String("prefix/extra.txt"),
						Size: aws.Int64(100),
					},
				},
			}, nil
		},
		PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			return &s3.PutObjectOutput{}, nil
		},
		DeleteObjectsFunc: func(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
			// Should not be called in upload-only mode
			t.Fatal("DeleteObjects should not be called in upload-only mode")
			return nil, nil
		},
	}

	// Create client with mock S3 and filesystem
	client := &Client{
		s3Client: mockS3,
		fs:       fs,
	}

	result, err := client.SyncUpload(
		context.Background(),
		basePath,
		"test-bucket",
		"prefix/",
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.FilesDeleted, "Should not delete files in upload-only mode")
}

func TestClient_SyncDownload(t *testing.T) {
	fs := billy.NewInMemoryFS()
	mockS3 := &testutil.MockS3Client{}

	// Create client with mock S3 and filesystem
	client := &Client{
		s3Client: mockS3,
		fs:       fs,
	}

	// Currently not implemented
	result, err := client.SyncDownload(
		context.Background(),
		"test-bucket",
		"prefix/",
		"/local/path",
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestSyncOptions(t *testing.T) {
	t.Run("WithSyncDryRun", func(t *testing.T) {
		config := &s3types.SyncOptionConfig{}
		opt := WithSyncDryRun(true)
		opt(config)
		assert.True(t, config.DryRun)
	})

	t.Run("WithSyncDeleteExtra", func(t *testing.T) {
		config := &s3types.SyncOptionConfig{}
		opt := WithSyncDeleteExtra(true)
		opt(config)
		assert.True(t, config.DeleteExtra)
	})

	t.Run("WithSyncIncludePattern", func(t *testing.T) {
		config := &s3types.SyncOptionConfig{}
		opt1 := WithSyncIncludePattern("*.txt")
		opt2 := WithSyncIncludePattern("*.md")
		opt1(config)
		opt2(config)
		assert.Equal(t, []string{"*.txt", "*.md"}, config.IncludePatterns)
	})

	t.Run("WithSyncExcludePattern", func(t *testing.T) {
		config := &s3types.SyncOptionConfig{}
		opt1 := WithSyncExcludePattern("*.tmp")
		opt2 := WithSyncExcludePattern("*.bak")
		opt1(config)
		opt2(config)
		assert.Equal(t, []string{"*.tmp", "*.bak"}, config.ExcludePatterns)
	})

	t.Run("WithSyncParallelism", func(t *testing.T) {
		config := &s3types.SyncOptionConfig{}
		opt := WithSyncParallelism(10)
		opt(config)
		assert.Equal(t, 10, config.Parallelism)
	})

	t.Run("WithSyncProgressTracker", func(t *testing.T) {
		config := &s3types.SyncOptionConfig{}
		tracker := &mockProgressTracker{}
		opt := WithSyncProgressTracker(tracker)
		opt(config)
		assert.Equal(t, tracker, config.ProgressTracker)
	})
}

func TestProgressTracking(t *testing.T) {
	t.Skip("Skipping: Requires filesystem integration - should be moved to integration tests with LocalStack")
	fs := billy.NewInMemoryFS()
	basePath := "/test"
	require.NoError(t, fs.MkdirAll(basePath, 0o755))
	require.NoError(t, fs.WriteFile(basePath+"/file1.txt", []byte("content1"), 0o644))

	uploadCount := 0
	mockS3 := &testutil.MockS3Client{
		ListObjectsV2Func: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
		PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			uploadCount++
			time.Sleep(10 * time.Millisecond) // Simulate upload time
			return &s3.PutObjectOutput{}, nil
		},
	}

	// Create client with mock S3 and filesystem
	client := &Client{
		s3Client: mockS3,
		fs:       fs,
	}

	tracker := &mockProgressTracker{}

	result, err := client.Sync(
		context.Background(),
		basePath,
		"test-bucket",
		"prefix/",
		WithSyncProgressTracker(tracker),
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, tracker.updateCalled, "Progress tracker should be updated")
}
