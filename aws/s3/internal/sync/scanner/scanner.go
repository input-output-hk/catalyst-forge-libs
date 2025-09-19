// Package scanner handles filesystem and S3 scanning operations.
// This includes walking local directories and listing S3 objects.
//
// The scanner provides a unified interface for discovering files in both
// local and remote locations.
package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/input-output-hk/catalyst-forge-libs/fs"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/s3api"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Scanner handles scanning operations for both local filesystem and remote S3.
type Scanner struct {
	s3Client       s3api.S3API
	filesystem     fs.Filesystem
	patternMatcher *PatternMatcher
}

// NewScanner creates a new scanner with the provided S3 client and filesystem.
func NewScanner(s3Client s3api.S3API, filesystem fs.Filesystem) *Scanner {
	return &Scanner{
		s3Client:       s3Client,
		filesystem:     filesystem,
		patternMatcher: NewPatternMatcher(),
	}
}

// ScanLocal scans the local filesystem starting from the given path.
// It respects include and exclude patterns and returns a list of LocalFile objects.
func (s *Scanner) ScanLocal(
	ctx context.Context,
	localPath string,
	includePatterns []string,
	excludePatterns []string,
) ([]*s3types.LocalFile, error) {
	var files []*s3types.LocalFile

	// Walk the local filesystem
	err := s.filesystem.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories (we only want files)
		if info.IsDir() {
			return nil
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get relative path from scan root
		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Apply include/exclude patterns
		if !s.patternMatcher.ShouldIncludeFile(relPath, includePatterns, excludePatterns) {
			return nil
		}

		// Create LocalFile object
		localFile := &s3types.LocalFile{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}

		files = append(files, localFile)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", localPath, err)
	}

	return files, nil
}

// ScanRemote scans the S3 bucket with the given prefix.
// It returns a list of RemoteFile objects representing S3 objects.
func (s *Scanner) ScanRemote(
	ctx context.Context,
	bucket string,
	prefix string,
) ([]*s3types.RemoteFile, error) {
	var objects []*s3types.RemoteFile

	// List objects with pagination
	var continuationToken *string

	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during S3 listing: %w", ctx.Err())
		default:
		}

		// Prepare list request
		input := &s3.ListObjectsV2Input{
			Bucket:            &bucket,
			Prefix:            &prefix,
			ContinuationToken: continuationToken,
			MaxKeys:           aws.Int32(1000), // AWS default and maximum
		}

		// Execute list request
		result, err := s.s3Client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucket, err)
		}

		// Process objects in this page
		for _, obj := range result.Contents {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during object processing: %w", ctx.Err())
			default:
			}

			// Skip objects that don't have the prefix (shouldn't happen but safety check)
			if !strings.HasPrefix(*obj.Key, prefix) {
				continue
			}

			// Create RemoteFile object
			remoteFile := &s3types.RemoteFile{
				Key:          *obj.Key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
			}

			// Handle ETag if present
			if obj.ETag != nil {
				remoteFile.ETag = strings.Trim(*obj.ETag, `"`)
			}

			objects = append(objects, remoteFile)
		}

		// Check if there are more pages
		if result.IsTruncated == nil || !*result.IsTruncated {
			break
		}

		continuationToken = result.NextContinuationToken
	}

	return objects, nil
}

// ScanRemoteWithPattern scans the S3 bucket and applies include/exclude patterns.
// This is useful when you want to filter S3 objects with the same patterns as local files.
func (s *Scanner) ScanRemoteWithPattern(
	ctx context.Context,
	bucket string,
	prefix string,
	includePatterns []string,
	excludePatterns []string,
) ([]*s3types.RemoteFile, error) {
	// First scan all objects with the prefix
	allObjects, err := s.ScanRemote(ctx, bucket, prefix)
	if err != nil {
		return nil, err
	}

	// Filter objects based on patterns
	var filteredObjects []*s3types.RemoteFile

	for _, obj := range allObjects {
		// Get relative path from prefix
		relPath := strings.TrimPrefix(obj.Key, prefix)
		relPath = strings.TrimPrefix(relPath, "/")

		// Apply include/exclude patterns
		if s.patternMatcher.ShouldIncludeFile(relPath, includePatterns, excludePatterns) {
			filteredObjects = append(filteredObjects, obj)
		}
	}

	return filteredObjects, nil
}

// GetLocalFileInfo gets detailed information about a local file.
// This is used when we need checksums or other metadata for comparison.
func (s *Scanner) GetLocalFileInfo(path string) (*s3types.LocalFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	localFile := &s3types.LocalFile{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}

	return localFile, nil
}

// GetRemoteFileInfo gets detailed information about a remote S3 object.
// This is used when we need additional metadata for comparison.
func (s *Scanner) GetRemoteFileInfo(ctx context.Context, bucket, key string) (*s3types.RemoteFile, error) {
	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	result, err := s.s3Client.HeadObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to head object %s/%s: %w", bucket, key, err)
	}

	remoteFile := &s3types.RemoteFile{
		Key:          key,
		Size:         *result.ContentLength,
		LastModified: *result.LastModified,
	}

	if result.ETag != nil {
		remoteFile.ETag = strings.Trim(*result.ETag, `"`)
	}

	return remoteFile, nil
}
