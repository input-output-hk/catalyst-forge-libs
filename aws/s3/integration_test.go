//go:build integration
// +build integration

package s3_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/errors"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/testutil"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// TestIntegrationUploadDownload tests upload and download operations against LocalStack.
func TestIntegrationUploadDownload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	s3Client, cleanup := testutil.SetupLocalStackTest(t)
	defer cleanup()

	bucketName := testutil.GenerateTestBucketName("integration")
	err := testutil.CreateTestBucketInLocalStack(ctx, s3Client, bucketName)
	require.NoError(t, err, "Failed to create test bucket")
	defer testutil.CleanupTestBucketInLocalStack(ctx, s3Client, bucketName)

	// Create S3 client wrapper
	client := s3.NewWithClient(s3Client)

	t.Run("Upload and Download bytes", func(t *testing.T) {
		key := testutil.GenerateTestKey("upload")
		testData := []byte("Hello, LocalStack!")

		// Upload
		err := client.Put(ctx, bucketName, key, testData)
		require.NoError(t, err)

		// Download
		downloadedData, err := client.Get(ctx, bucketName, key)
		require.NoError(t, err)
		assert.Equal(t, testData, downloadedData)
	})

	t.Run("Upload and Download stream", func(t *testing.T) {
		key := testutil.GenerateTestKey("stream")
		testData := testutil.GenerateRandomData(1024 * 10) // 10KB

		// Upload stream
		reader := bytes.NewReader(testData)
		_, err := client.Upload(ctx, bucketName, key, reader)
		require.NoError(t, err)

		// Download stream
		var buf bytes.Buffer
		_, err = client.Download(ctx, bucketName, key, &buf)
		require.NoError(t, err)
		assert.Equal(t, testData, buf.Bytes())
	})

	t.Run("Upload and Download file", func(t *testing.T) {
		key := testutil.GenerateTestKey("file")
		testData := testutil.GenerateRandomData(1024 * 100) // 100KB

		// Create temp file
		tempDir := t.TempDir()
		tempFile := filepath.Join(tempDir, "test-upload.bin")
		err := os.WriteFile(tempFile, testData, 0o644)
		require.NoError(t, err)

		// Upload file
		_, err = client.UploadFile(ctx, bucketName, key, tempFile)
		require.NoError(t, err)

		// Download file
		downloadFile := filepath.Join(tempDir, "test-download.bin")
		_, err = client.DownloadFile(ctx, bucketName, key, downloadFile)
		require.NoError(t, err)

		// Verify contents
		downloadedData, err := os.ReadFile(downloadFile)
		require.NoError(t, err)
		assert.Equal(t, testData, downloadedData)
	})
}

// TestIntegrationListOperations tests listing operations against LocalStack.
func TestIntegrationListOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	s3Client, cleanup := testutil.SetupLocalStackTest(t)
	defer cleanup()

	bucketName := testutil.GenerateTestBucketName("integration")
	err := testutil.CreateTestBucketInLocalStack(ctx, s3Client, bucketName)
	require.NoError(t, err)
	defer testutil.CleanupTestBucketInLocalStack(ctx, s3Client, bucketName)

	client := s3.NewWithClient(s3Client)

	// Upload test objects
	objectCount := 25
	for i := 0; i < objectCount; i++ {
		key := fmt.Sprintf("test-object-%03d.txt", i)
		err := client.Put(ctx, bucketName, key, []byte(fmt.Sprintf("content-%d", i)))
		require.NoError(t, err)
	}

	t.Run("List all objects", func(t *testing.T) {
		// List should return all objects
		result, err := client.List(ctx, bucketName, "")
		require.NoError(t, err)
		assert.Len(t, result.Objects, objectCount)
	})

	t.Run("ListAll with channel", func(t *testing.T) {
		objectChan := client.ListAll(ctx, bucketName, "")

		var objects []s3types.Object
		for obj := range objectChan {
			objects = append(objects, obj)
		}

		assert.Len(t, objects, objectCount)
	})

	t.Run("List with prefix", func(t *testing.T) {
		// Upload objects with prefix
		for i := 0; i < 5; i++ {
			key := fmt.Sprintf("prefix/sub-%03d.txt", i)
			err := client.Put(ctx, bucketName, key, []byte("content"))
			require.NoError(t, err)
		}

		result, err := client.List(ctx, bucketName, "prefix/")
		require.NoError(t, err)
		assert.Len(t, result.Objects, 5)
	})
}

