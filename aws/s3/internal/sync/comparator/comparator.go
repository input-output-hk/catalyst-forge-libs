// Package comparator provides file comparison strategies.
// This includes different algorithms for determining if files have changed.
//
// The package supports multiple comparison strategies including size-only,
// checksum-based, and smart comparison with ETag handling.
package comparator

import (
	"crypto/md5"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
	"time"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Comparator defines the interface for comparing local and remote files.
type Comparator interface {
	// HasChanged determines if the local and remote files are different
	HasChanged(local *s3types.LocalFile, remote *s3types.RemoteFile) (bool, error)
}

// SmartComparator is the default comparator that uses intelligent comparison strategies.
// It compares size first, then ETag/MD5 if available, and falls back to modification time.
type SmartComparator struct {
	// MaxTimeDiff is the maximum time difference allowed for modification time comparison (in seconds)
	MaxTimeDiff time.Duration
}

// NewSmartComparator creates a new smart comparator with default settings.
func NewSmartComparator() *SmartComparator {
	return &SmartComparator{
		MaxTimeDiff: 2 * time.Second, // 2 second tolerance for time differences
	}
}

// HasChanged implements the Comparator interface for SmartComparator.
func (c *SmartComparator) HasChanged(local *s3types.LocalFile, remote *s3types.RemoteFile) (bool, error) {
	// Step 1: Compare sizes - different sizes mean the files are different
	if local.Size != remote.Size {
		return true, nil
	}

	// Step 2: If we have an ETag from S3, try to compare it with local checksum
	if remote.ETag != "" && !strings.Contains(remote.ETag, "-") {
		// ETag is not multipart (doesn't contain "-"), so it's likely an MD5 hash
		localMD5, err := c.computeMD5(local.Path)
		if err != nil {
			// If we can't compute local MD5, fall back to time comparison
			changed, timeErr := c.compareByTime(local, remote)
			if timeErr != nil {
				return false, fmt.Errorf("failed to compare by time after MD5 error: %w", timeErr)
			}
			return changed, nil
		}

		// Compare MD5 hashes
		if localMD5 != remote.ETag {
			return true, nil
		}

		// Files are the same based on size and MD5
		return false, nil
	}

	// Step 3: Fall back to modification time comparison
	changed, err := c.compareByTime(local, remote)
	if err != nil {
		return false, fmt.Errorf("failed to compare by time: %w", err)
	}
	return changed, nil
}

// computeMD5 computes the MD5 hash of a local file.
func (c *SmartComparator) computeMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for MD5 computation: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to compute MD5: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// compareByTime compares files based on their modification times.
func (c *SmartComparator) compareByTime(local *s3types.LocalFile, remote *s3types.RemoteFile) (bool, error) {
	// Calculate the time difference
	timeDiff := local.ModTime.Sub(remote.LastModified)

	// If the absolute difference is greater than our tolerance, consider them changed
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	return timeDiff > c.MaxTimeDiff, nil
}

// SizeOnlyComparator only compares file sizes.
// This is the fastest comparator but may miss changes that don't affect size.
type SizeOnlyComparator struct{}

// NewSizeOnlyComparator creates a new size-only comparator.
func NewSizeOnlyComparator() *SizeOnlyComparator {
	return &SizeOnlyComparator{}
}

// HasChanged implements the Comparator interface for SizeOnlyComparator.
func (c *SizeOnlyComparator) HasChanged(local *s3types.LocalFile, remote *s3types.RemoteFile) (bool, error) {
	return local.Size != remote.Size, nil
}

// ChecksumComparator always computes and compares checksums.
// This is the most accurate comparator but requires reading entire files.
type ChecksumComparator struct {
	// HashFunc is the hash function to use (defaults to MD5)
	HashFunc func() hash.Hash
}

// NewChecksumComparator creates a new checksum comparator with the default hash function (MD5).
func NewChecksumComparator() *ChecksumComparator {
	return &ChecksumComparator{
		HashFunc: md5.New,
	}
}

// NewChecksumComparatorWithHash creates a new checksum comparator with a custom hash function.
func NewChecksumComparatorWithHash(hashFunc func() hash.Hash) *ChecksumComparator {
	return &ChecksumComparator{
		HashFunc: hashFunc,
	}
}

// HasChanged implements the Comparator interface for ChecksumComparator.
func (c *ChecksumComparator) HasChanged(local *s3types.LocalFile, remote *s3types.RemoteFile) (bool, error) {
	// Always compute local checksum
	localChecksum, err := c.computeChecksum(local.Path)
	if err != nil {
		return false, fmt.Errorf("failed to compute local checksum: %w", err)
	}

	// If remote has an ETag and it's not multipart, use it for comparison
	if remote.ETag != "" && !strings.Contains(remote.ETag, "-") {
		return localChecksum != remote.ETag, nil
	}

	// If no reliable remote checksum, we can't determine if changed
	// In this case, we assume they are different to be safe
	return true, nil
}

// computeChecksum computes the checksum of a local file using the configured hash function.
func (c *ChecksumComparator) computeChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for checksum computation: %w", err)
	}
	defer file.Close()

	hash := c.HashFunc()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to compute checksum: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// TimeComparator compares files based on modification time with a configurable tolerance.
// This is useful for scenarios where content changes are reflected in timestamps.
type TimeComparator struct {
	// MaxTimeDiff is the maximum time difference allowed (in seconds)
	MaxTimeDiff time.Duration
}

// NewTimeComparator creates a new time comparator with default settings.
func NewTimeComparator() *TimeComparator {
	return &TimeComparator{
		MaxTimeDiff: 1 * time.Second,
	}
}

// HasChanged implements the Comparator interface for TimeComparator.
func (c *TimeComparator) HasChanged(local *s3types.LocalFile, remote *s3types.RemoteFile) (bool, error) {
	timeDiff := local.ModTime.Sub(remote.LastModified)

	// Get absolute difference
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	return timeDiff > c.MaxTimeDiff, nil
}

// CompositeComparator allows combining multiple comparators with logical operations.
type CompositeComparator struct {
	comparators []Comparator
	// RequireAll determines if all comparators must agree (AND) or any (OR)
	RequireAll bool
}

// NewCompositeComparator creates a new composite comparator.
func NewCompositeComparator(requireAll bool, comparators ...Comparator) *CompositeComparator {
	return &CompositeComparator{
		comparators: comparators,
		RequireAll:  requireAll,
	}
}

// HasChanged implements the Comparator interface for CompositeComparator.
func (c *CompositeComparator) HasChanged(local *s3types.LocalFile, remote *s3types.RemoteFile) (bool, error) {
	if len(c.comparators) == 0 {
		return false, fmt.Errorf("no comparators configured")
	}

	// Check all comparators
	for _, comp := range c.comparators {
		changed, err := comp.HasChanged(local, remote)
		if err != nil {
			return false, fmt.Errorf("composite comparator failed: %w", err)
		}

		// If any comparator says changed, return changed immediately
		// This works for both RequireAll and !RequireAll cases
		if changed {
			return true, nil
		}
	}

	// All comparators agree files are the same (no change)
	return false, nil
}

// NullComparator always returns that files have not changed.
// This is useful for force-sync scenarios or testing.
type NullComparator struct{}

// NewNullComparator creates a new null comparator.
func NewNullComparator() *NullComparator {
	return &NullComparator{}
}

// HasChanged implements the Comparator interface for NullComparator.
func (c *NullComparator) HasChanged(local *s3types.LocalFile, remote *s3types.RemoteFile) (bool, error) {
	return false, nil
}
