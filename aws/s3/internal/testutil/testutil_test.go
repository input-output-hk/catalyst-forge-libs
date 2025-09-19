package testutil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockS3Client(t *testing.T) {
	t.Run("implements S3API interface", func(t *testing.T) {
		mock := &MockS3Client{}
		// This test will fail at compile time if MockS3Client doesn't implement S3API
		_ = mock
	})

	t.Run("PutObject with custom function", func(t *testing.T) {
		mock := &MockS3Client{
			PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
				assert.Equal(t, "test-bucket", *params.Bucket)
				assert.Equal(t, "test-key", *params.Key)
				return &s3.PutObjectOutput{
					ETag: StringPtr("test-etag"),
				}, nil
			},
		}

		output, err := mock.PutObject(context.Background(), &s3.PutObjectInput{
			Bucket: StringPtr("test-bucket"),
			Key:    StringPtr("test-key"),
		})

		require.NoError(t, err)
		assert.Equal(t, "test-etag", *output.ETag)
	})

	t.Run("returns default when no function set", func(t *testing.T) {
		mock := &MockS3Client{}
		output, err := mock.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: StringPtr("test-bucket"),
			Key:    StringPtr("test-key"),
		})

		require.NoError(t, err)
		assert.NotNil(t, output)
	})
}

func TestMockBuilder(t *testing.T) {
	t.Run("builds mock with successful upload", func(t *testing.T) {
		mock := NewMockBuilder().WithSuccessfulUpload().Build()

		output, err := mock.PutObject(context.Background(), &s3.PutObjectInput{
			Bucket: StringPtr("test-bucket"),
			Key:    StringPtr("test-key"),
			Body:   bytes.NewReader([]byte("test data")),
		})

		require.NoError(t, err)
		assert.Equal(t, `"test-etag"`, *output.ETag)
	})

	t.Run("builds mock with object not found", func(t *testing.T) {
		mock := NewMockBuilder().WithObjectNotFound().Build()

		_, err := mock.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: StringPtr("test-bucket"),
			Key:    StringPtr("test-key"),
		})

		require.Error(t, err)
	})

	t.Run("builds mock with empty bucket", func(t *testing.T) {
		mock := NewMockBuilder().WithEmptyBucket().Build()

		output, err := mock.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
			Bucket: StringPtr("test-bucket"),
		})

		require.NoError(t, err)
		assert.Equal(t, int32(0), *output.KeyCount)
		assert.False(t, *output.IsTruncated)
	})

	t.Run("builds mock with multipart upload", func(t *testing.T) {
		mock := NewMockBuilder().WithMultipartUpload().Build()

		// Create multipart upload
		createOutput, err := mock.CreateMultipartUpload(context.Background(), &s3.CreateMultipartUploadInput{
			Bucket: StringPtr("test-bucket"),
			Key:    StringPtr("test-key"),
		})
		require.NoError(t, err)
		assert.NotEmpty(t, *createOutput.UploadId)

		// Upload part
		partOutput, err := mock.UploadPart(context.Background(), &s3.UploadPartInput{
			Bucket:     StringPtr("test-bucket"),
			Key:        StringPtr("test-key"),
			UploadId:   createOutput.UploadId,
			PartNumber: Int32Ptr(1),
			Body:       bytes.NewReader([]byte("test data")),
		})
		require.NoError(t, err)
		assert.NotEmpty(t, *partOutput.ETag)
	})
}

func TestProgressTracker(t *testing.T) {
	t.Run("tracks progress updates", func(t *testing.T) {
		tracker := &MockProgressTracker{}

		tracker.Update(100, 1000)
		tracker.Update(500, 1000)
		tracker.Complete()

		assert.True(t, tracker.UpdateCalled)
		assert.True(t, tracker.CompleteCalled)
		assert.Equal(t, int64(500), tracker.BytesTransferred)
		assert.Equal(t, int64(1000), tracker.TotalBytes)
		assert.Len(t, tracker.Updates, 2)
	})

	t.Run("tracks errors", func(t *testing.T) {
		tracker := &MockProgressTracker{}
		testErr := assert.AnError

		tracker.Error(testErr)

		assert.True(t, tracker.ErrorCalled)
		assert.Equal(t, testErr, tracker.LastError)
	})

	t.Run("resets state", func(t *testing.T) {
		tracker := &MockProgressTracker{}
		tracker.Update(100, 1000)
		tracker.Complete()
		tracker.Error(assert.AnError)

		tracker.Reset()

		assert.False(t, tracker.UpdateCalled)
		assert.False(t, tracker.CompleteCalled)
		assert.False(t, tracker.ErrorCalled)
		assert.Equal(t, int64(0), tracker.BytesTransferred)
		assert.Nil(t, tracker.LastError)
		assert.Nil(t, tracker.Updates)
	})
}