// TestIntegrationManagementOperations tests management operations against LocalStack.
func TestIntegrationManagementOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	s3Client, cleanup := testutil.SetupLocalStackTest(t)
	defer cleanup()

	bucketName := testutil.GenerateTestBucketName("integration")
	err := testutil.CreateTestBucketInLocalStack(ctx, s3Client, bucketName)
	require.NoError(t, err)
	defer testutil.CleanupTestBucketInLocalStack(ctx, s3Client, bucketName)

	client := s3.NewWithClient(s3Client)

	t.Run("Delete single object", func(t *testing.T) {
		key := testutil.GenerateTestKey("delete")

		// Upload object
		err := client.Put(ctx, bucketName, key, []byte("to be deleted"))
		require.NoError(t, err)

		// Verify it exists
		exists, err := client.Exists(ctx, bucketName, key)
		require.NoError(t, err)
		assert.True(t, exists)

		// Delete object
		err = client.Delete(ctx, bucketName, key)
		require.NoError(t, err)

		// Verify it's gone
		exists, err = client.Exists(ctx, bucketName, key)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Delete multiple objects", func(t *testing.T) {
		// Upload multiple objects
		keys := make([]string, 10)
		for i := 0; i < 10; i++ {
			keys[i] = fmt.Sprintf("delete-many-%03d.txt", i)
			err := client.Put(ctx, bucketName, keys[i], []byte("content"))
			require.NoError(t, err)
		}

		// Delete all at once
		results, err := client.DeleteMany(ctx, bucketName, keys)
		require.NoError(t, err)
		assert.Len(t, results.Deleted, 10)
		assert.Empty(t, results.Errors)

		// Verify all are gone
		for _, key := range keys {
			exists, err := client.Exists(ctx, bucketName, key)
			require.NoError(t, err)
			assert.False(t, exists)
		}
	})

	t.Run("Copy object", func(t *testing.T) {
		sourceKey := testutil.GenerateTestKey("source")
		destKey := testutil.GenerateTestKey("dest")
		testData := []byte("copy me")

		// Upload source object
		err := client.Put(ctx, bucketName, sourceKey, testData)
		require.NoError(t, err)

		// Copy object
		err = client.Copy(ctx, bucketName, sourceKey, bucketName, destKey)
		require.NoError(t, err)

		// Verify copy
		copiedData, err := client.Get(ctx, bucketName, destKey)
		require.NoError(t, err)
		assert.Equal(t, testData, copiedData)
	})

	t.Run("Move object", func(t *testing.T) {
		sourceKey := testutil.GenerateTestKey("move-source")
		destKey := testutil.GenerateTestKey("move-dest")
		testData := []byte("move me")

		// Upload source object
		err := client.Put(ctx, bucketName, sourceKey, testData)
		require.NoError(t, err)

		// Move object
		err = client.Move(ctx, bucketName, sourceKey, bucketName, destKey)
		require.NoError(t, err)

		// Verify source is gone
		exists, err := client.Exists(ctx, bucketName, sourceKey)
		require.NoError(t, err)
		assert.False(t, exists)

		// Verify destination exists
		movedData, err := client.Get(ctx, bucketName, destKey)
		require.NoError(t, err)
		assert.Equal(t, testData, movedData)
	})

	t.Run("Get metadata", func(t *testing.T) {
		key := testutil.GenerateTestKey("metadata")
		testData := []byte("metadata test")

		// Upload with metadata
		opts := []s3types.UploadOption{
			s3.WithMetadata(map[string]string{
				"test-key": "test-value",
				"author":   "integration-test",
			}),
		}
		err := client.Put(ctx, bucketName, key, testData, opts...)
		require.NoError(t, err)

		// Get metadata
		metadata, err := client.GetMetadata(ctx, bucketName, key)
		require.NoError(t, err)
		assert.Equal(t, int64(len(testData)), metadata.ContentLength)
		assert.NotNil(t, metadata.LastModified)
		assert.Equal(t, "test-value", metadata.Metadata["test-key"])
		assert.Equal(t, "integration-test", metadata.Metadata["author"])
	})
}

// TestIntegrationBucketOperations tests bucket operations against LocalStack.
func TestIntegrationBucketOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	s3Client, cleanup := testutil.SetupLocalStackTest(t)
	defer cleanup()

	client := s3.NewWithClient(s3Client)

	t.Run("Create and delete bucket", func(t *testing.T) {
		bucketName := testutil.GenerateTestBucketName("create-delete")

		// Create bucket
		err := client.CreateBucket(ctx, bucketName)
		require.NoError(t, err)

		// Verify bucket exists by uploading an object
		err = client.Put(ctx, bucketName, "test.txt", []byte("test"))
		require.NoError(t, err)

		// Clean up objects first
		err = client.Delete(ctx, bucketName, "test.txt")
		require.NoError(t, err)

		// Delete bucket
		err = client.DeleteBucket(ctx, bucketName)
		require.NoError(t, err)
	})

	t.Run("Delete non-empty bucket fails", func(t *testing.T) {
		bucketName := testutil.GenerateTestBucketName("non-empty")

		// Create bucket
		err := client.CreateBucket(ctx, bucketName)
		require.NoError(t, err)
		defer func() {
			// Clean up
			client.Delete(ctx, bucketName, "object.txt")
			client.DeleteBucket(ctx, bucketName)
		}()

		// Add an object
		err = client.Put(ctx, bucketName, "object.txt", []byte("content"))
		require.NoError(t, err)

		// Try to delete non-empty bucket
		err = client.DeleteBucket(ctx, bucketName)
		assert.Error(t, err)
	})
}

