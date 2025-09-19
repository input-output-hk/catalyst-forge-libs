// Package testutil provides test helper functions.
package testutil

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// StringPtr returns a pointer to the given string.
// This is useful for AWS SDK inputs that require string pointers.
func StringPtr(s string) *string {
	return aws.String(s)
}

// Int64Ptr returns a pointer to the given int64.
// This is useful for AWS SDK inputs that require int64 pointers.
func Int64Ptr(i int64) *int64 {
	return aws.Int64(i)
}

// Int32Ptr returns a pointer to the given int32.
// This is useful for AWS SDK inputs that require int32 pointers.
func Int32Ptr(i int32) *int32 {
	return aws.Int32(i)
}

// BoolPtr returns a pointer to the given bool.
// This is useful for AWS SDK inputs that require bool pointers.
func BoolPtr(b bool) *bool {
	return aws.Bool(b)
}

// TimePtr returns a pointer to the given time.
// This is useful for AWS SDK outputs that return time pointers.
func TimePtr(t time.Time) *time.Time {
	return &t
}

// GenerateRandomData generates random bytes of the specified size.
// This is useful for creating test data for uploads.
func GenerateRandomData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}
	return data
}

// GenerateRandomReader creates an io.Reader with random data of the specified size.
// This is useful for testing stream-based uploads.
func GenerateRandomReader(size int) io.Reader {
	return bytes.NewReader(GenerateRandomData(size))
}

// GenerateTestKey generates a test S3 object key with optional prefix.
// This helps ensure test isolation by using unique keys.
func GenerateTestKey(prefix string) string {
	timestamp := time.Now().UnixNano()
	random := rand.Int63n(100000)
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%stest-object-%d-%d", prefix, timestamp, random)
}

// GenerateTestBucketName generates a valid test bucket name.
// Bucket names must be DNS-compliant and globally unique.
func GenerateTestBucketName(prefix string) string {
	timestamp := time.Now().Unix()
	random := rand.Int31n(10000)
	name := fmt.Sprintf("%s-%d-%d", prefix, timestamp, random)
	// Ensure DNS compliance
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

// CalculateMD5 calculates the MD5 hash of data and returns it as a base64-encoded string.
// This is useful for verifying object integrity.
func CalculateMD5(data []byte) string {
	h := md5.Sum(data)
	return base64.StdEncoding.EncodeToString(h[:])
}

// CalculateETag calculates the ETag for the given data.
// For simple uploads, this is the MD5 hash. For multipart uploads, the format is different.
func CalculateETag(data []byte) string {
	h := md5.Sum(data)
	return fmt.Sprintf(`"%x"`, h)
}

// CreateTestObject creates a test S3 object structure.
// This is useful for mocking ListObjectsV2 responses.
func CreateTestObject(key string, size int64, lastModified time.Time) types.Object {
	return types.Object{
		Key:          StringPtr(key),
		Size:         Int64Ptr(size),
		LastModified: TimePtr(lastModified),
		ETag:         StringPtr(fmt.Sprintf(`"%x"`, md5.Sum([]byte(key)))),
		StorageClass: types.ObjectStorageClassStandard,
	}
}

// CreateTestObjectVersion creates a test S3 object version structure.
// This is useful for testing versioned bucket operations.
func CreateTestObjectVersion(key, versionID string, size int64, lastModified time.Time) types.ObjectVersion {
	return types.ObjectVersion{
		Key:          StringPtr(key),
		VersionId:    StringPtr(versionID),
		Size:         Int64Ptr(size),
		LastModified: TimePtr(lastModified),
		ETag:         StringPtr(fmt.Sprintf(`"%x"`, md5.Sum([]byte(key)))),
		StorageClass: types.ObjectVersionStorageClassStandard,
		IsLatest:     BoolPtr(true),
	}
}

// CreateListObjectsV2Output creates a test ListObjectsV2Output structure.
// This is useful for mocking S3 list operations.
func CreateListObjectsV2Output(
	objects []types.Object, prefix, delimiter string, truncated bool,
) *s3.ListObjectsV2Output {
	output := &s3.ListObjectsV2Output{
		Contents:    objects,
		KeyCount:    Int32Ptr(int32(len(objects))),
		MaxKeys:     Int32Ptr(1000),
		Name:        StringPtr("test-bucket"),
		Prefix:      StringPtr(prefix),
		Delimiter:   StringPtr(delimiter),
		IsTruncated: BoolPtr(truncated),
	}
	if truncated && len(objects) > 0 {
		output.NextContinuationToken = StringPtr("next-token")
	}
	return output
}

// CreateHeadObjectOutput creates a test HeadObjectOutput structure.
// This is useful for mocking HeadObject operations.
func CreateHeadObjectOutput(size int64, lastModified time.Time, contentType string) *s3.HeadObjectOutput {
	return &s3.HeadObjectOutput{
		ContentLength: Int64Ptr(size),
		LastModified:  TimePtr(lastModified),
		ContentType:   StringPtr(contentType),
		ETag:          StringPtr(fmt.Sprintf(`"%x"`, md5.Sum([]byte("test")))),
		Metadata:      map[string]string{},
	}
}

// CreatePutObjectOutput creates a test PutObjectOutput structure.
// This is useful for mocking upload operations.
func CreatePutObjectOutput(etag string) *s3.PutObjectOutput {
	return &s3.PutObjectOutput{
		ETag: StringPtr(etag),
	}
}

// CreateGetObjectOutput creates a test GetObjectOutput structure.
// This is useful for mocking download operations.
func CreateGetObjectOutput(data []byte, contentType string) *s3.GetObjectOutput {
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: Int64Ptr(int64(len(data))),
		ContentType:   StringPtr(contentType),
		ETag:          StringPtr(CalculateETag(data)),
		LastModified:  TimePtr(time.Now()),
	}
}

// AssertNoError checks that an error is nil and fails the test if it's not.
// This reduces boilerplate in tests.
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", msg, err)
	}
}

// AssertError checks that an error is not nil and fails the test if it is.
// This reduces boilerplate in tests.
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", msg)
	}
}

// AssertEqual checks that two values are equal and fails the test if they're not.
// This reduces boilerplate in tests.
func AssertEqual(t *testing.T, expected, actual interface{}, msg string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("%s: expected %v but got %v", msg, expected, actual)
	}
}

// CleanupTestBucket creates a function to clean up a test bucket.
// This should be used with t.Cleanup() to ensure buckets are deleted after tests.
func CleanupTestBucket(client *s3.Client, bucket string) func() {
	return func() {
		// First, delete all objects in the bucket
		listInput := &s3.ListObjectsV2Input{
			Bucket: StringPtr(bucket),
		}
		for {
			listOutput, err := client.ListObjectsV2(context.Background(), listInput)
			if err != nil {
				break
			}
			if len(listOutput.Contents) == 0 {
				break
			}
			var objects []types.ObjectIdentifier
			for _, obj := range listOutput.Contents {
				objects = append(objects, types.ObjectIdentifier{
					Key: obj.Key,
				})
			}
			deleteInput := &s3.DeleteObjectsInput{
				Bucket: StringPtr(bucket),
				Delete: &types.Delete{
					Objects: objects,
				},
			}
			_, _ = client.DeleteObjects(context.Background(), deleteInput)
			if !aws.ToBool(listOutput.IsTruncated) {
				break
			}
			listInput.ContinuationToken = listOutput.NextContinuationToken
		}
		// Then delete the bucket
		deleteInput := &s3.DeleteBucketInput{
			Bucket: StringPtr(bucket),
		}
		_, _ = client.DeleteBucket(context.Background(), deleteInput)
	}
}
