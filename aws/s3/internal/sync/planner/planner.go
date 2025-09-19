// Package planner creates operation plans for sync operations.
// This includes determining which files need to be uploaded, downloaded, or deleted.
//
// The planner analyzes scan results and creates an optimized execution plan.
package planner

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/internal/sync/comparator"
	"github.com/input-output-hk/catalyst-forge-libs/aws/s3/s3types"
)

// Planner creates operation plans for sync operations.
type Planner struct {
	comparator comparator.Comparator
}

// NewPlanner creates a new planner with the given comparator.
func NewPlanner(comp comparator.Comparator) *Planner {
	return &Planner{
		comparator: comp,
	}
}

// Operation represents a planned sync operation.
type Operation struct {
	// Type of operation (upload, delete, skip)
	Type OperationType

	// LocalPath is the local file path (for uploads)
	LocalPath string

	// RemoteKey is the S3 object key (for uploads and deletes)
	RemoteKey string

	// Size is the file/object size in bytes
	Size int64

	// Reason describes why this operation was planned
	Reason string

	// Priority for ordering operations (lower numbers = higher priority)
	Priority int
}

// OperationType defines the type of sync operation.
type OperationType string

const (
	// OperationUpload indicates a file needs to be uploaded
	OperationUpload OperationType = "upload"

	// OperationDelete indicates a remote file needs to be deleted
	OperationDelete OperationType = "delete"

	// OperationSkip indicates a file is unchanged and should be skipped
	OperationSkip OperationType = "skip"
)

// Plan creates an execution plan from local files and remote objects.
func (p *Planner) Plan(
	localPath string,
	bucket string,
	prefix string,
	localFiles []*s3types.LocalFile,
	remoteObjects []*s3types.RemoteFile,
	deleteExtra bool,
) ([]*Operation, error) {
	// Create maps for efficient lookups
	localMap := p.buildLocalMap(localPath, localFiles)
	remoteMap := p.buildRemoteMap(prefix, remoteObjects)

	var operations []*Operation

	// Plan uploads and updates
	uploadOps, err := p.planUploads(localPath, prefix, localMap, remoteMap)
	if err != nil {
		return nil, fmt.Errorf("failed to plan uploads: %w", err)
	}
	operations = append(operations, uploadOps...)

	// Plan deletions if requested
	if deleteExtra {
		deleteOps := p.planDeletes(prefix, localMap, remoteMap)
		operations = append(operations, deleteOps...)
	}

	// Plan skips for unchanged files
	skipOps := p.planSkips(prefix, localMap, remoteMap)
	operations = append(operations, skipOps...)

	// Optimize and sort operations
	optimized := p.optimizePlan(operations)

	return optimized, nil
}

// buildLocalMap creates a map of relative paths to LocalFile objects.
func (p *Planner) buildLocalMap(localPath string, files []*s3types.LocalFile) map[string]*s3types.LocalFile {
	localMap := make(map[string]*s3types.LocalFile)

	for _, file := range files {
		relPath, err := filepath.Rel(localPath, file.Path)
		if err != nil {
			// Skip files we can't make relative
			continue
		}
		// Normalize path separators to forward slashes for S3 compatibility
		relPath = filepath.ToSlash(relPath)
		localMap[relPath] = file
	}

	return localMap
}

// buildRemoteMap creates a map of relative paths to RemoteFile objects.
func (p *Planner) buildRemoteMap(prefix string, objects []*s3types.RemoteFile) map[string]*s3types.RemoteFile {
	remoteMap := make(map[string]*s3types.RemoteFile)

	for _, obj := range objects {
		if strings.HasPrefix(obj.Key, prefix) {
			relPath := strings.TrimPrefix(obj.Key, prefix)
			// Remove leading slash if present
			relPath = strings.TrimPrefix(relPath, "/")
			remoteMap[relPath] = obj
		}
	}

	return remoteMap
}

// planUploads determines which files need to be uploaded or updated.
func (p *Planner) planUploads(
	localPath, prefix string,
	localMap map[string]*s3types.LocalFile,
	remoteMap map[string]*s3types.RemoteFile,
) ([]*Operation, error) {
	var operations []*Operation

	for relPath, localFile := range localMap {
		remoteFile, exists := remoteMap[relPath]

		if !exists {
			// File exists locally but not remotely - needs upload
			operations = append(operations, &Operation{
				Type:      OperationUpload,
				LocalPath: localFile.Path,
				RemoteKey: prefix + relPath,
				Size:      localFile.Size,
				Reason:    "new file",
				Priority:  p.calculateUploadPriority(localFile.Size),
			})
		} else {
			// File exists in both locations - check if changed
			changed, err := p.comparator.HasChanged(localFile, remoteFile)
			if err != nil {
				return nil, fmt.Errorf("failed to compare files %s: %w", relPath, err)
			}

			if changed {
				operations = append(operations, &Operation{
					Type:      OperationUpload,
					LocalPath: localFile.Path,
					RemoteKey: prefix + relPath,
					Size:      localFile.Size,
					Reason:    "modified",
					Priority:  p.calculateUploadPriority(localFile.Size),
				})
			}
		}
	}

	return operations, nil
}