// TestIntegrationMultipartUpload tests multipart upload against LocalStack.
func TestIntegrationMultipartUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	s3Client, cleanup := testutil.SetupLocalStackTest(t)
	defer cleanup()

	bucketName := testutil.GenerateTestBucketName("multipart")
	err := testutil.CreateTestBucketInLocalStack(ctx, s3Client, bucketName)
	require.NoError(t, err)
	defer testutil.CleanupTestBucketInLocalStack(ctx, s3Client, bucketName)

	client := s3.NewWithClient(s3Client)

	t.Run("Large file triggers multipart upload", func(t *testing.T) {
		key := testutil.GenerateTestKey("large")
		// Create data larger than multipart threshold (100MB)
		largeData := testutil.GenerateRandomData(110 * 1024 * 1024) // 110MB

		// Upload large data
		reader := bytes.NewReader(largeData)
		_, err := client.Upload(ctx, bucketName, key, reader)
		require.NoError(t, err)

		// Download and verify
		var buf bytes.Buffer
		_, err = client.Download(ctx, bucketName, key, &buf)
		require.NoError(t, err)
		assert.Equal(t, len(largeData), buf.Len())
	})
}

// TestIntegrationSyncOperations tests sync functionality against LocalStack.
func TestIntegrationSyncOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	s3Client, cleanup := testutil.SetupLocalStackTest(t)
	defer cleanup()

	bucketName := testutil.GenerateTestBucketName("sync")
	err := testutil.CreateTestBucketInLocalStack(ctx, s3Client, bucketName)
	require.NoError(t, err)
	defer testutil.CleanupTestBucketInLocalStack(ctx, s3Client, bucketName)

	client := s3.NewWithClient(s3Client)

	t.Run("Sync local directory to S3", func(t *testing.T) {
		// Create temp directory with files
		tempDir := t.TempDir()
		files := map[string][]byte{
			"file1.txt":        []byte("content1"),
			"file2.txt":        []byte("content2"),
			"dir/file3.txt":    []byte("content3"),
			"dir/subdir/f4.md": []byte("content4"),
		}

		for path, content := range files {
			fullPath := filepath.Join(tempDir, path)
			err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
			require.NoError(t, err)
			err = os.WriteFile(fullPath, content, 0o644)
			require.NoError(t, err)
		}

		// Sync to S3
		result, err := client.Sync(ctx, tempDir, bucketName, "sync-test/")
		require.NoError(t, err)
		assert.Equal(t, 4, result.FilesUploaded)
		assert.Equal(t, 0, result.FilesDeleted)
		assert.Empty(t, result.Errors)

		// Verify all files were uploaded
		listResult, err := client.List(ctx, bucketName, "sync-test/")
		require.NoError(t, err)
		assert.Len(t, listResult.Objects, 4)
	})

	t.Run("Sync with delete option", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create initial files
		file1 := filepath.Join(tempDir, "keep.txt")
		err := os.WriteFile(file1, []byte("keep me"), 0o644)
		require.NoError(t, err)

		// Upload extra file that should be deleted
		err = client.Put(ctx, bucketName, "delete-test/remove.txt", []byte("delete me"))
		require.NoError(t, err)

		// Sync with delete
		syncOpts := []s3types.SyncOption{
			s3.WithSyncDeleteExtra(true),
		}
		result, err := client.Sync(ctx, tempDir, bucketName, "delete-test/", syncOpts...)
		require.NoError(t, err)
		assert.Equal(t, 1, result.FilesUploaded)
		assert.Equal(t, 1, result.FilesDeleted)

		// Verify removed file is gone
		exists, err := client.Exists(ctx, bucketName, "delete-test/remove.txt")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

// TestIntegrationErrorScenarios tests error handling against LocalStack.
func TestIntegrationErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	s3Client, cleanup := testutil.SetupLocalStackTest(t)
	defer cleanup()

	client := s3.NewWithClient(s3Client)

	t.Run("Get non-existent object", func(t *testing.T) {
		bucketName := testutil.GenerateTestBucketName("errors")
		err := testutil.CreateTestBucketInLocalStack(ctx, s3Client, bucketName)
		require.NoError(t, err)
		defer testutil.CleanupTestBucketInLocalStack(ctx, s3Client, bucketName)

		// Try to get non-existent object
		_, err = client.Get(ctx, bucketName, "does-not-exist.txt")
		assert.Error(t, err)
		assert.True(t, errors.IsObjectNotFound(err))
	})

	t.Run("Upload to non-existent bucket", func(t *testing.T) {
		// Try to upload to non-existent bucket
		err := client.Put(ctx, "bucket-does-not-exist", "test.txt", []byte("test"))
		assert.Error(t, err)
	})

	t.Run("Delete non-existent object", func(t *testing.T) {
		bucketName := testutil.GenerateTestBucketName("errors")
		err := testutil.CreateTestBucketInLocalStack(ctx, s3Client, bucketName)
		require.NoError(t, err)
		defer testutil.CleanupTestBucketInLocalStack(ctx, s3Client, bucketName)

		// Delete non-existent object (should not error)
		err = client.Delete(ctx, bucketName, "does-not-exist.txt")
		assert.NoError(t, err) // S3 doesn't error on deleting non-existent objects
	})
}
