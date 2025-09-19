package comparator

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

func setupTestFile(t *testing.T, content string) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(filePath, []byte(content), 0o644)
	require.NoError(t, err)

	return filePath
}

func getFileInfo(t *testing.T, path string) (int64, time.Time) {
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Size(), info.ModTime()
}

func computeMD5String(content string) string {
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

func TestSmartComparator(t *testing.T) {
	comp := NewSmartComparator()

	t.Run("different sizes", func(t *testing.T) {
		localPath := setupTestFile(t, "hello")
		size, modTime := getFileInfo(t, localPath)

		local := &s3types.LocalFile{
			Path:    localPath,
			Size:    size,
			ModTime: modTime,
		}

		remote := &s3types.RemoteFile{
			Key:          "test.txt",
			Size:         size + 100, // Different size
			LastModified: modTime,
			ETag:         computeMD5String("hello"),
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.True(t, changed, "Files with different sizes should be marked as changed")
	})

	t.Run("same size same MD5", func(t *testing.T) {
		content := "hello world"
		localPath := setupTestFile(t, content)
		size, modTime := getFileInfo(t, localPath)

		local := &s3types.LocalFile{
			Path:    localPath,
			Size:    size,
			ModTime: modTime,
		}

		remote := &s3types.RemoteFile{
			Key:          "test.txt",
			Size:         size,
			LastModified: modTime,
			ETag:         computeMD5String(content),
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.False(t, changed, "Files with same size and MD5 should not be marked as changed")
	})

	t.Run("same size different MD5", func(t *testing.T) {
		localPath := setupTestFile(t, "hello world")
		size, modTime := getFileInfo(t, localPath)

		local := &s3types.LocalFile{
			Path:    localPath,
			Size:    size,
			ModTime: modTime,
		}

		remote := &s3types.RemoteFile{
			Key:          "test.txt",
			Size:         size,
			LastModified: modTime,
			ETag:         computeMD5String("world hello"), // Different content, same size
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.True(t, changed, "Files with same size but different MD5 should be marked as changed")
	})

	t.Run("multipart ETag falls back to time comparison", func(t *testing.T) {
		localPath := setupTestFile(t, "hello")
		size, modTime := getFileInfo(t, localPath)

		local := &s3types.LocalFile{
			Path:    localPath,
			Size:    size,
			ModTime: modTime,
		}

		remote := &s3types.RemoteFile{
			Key:          "test.txt",
			Size:         size,
			LastModified: modTime.Add(-10 * time.Second), // Older remote file
			ETag:         "abc123-2",                     // Multipart ETag (contains dash)
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.True(t, changed, "Files with time difference > tolerance should be marked as changed")
	})

	t.Run("time comparison within tolerance", func(t *testing.T) {
		localPath := setupTestFile(t, "hello")
		size, modTime := getFileInfo(t, localPath)

		local := &s3types.LocalFile{
			Path:    localPath,
			Size:    size,
			ModTime: modTime,
		}

		remote := &s3types.RemoteFile{
			Key:          "test.txt",
			Size:         size,
			LastModified: modTime.Add(-1 * time.Second), // Within tolerance
			ETag:         "abc123-2",                    // Multipart ETag
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.False(t, changed, "Files with time difference within tolerance should not be marked as changed")
	})

	t.Run("no ETag falls back to time", func(t *testing.T) {
		localPath := setupTestFile(t, "hello")
		size, modTime := getFileInfo(t, localPath)

		local := &s3types.LocalFile{
			Path:    localPath,
			Size:    size,
			ModTime: modTime,
		}

		remote := &s3types.RemoteFile{
			Key:          "test.txt",
			Size:         size,
			LastModified: modTime,
			ETag:         "", // No ETag
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.False(t, changed, "Files with same time should not be marked as changed")
	})
}

func TestSizeOnlyComparator(t *testing.T) {
	comp := NewSizeOnlyComparator()

	t.Run("different sizes", func(t *testing.T) {
		local := &s3types.LocalFile{
			Path: "/fake/path",
			Size: 100,
		}

		remote := &s3types.RemoteFile{
			Key:  "test.txt",
			Size: 200,
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.True(t, changed, "Files with different sizes should be marked as changed")
	})

	t.Run("same size", func(t *testing.T) {
		local := &s3types.LocalFile{
			Path: "/fake/path",
			Size: 100,
		}

		remote := &s3types.RemoteFile{
			Key:  "test.txt",
			Size: 100,
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.False(t, changed, "Files with same size should not be marked as changed")
	})
}

func TestChecksumComparator(t *testing.T) {
	t.Run("with MD5 hash", func(t *testing.T) {
		comp := NewChecksumComparator()
		content := "hello world"
		localPath := setupTestFile(t, content)

		local := &s3types.LocalFile{
			Path: localPath,
		}

		t.Run("matching checksums", func(t *testing.T) {
			remote := &s3types.RemoteFile{
				Key:  "test.txt",
				ETag: computeMD5String(content),
			}

			changed, err := comp.HasChanged(local, remote)
			assert.NoError(t, err)
			assert.False(t, changed, "Files with matching checksums should not be marked as changed")
		})

		t.Run("different checksums", func(t *testing.T) {
			remote := &s3types.RemoteFile{
				Key:  "test.txt",
				ETag: computeMD5String("different content"),
			}

			changed, err := comp.HasChanged(local, remote)
			assert.NoError(t, err)
			assert.True(t, changed, "Files with different checksums should be marked as changed")
		})

		t.Run("multipart ETag", func(t *testing.T) {
			remote := &s3types.RemoteFile{
				Key:  "test.txt",
				ETag: "abc123-2", // Multipart
			}

			changed, err := comp.HasChanged(local, remote)
			assert.NoError(t, err)
			assert.True(t, changed, "Files with multipart ETag should be marked as changed (can't compare)")
		})

		t.Run("no ETag", func(t *testing.T) {
			remote := &s3types.RemoteFile{
				Key:  "test.txt",
				ETag: "",
			}

			changed, err := comp.HasChanged(local, remote)
			assert.NoError(t, err)
			assert.True(t, changed, "Files with no ETag should be marked as changed (can't compare)")
		})
	})

	t.Run("with custom hash function", func(t *testing.T) {
		comp := NewChecksumComparatorWithHash(sha256.New)
		content := "hello world"
		localPath := setupTestFile(t, content)

		local := &s3types.LocalFile{
			Path: localPath,
		}

		// Note: In practice, S3 ETags are MD5, so this test shows that
		// custom hash functions would always see differences
		remote := &s3types.RemoteFile{
			Key:  "test.txt",
			ETag: computeMD5String(content), // MD5 won't match SHA256
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.True(t, changed, "SHA256 hash won't match MD5 ETag")
	})

	t.Run("file not found", func(t *testing.T) {
		comp := NewChecksumComparator()

		local := &s3types.LocalFile{
			Path: "/non/existent/file",
		}

		remote := &s3types.RemoteFile{
			Key:  "test.txt",
			ETag: "abc123",
		}

		_, err := comp.HasChanged(local, remote)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to compute local checksum")
	})
}

func TestTimeComparator(t *testing.T) {
	comp := NewTimeComparator()

	baseTime := time.Now()

	t.Run("same time", func(t *testing.T) {
		local := &s3types.LocalFile{
			ModTime: baseTime,
		}

		remote := &s3types.RemoteFile{
			LastModified: baseTime,
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.False(t, changed)
	})

	t.Run("within tolerance", func(t *testing.T) {
		local := &s3types.LocalFile{
			ModTime: baseTime,
		}

		remote := &s3types.RemoteFile{
			LastModified: baseTime.Add(500 * time.Millisecond), // Within 1 second
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.False(t, changed)
	})

	t.Run("outside tolerance - local newer", func(t *testing.T) {
		local := &s3types.LocalFile{
			ModTime: baseTime,
		}

		remote := &s3types.RemoteFile{
			LastModified: baseTime.Add(-2 * time.Second),
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.True(t, changed)
	})

	t.Run("outside tolerance - remote newer", func(t *testing.T) {
		local := &s3types.LocalFile{
			ModTime: baseTime,
		}

		remote := &s3types.RemoteFile{
			LastModified: baseTime.Add(2 * time.Second),
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.True(t, changed)
	})
}

func TestCompositeComparator(t *testing.T) {
	t.Run("require all - all say no change", func(t *testing.T) {
		sizeComp := NewSizeOnlyComparator()
		timeComp := NewTimeComparator()

		comp := NewCompositeComparator(true, sizeComp, timeComp)

		now := time.Now()
		local := &s3types.LocalFile{
			Size:    100,
			ModTime: now,
		}

		remote := &s3types.RemoteFile{
			Size:         100, // Same size
			LastModified: now, // Same time
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.False(t, changed)
	})

	t.Run("require all - one says change", func(t *testing.T) {
		sizeComp := NewSizeOnlyComparator()
		timeComp := NewTimeComparator()

		comp := NewCompositeComparator(true, sizeComp, timeComp)

		now := time.Now()
		local := &s3types.LocalFile{
			Size:    100,
			ModTime: now,
		}

		remote := &s3types.RemoteFile{
			Size:         200, // Different size
			LastModified: now, // Same time
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.True(t, changed, "If any comparator says changed, composite should return changed")
	})

	t.Run("no comparators", func(t *testing.T) {
		comp := NewCompositeComparator(true)

		local := &s3types.LocalFile{}
		remote := &s3types.RemoteFile{}

		_, err := comp.HasChanged(local, remote)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no comparators configured")
	})

	t.Run("error propagation", func(t *testing.T) {
		// Use checksum comparator with non-existent file to trigger error
		checksumComp := NewChecksumComparator()
		comp := NewCompositeComparator(true, checksumComp)

		local := &s3types.LocalFile{
			Path: "/non/existent/file",
		}

		remote := &s3types.RemoteFile{
			ETag: "abc123",
		}

		_, err := comp.HasChanged(local, remote)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "composite comparator failed")
	})
}

func TestNullComparator(t *testing.T) {
	comp := NewNullComparator()

	t.Run("always returns false", func(t *testing.T) {
		local := &s3types.LocalFile{
			Size: 100,
		}

		remote := &s3types.RemoteFile{
			Size: 200, // Different size
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.False(t, changed, "Null comparator should always return false")
	})

	t.Run("with nil inputs", func(t *testing.T) {
		changed, err := comp.HasChanged(nil, nil)
		assert.NoError(t, err)
		assert.False(t, changed, "Null comparator should always return false even with nil")
	})
}

func TestSmartComparatorEdgeCases(t *testing.T) {
	t.Run("file read error falls back to time", func(t *testing.T) {
		comp := NewSmartComparator()

		// Create a file that we'll delete before comparison
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test.txt")

		err := os.WriteFile(filePath, []byte("test"), 0o644)
		require.NoError(t, err)

		size, modTime := getFileInfo(t, filePath)

		// Delete the file to cause read error
		err = os.Remove(filePath)
		require.NoError(t, err)

		local := &s3types.LocalFile{
			Path:    filePath,
			Size:    size,
			ModTime: modTime,
		}

		remote := &s3types.RemoteFile{
			Key:          "test.txt",
			Size:         size,
			LastModified: modTime,
			ETag:         "abc123", // Non-multipart ETag
		}

		// Should fall back to time comparison
		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.False(t, changed, "Should fall back to time comparison")
	})

	t.Run("custom time tolerance", func(t *testing.T) {
		comp := &SmartComparator{
			MaxTimeDiff: 5 * time.Second, // Custom tolerance
		}

		localPath := setupTestFile(t, "hello")
		size, modTime := getFileInfo(t, localPath)

		local := &s3types.LocalFile{
			Path:    localPath,
			Size:    size,
			ModTime: modTime,
		}

		// Within custom tolerance
		remote := &s3types.RemoteFile{
			Key:          "test.txt",
			Size:         size,
			LastModified: modTime.Add(-3 * time.Second),
			ETag:         "abc123-2", // Multipart
		}

		changed, err := comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.False(t, changed, "Should be within custom tolerance")

		// Outside custom tolerance
		remote.LastModified = modTime.Add(-6 * time.Second)
		changed, err = comp.HasChanged(local, remote)
		assert.NoError(t, err)
		assert.True(t, changed, "Should be outside custom tolerance")
	})
}
