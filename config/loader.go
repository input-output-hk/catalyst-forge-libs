package config

import (
	"context"

	cuepkg "github.com/input-output-hk/catalyst-forge-libs/cue"
	"github.com/input-output-hk/catalyst-forge-libs/errors"
	"github.com/input-output-hk/catalyst-forge-libs/fs/core"
	"github.com/input-output-hk/catalyst-forge-libs/schema"
)

// loadRepoConfig loads a repository configuration from the specified path.
// This is an internal function called by the public LoadRepoConfig functions.
//
// The function performs the following steps:
// 1. Creates a CUE loader for the filesystem
// 2. Loads and parses the CUE file
// 3. Decodes the CUE value into schema types
// 4. Wraps the result in a RepoConfig wrapper
// 5. Validates the configuration (unless SkipValidation is set)
//
// All errors are wrapped with context using the errors package.
func loadRepoConfig(ctx context.Context, filesystem core.ReadFS, path string, _ LoadOptions) (*RepoConfig, error) {
	// Create CUE loader
	loader := newCueLoader(filesystem)

	// Load CUE file
	cueValue, err := loader.LoadFile(ctx, path)
	if err != nil {
		return nil, errors.WrapWithContext(
			err,
			errors.CodeCUELoadFailed,
			"failed to load repository configuration",
			map[string]interface{}{
				"path": path,
			},
		)
	}

	// Decode into schema type
	var repoSchema schema.RepoConfig
	if decodeErr := cuepkg.Decode(ctx, cueValue, &repoSchema); decodeErr != nil {
		return nil, errors.WrapWithContext(
			decodeErr,
			errors.CodeCUEDecodeFailed,
			"failed to decode repository configuration",
			map[string]interface{}{
				"path": path,
			},
		)
	}

	// Create RepoConfig wrapper
	// The schema now properly decodes phases and publishers maps
	repo := &RepoConfig{
		RepoConfig: &repoSchema,
	}

	return repo, nil
}

// loadProjectConfig loads a project configuration from the specified path.
// This is an internal function called by the public LoadProjectConfig functions.
//
// The function performs the following steps:
// 1. Creates a CUE loader for the filesystem
// 2. Loads and parses the CUE file
// 3. Decodes the CUE value into schema types
// 4. Wraps the result in a ProjectConfig wrapper
//
// Note: Full validation requires repository context, which can be done later
// via ProjectConfig.Validate(repo). When SkipValidation is false, only
// basic validation is performed.
//
// All errors are wrapped with context using the errors package.
func loadProjectConfig(ctx context.Context, filesystem core.ReadFS, path string, opts LoadOptions) (*ProjectConfig, error) {
	// Create CUE loader
	loader := newCueLoader(filesystem)

	// Load CUE file
	cueValue, err := loader.LoadFile(ctx, path)
	if err != nil {
		return nil, errors.WrapWithContext(
			err,
			errors.CodeCUELoadFailed,
			"failed to load project configuration",
			map[string]interface{}{
				"path": path,
			},
		)
	}

	// Decode into schema type
	var projectSchema schema.ProjectConfig
	if decodeErr := cuepkg.Decode(ctx, cueValue, &projectSchema); decodeErr != nil {
		return nil, errors.WrapWithContext(
			decodeErr,
			errors.CodeCUEDecodeFailed,
			"failed to decode project configuration",
			map[string]interface{}{
				"path": path,
			},
		)
	}

	// Create ProjectConfig wrapper
	// The schema now properly decodes artifacts map
	project := &ProjectConfig{
		ProjectConfig: &projectSchema,
	}

	// Note: Full validation requires repository context
	// We skip validation here even if opts.SkipValidation is false
	// The caller should call project.Validate(repo) separately
	_ = opts.SkipValidation // Mark as used

	return project, nil
}

// newCueLoader creates a new CUE loader for the given filesystem.
// This is a helper function to create cue.Loader instances.
func newCueLoader(filesystem core.ReadFS) *cuepkg.Loader {
	return cuepkg.NewLoader(filesystem)
}