// planDeletes determines which remote files need to be deleted.
func (p *Planner) planDeletes(
	prefix string,
	localMap map[string]*s3types.LocalFile,
	remoteMap map[string]*s3types.RemoteFile,
) []*Operation {
	var operations []*Operation

	for relPath, remoteFile := range remoteMap {
		if _, exists := localMap[relPath]; !exists {
			operations = append(operations, &Operation{
				Type:      OperationDelete,
				RemoteKey: remoteFile.Key,
				Size:      remoteFile.Size,
				Reason:    "extra remote file",
				Priority:  10, // Lower priority than uploads
			})
		}
	}

	return operations
}

// planSkips creates skip operations for unchanged files.
func (p *Planner) planSkips(
	prefix string,
	localMap map[string]*s3types.LocalFile,
	remoteMap map[string]*s3types.RemoteFile,
) []*Operation {
	var operations []*Operation

	for relPath, localFile := range localMap {
		if _, exists := remoteMap[relPath]; exists {
			// File exists in both - assume it's unchanged (comparison already done in planUploads)
			operations = append(operations, &Operation{
				Type:      OperationSkip,
				LocalPath: localFile.Path,
				RemoteKey: prefix + relPath,
				Size:      localFile.Size,
				Reason:    "unchanged",
				Priority:  100, // Lowest priority
			})
		}
	}

	return operations
}

// calculateUploadPriority assigns priority based on file size.
// Smaller files get higher priority for faster feedback.
func (p *Planner) calculateUploadPriority(size int64) int {
	switch {
	case size < 1024*1024: // < 1MB
		return 1
	case size < 10*1024*1024: // < 10MB
		return 2
	case size < 100*1024*1024: // < 100MB
		return 3
	default: // >= 100MB
		return 4
	}
}

// optimizePlan sorts operations for optimal execution.
func (p *Planner) optimizePlan(operations []*Operation) []*Operation {
	// Sort by priority first, then by operation type
	sort.Slice(operations, func(i, j int) bool {
		// First sort by priority (ascending - lower priority number first)
		if operations[i].Priority != operations[j].Priority {
			return operations[i].Priority < operations[j].Priority
		}

		// Then sort by operation type (uploads first, then deletes, then skips)
		typeOrder := map[OperationType]int{
			OperationUpload: 1,
			OperationDelete: 2,
			OperationSkip:   3,
		}

		iOrder := typeOrder[operations[i].Type]
		jOrder := typeOrder[operations[j].Type]

		return iOrder < jOrder
	})

	return operations
}

// GetOperationStats returns statistics about the planned operations.
func (p *Planner) GetOperationStats(operations []*Operation) OperationStats {
	stats := OperationStats{}

	for _, op := range operations {
		switch op.Type {
		case OperationUpload:
			stats.Uploads++
			stats.BytesToUpload += op.Size
		case OperationDelete:
			stats.Deletes++
			stats.BytesToDelete += op.Size
		case OperationSkip:
			stats.Skips++
		}
	}

	return stats
}

// OperationStats contains statistics about planned operations.
type OperationStats struct {
	// Number of files to upload
	Uploads int

	// Number of files to delete
	Deletes int

	// Number of files to skip
	Skips int

	// Total bytes to upload
	BytesToUpload int64

	// Total bytes to delete
	BytesToDelete int64
}

// ValidatePlan checks if a plan is valid and executable.
func (p *Planner) ValidatePlan(operations []*Operation) error {
	if len(operations) == 0 {
		return fmt.Errorf("plan contains no operations")
	}

	// Check for conflicting operations on the same remote key
	keyOperations := make(map[string][]*Operation)

	for _, op := range operations {
		if op.RemoteKey != "" {
			keyOperations[op.RemoteKey] = append(keyOperations[op.RemoteKey], op)
		}
	}

	// Check for conflicts
	for key, ops := range keyOperations {
		if len(ops) > 1 {
			// Multiple operations on same key - check for conflicts
			hasUpload := false
			hasDelete := false

			for _, op := range ops {
				switch op.Type {
				case OperationUpload:
					hasUpload = true
				case OperationDelete:
					hasDelete = true
				}
			}

			if hasUpload && hasDelete {
				return fmt.Errorf("conflicting operations on key %s: both upload and delete planned", key)
			}
		}
	}

	return nil
}

// FilterOperations filters operations based on criteria.
func (p *Planner) FilterOperations(
	operations []*Operation,
	includeTypes []OperationType,
	minSize, maxSize int64,
) []*Operation {
	var filtered []*Operation

	for _, op := range operations {
		// Check operation type
		if len(includeTypes) > 0 {
			included := false
			for _, t := range includeTypes {
				if op.Type == t {
					included = true
					break
				}
			}
			if !included {
				continue
			}
		}

		// Check size range
		if minSize > 0 && op.Size < minSize {
			continue
		}
		if maxSize > 0 && op.Size > maxSize {
			continue
		}

		filtered = append(filtered, op)
	}

	return filtered
}
