package test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile"
)

func TestIntegration_BasicFixture(t *testing.T) {
	path := filepath.Join("fixtures", "basic", "Earthfile")
	ef, err := earthfile.Parse(path)
	require.NoError(t, err)
	require.NotNil(t, ef)

	// Version
	require.True(t, ef.HasVersion())
	require.Equal(t, "0.8", ef.Version())

	// Targets and functions
	require.True(t, ef.HasTarget("common"))
	require.True(t, ef.HasTarget("build"))
	require.True(t, ef.HasTarget("image"))

	// BaseCommands should include none beyond VERSION for this fixture
	_ = ef.BaseCommands()

	// Query commands on target
	build := ef.Target("build")
	require.NotNil(t, build)
	require.True(t, build.HasCommand(earthfile.CommandTypeArg))
	require.True(t, build.HasCommand(earthfile.CommandTypeRun))
	require.True(t, build.HasCommand(earthfile.CommandTypeSaveArtifact))

	// FindCommands caching path
	_ = build.GetArgs()
	_ = build.GetBuilds()
	_ = build.GetArtifacts()

	// Dependencies
	deps := ef.Dependencies()
	require.NotEmpty(t, deps)
}

func TestIntegration_ComplexFixture(t *testing.T) {
	path := filepath.Join("fixtures", "complex", "Earthfile")
	ef, err := earthfile.Parse(path)
	require.NoError(t, err)
	require.NotNil(t, ef)

	// Walk with Visitor and WalkCommands
	count := 0
	err = ef.WalkCommands(func(c *earthfile.Command, depth int) error {
		count++
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, count, 0)

	// Low-level AST access
	require.NotNil(t, ef.AST())
}

func TestIntegration_EmptyFixture(t *testing.T) {
	path := filepath.Join("fixtures", "empty", "Earthfile")
	ef, err := earthfile.Parse(path)
	require.NoError(t, err)
	require.NotNil(t, ef)
	require.False(t, ef.HasVersion())
	// No targets expected
	require.Len(t, ef.Targets(), 0)
}

func TestIntegration_BaseRecipeFixture(t *testing.T) {
	path := filepath.Join("fixtures", "base", "Earthfile")
	ef, err := earthfile.Parse(path)
	require.NoError(t, err)
	require.NotNil(t, ef)
	baseCmds := ef.BaseCommands()
	require.NotEmpty(t, baseCmds)
}
