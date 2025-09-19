// Package testutil provides test data generators.
package testutil

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestDataGenerator provides methods for generating test data.
type TestDataGenerator struct {
	rand *rand.Rand
}

// NewTestDataGenerator creates a new test data generator with a seeded random source.
func NewTestDataGenerator(seed int64) *TestDataGenerator {
	return &TestDataGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateObjectList generates a list of test S3 objects.
func (g *TestDataGenerator) GenerateObjectList(count int, prefix string) []types.Object {
	objects := make([]types.Object, count)
	baseTime := time.Now().Add(-24 * time.Hour)

	for i := 0; i < count; i++ {
		key := fmt.Sprintf("%sobject-%04d.txt", prefix, i)
		size := int64(g.rand.Intn(1000000) + 1000) // 1KB to 1MB
		modified := baseTime.Add(time.Duration(i) * time.Minute)
		objects[i] = CreateTestObject(key, size, modified)
	}

	return objects
}

// GenerateCommonPrefixes generates common prefixes for directory-like structures.
func (g *TestDataGenerator) GenerateCommonPrefixes(count int, base string) []types.CommonPrefix {
	prefixes := make([]types.CommonPrefix, count)

	for i := 0; i < count; i++ {
		prefixes[i] = types.CommonPrefix{
			Prefix: StringPtr(fmt.Sprintf("%sdir%02d/", base, i)),
		}
	}

	return prefixes
}

// GenerateMultipartUpload generates a test multipart upload structure.
func (g *TestDataGenerator) GenerateMultipartUpload(key, uploadID string) types.MultipartUpload {
	return types.MultipartUpload{
		Key:          StringPtr(key),
		UploadId:     StringPtr(uploadID),
		StorageClass: types.StorageClassStandard,
		Initiated:    TimePtr(time.Now()),
	}
}

// GenerateCompletedParts generates completed multipart upload parts.
func (g *TestDataGenerator) GenerateCompletedParts(count int) []types.CompletedPart {
	parts := make([]types.CompletedPart, count)

	for i := 0; i < count; i++ {
		parts[i] = types.CompletedPart{
			PartNumber: Int32Ptr(int32(i + 1)),
			ETag:       StringPtr(fmt.Sprintf(`"%x"`, g.rand.Int63())),
		}
	}

	return parts
}

// GenerateDeleteMarkers generates delete markers for versioned buckets.
func (g *TestDataGenerator) GenerateDeleteMarkers(count int) []types.DeleteMarkerEntry {
	markers := make([]types.DeleteMarkerEntry, count)
	baseTime := time.Now().Add(-24 * time.Hour)

	for i := 0; i < count; i++ {
		markers[i] = types.DeleteMarkerEntry{
			Key:          StringPtr(fmt.Sprintf("deleted-object-%04d", i)),
			VersionId:    StringPtr(fmt.Sprintf("version-%d", g.rand.Int63())),
			IsLatest:     BoolPtr(i == count-1),
			LastModified: TimePtr(baseTime.Add(time.Duration(i) * time.Hour)),
		}
	}

	return markers
}

// GenerateS3Error generates a test S3 error response.
func (g *TestDataGenerator) GenerateS3Error(code, message string) *types.NoSuchKey {
	return &types.NoSuchKey{
		Message: StringPtr(message),
	}
}

// GenerateCopyObjectResult generates a test copy object result.
func (g *TestDataGenerator) GenerateCopyObjectResult() *types.CopyObjectResult {
	return &types.CopyObjectResult{
		ETag:         StringPtr(fmt.Sprintf(`"%x"`, g.rand.Int63())),
		LastModified: TimePtr(time.Now()),
	}
}

// GenerateObjectMetadata generates test object metadata.
func (g *TestDataGenerator) GenerateObjectMetadata(size int64) map[string]string {
	return map[string]string{
		"test-key-1": fmt.Sprintf("test-value-%d", g.rand.Intn(100)),
		"test-key-2": fmt.Sprintf("test-value-%d", g.rand.Intn(100)),
		"size":       fmt.Sprintf("%d", size),
	}
}

// GenerateBucketMetadata generates test bucket metadata.
func (g *TestDataGenerator) GenerateBucketMetadata(name, region string) *s3.HeadBucketOutput {
	return &s3.HeadBucketOutput{
		BucketRegion: StringPtr(region),
	}
}

// GenerateTags generates test object tags.
func (g *TestDataGenerator) GenerateTags(count int) []types.Tag {
	tags := make([]types.Tag, count)

	for i := 0; i < count; i++ {
		tags[i] = types.Tag{
			Key:   StringPtr(fmt.Sprintf("tag-key-%d", i)),
			Value: StringPtr(fmt.Sprintf("tag-value-%d", g.rand.Intn(100))),
		}
	}

	return tags
}

// GenerateLifecycleRules generates test lifecycle rules.
func (g *TestDataGenerator) GenerateLifecycleRules(count int) []types.LifecycleRule {
	rules := make([]types.LifecycleRule, count)

	for i := 0; i < count; i++ {
		rules[i] = types.LifecycleRule{
			ID:     StringPtr(fmt.Sprintf("rule-%d", i)),
			Status: types.ExpirationStatusEnabled,
			Filter: &types.LifecycleRuleFilter{
				Prefix: StringPtr(fmt.Sprintf("prefix-%d/", i)),
			},
		}
	}

	return rules
}

// GenerateBucketVersioning generates test bucket versioning configuration.
func (g *TestDataGenerator) GenerateBucketVersioning(
	status types.BucketVersioningStatus,
) *types.VersioningConfiguration {
	return &types.VersioningConfiguration{
		Status: status,
	}
}

// GenerateObjectLockConfiguration generates test object lock configuration.
func (g *TestDataGenerator) GenerateObjectLockConfiguration() *types.ObjectLockConfiguration {
	return &types.ObjectLockConfiguration{
		ObjectLockEnabled: types.ObjectLockEnabledEnabled,
		Rule: &types.ObjectLockRule{
			DefaultRetention: &types.DefaultRetention{
				Mode: types.ObjectLockRetentionModeGovernance,
				Days: Int32Ptr(30),
			},
		},
	}
}