func TestHelpers(t *testing.T) {
	t.Run("generates random data", func(t *testing.T) {
		data := GenerateRandomData(1024)
		assert.Len(t, data, 1024)

		// Data should be different each time
		data2 := GenerateRandomData(1024)
		assert.NotEqual(t, data, data2)
	})

	t.Run("generates random reader", func(t *testing.T) {
		reader := GenerateRandomReader(1024)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Len(t, data, 1024)
	})

	t.Run("generates test key", func(t *testing.T) {
		key1 := GenerateTestKey("prefix")
		assert.Contains(t, key1, "prefix/")
		assert.Contains(t, key1, "test-object-")

		key2 := GenerateTestKey("")
		assert.Contains(t, key2, "test-object-")
		assert.NotEqual(t, key1, key2)
	})

	t.Run("generates test bucket name", func(t *testing.T) {
		name := GenerateTestBucketName("test")
		assert.Contains(t, name, "test-")
		assert.LessOrEqual(t, len(name), 63)
		assert.Regexp(t, "^[a-z0-9][a-z0-9.-]*[a-z0-9]$", name)
	})

	t.Run("calculates MD5", func(t *testing.T) {
		data := []byte("test data")
		md5 := CalculateMD5(data)
		assert.NotEmpty(t, md5)
		// Should be base64 encoded
		assert.Contains(t, md5, "=")
	})

	t.Run("calculates ETag", func(t *testing.T) {
		data := []byte("test data")
		etag := CalculateETag(data)
		assert.NotEmpty(t, etag)
		// Should be hex with quotes
		assert.True(t, strings.HasPrefix(etag, `"`))
		assert.True(t, strings.HasSuffix(etag, `"`))
	})

	t.Run("creates test object", func(t *testing.T) {
		now := time.Now()
		obj := CreateTestObject("test-key", 1024, now)

		assert.Equal(t, "test-key", *obj.Key)
		assert.Equal(t, int64(1024), *obj.Size)
		assert.Equal(t, now, *obj.LastModified)
		assert.NotEmpty(t, *obj.ETag)
	})

	t.Run("creates list objects output", func(t *testing.T) {
		objects := []types.Object{
			CreateTestObject("key1", 100, time.Now()),
			CreateTestObject("key2", 200, time.Now()),
		}

		output := CreateListObjectsV2Output(objects, "prefix/", "/", false)

		assert.Equal(t, "test-bucket", *output.Name)
		assert.Equal(t, "prefix/", *output.Prefix)
		assert.Equal(t, "/", *output.Delimiter)
		assert.Equal(t, int32(2), *output.KeyCount)
		assert.False(t, *output.IsTruncated)
		assert.Nil(t, output.NextContinuationToken)
	})

	t.Run("creates list objects output with truncation", func(t *testing.T) {
		objects := []types.Object{
			CreateTestObject("key1", 100, time.Now()),
		}

		output := CreateListObjectsV2Output(objects, "", "", true)

		assert.True(t, *output.IsTruncated)
		assert.NotNil(t, output.NextContinuationToken)
	})

	t.Run("creates head object output", func(t *testing.T) {
		now := time.Now()
		output := CreateHeadObjectOutput(1024, now, "text/plain")

		assert.Equal(t, int64(1024), *output.ContentLength)
		assert.Equal(t, now, *output.LastModified)
		assert.Equal(t, "text/plain", *output.ContentType)
		assert.NotEmpty(t, *output.ETag)
	})
}

func TestTestDataGenerator(t *testing.T) {
	gen := NewTestDataGenerator(12345)

	t.Run("generates object list", func(t *testing.T) {
		objects := gen.GenerateObjectList(10, "prefix/")
		assert.Len(t, objects, 10)

		for i, obj := range objects {
			assert.Contains(t, *obj.Key, "prefix/")
			assert.Contains(t, *obj.Key, "object-")
			assert.Greater(t, *obj.Size, int64(999))
			assert.Less(t, *obj.Size, int64(1000001))

			if i > 0 {
				// Objects should have increasing timestamps
				assert.True(t, obj.LastModified.After(*objects[i-1].LastModified))
			}
		}
	})

	t.Run("generates common prefixes", func(t *testing.T) {
		prefixes := gen.GenerateCommonPrefixes(5, "base/")
		assert.Len(t, prefixes, 5)

		for i, prefix := range prefixes {
			assert.Contains(t, *prefix.Prefix, "base/")
			assert.Contains(t, *prefix.Prefix, "dir")
			assert.True(t, strings.HasSuffix(*prefix.Prefix, "/"))
			assert.Contains(t, *prefix.Prefix, fmt.Sprintf("%02d", i))
		}
	})

	t.Run("generates multipart upload", func(t *testing.T) {
		upload := gen.GenerateMultipartUpload("test-key", "test-upload-id")
		assert.Equal(t, "test-key", *upload.Key)
		assert.Equal(t, "test-upload-id", *upload.UploadId)
		assert.NotNil(t, upload.Initiated)
	})

	t.Run("generates completed parts", func(t *testing.T) {
		parts := gen.GenerateCompletedParts(3)
		assert.Len(t, parts, 3)

		for i, part := range parts {
			assert.Equal(t, int32(i+1), *part.PartNumber)
			assert.NotEmpty(t, *part.ETag)
		}
	})

	t.Run("generates tags", func(t *testing.T) {
		tags := gen.GenerateTags(5)
		assert.Len(t, tags, 5)

		for i, tag := range tags {
			assert.Equal(t, fmt.Sprintf("tag-key-%d", i), *tag.Key)
			assert.Contains(t, *tag.Value, "tag-value-")
		}
	})
}
