package lint

import (
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContext(t *testing.T) {
	ef := earthfile.NewEarthfile()
	ctx := NewContext(ef)

	assert.NotNil(t, ctx)
	assert.Equal(t, ef, ctx.File)
	assert.Nil(t, ctx.Target)
	assert.Nil(t, ctx.Command)
	assert.Nil(t, ctx.Parent)
	assert.NotNil(t, ctx.cache)
	assert.NotNil(t, ctx.visited)
	assert.True(t, ctx.IsFileLevel())
	assert.False(t, ctx.IsTargetLevel())
	assert.False(t, ctx.IsCommandLevel())
}

func TestNewTargetContext(t *testing.T) {
	ef := earthfile.NewEarthfile()
	target := &earthfile.Target{Name: "test-target"}
	rootCtx := NewContext(ef)
	targetCtx := NewTargetContext(rootCtx, target)

	assert.NotNil(t, targetCtx)
	assert.Equal(t, ef, targetCtx.File)
	assert.Equal(t, target, targetCtx.Target)
	assert.Nil(t, targetCtx.Command)
	assert.Equal(t, rootCtx, targetCtx.Parent)
	assert.Equal(t, rootCtx.cache, targetCtx.cache)     // Shared cache
	assert.Equal(t, rootCtx.visited, targetCtx.visited) // Shared visited map
	assert.False(t, targetCtx.IsFileLevel())
	assert.True(t, targetCtx.IsTargetLevel())
	assert.False(t, targetCtx.IsCommandLevel())
}

func TestNewCommandContext(t *testing.T) {
	ef := earthfile.NewEarthfile()
	target := &earthfile.Target{Name: "test-target"}
	command := &earthfile.Command{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"echo", "hello"}}
	rootCtx := NewContext(ef)
	targetCtx := NewTargetContext(rootCtx, target)
	commandCtx := NewCommandContext(targetCtx, command)

	assert.NotNil(t, commandCtx)
	assert.Equal(t, ef, commandCtx.File)
	assert.Equal(t, target, commandCtx.Target)
	assert.Equal(t, command, commandCtx.Command)
	assert.Equal(t, targetCtx, commandCtx.Parent)
	assert.Equal(t, rootCtx.cache, commandCtx.cache)     // Shared cache
	assert.Equal(t, rootCtx.visited, commandCtx.visited) // Shared visited map
	assert.False(t, commandCtx.IsFileLevel())
	assert.False(t, commandCtx.IsTargetLevel())
	assert.True(t, commandCtx.IsCommandLevel())
}

func TestContextHierarchy(t *testing.T) {
	ef := earthfile.NewEarthfile()
	target := &earthfile.Target{Name: "test-target"}
	command := &earthfile.Command{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"echo", "hello"}}

	rootCtx := NewContext(ef)
	targetCtx := NewTargetContext(rootCtx, target)
	commandCtx := NewCommandContext(targetCtx, command)

	// Test GetRootContext
	assert.Equal(t, rootCtx, commandCtx.GetRootContext())
	assert.Equal(t, rootCtx, targetCtx.GetRootContext())
	assert.Equal(t, rootCtx, rootCtx.GetRootContext())

	// Test GetTargetContext
	assert.Equal(t, targetCtx, commandCtx.GetTargetContext())
	assert.Equal(t, targetCtx, targetCtx.GetTargetContext())
	assert.Nil(t, rootCtx.GetTargetContext())
}

func TestContextCache(t *testing.T) {
	ctx := NewContext(earthfile.NewEarthfile())

	// Test GetCache on empty cache
	assert.Nil(t, ctx.GetCache("nonexistent"))

	// Test SetCache and GetCache
	ctx.SetCache("key1", "value1")
	ctx.SetCache("key2", 42)

	assert.Equal(t, "value1", ctx.GetCache("key1"))
	assert.Equal(t, 42, ctx.GetCache("key2"))

	// Test cache sharing between contexts
	targetCtx := NewTargetContext(ctx, &earthfile.Target{Name: "test"})
	commandCtx := NewCommandContext(targetCtx, &earthfile.Command{Name: "RUN"})

	// Values set in root should be visible in child contexts
	assert.Equal(t, "value1", targetCtx.GetCache("key1"))
	assert.Equal(t, 42, commandCtx.GetCache("key2"))

	// Values set in child contexts should be visible everywhere
	targetCtx.SetCache("shared", "shared-value")
	assert.Equal(t, "shared-value", ctx.GetCache("shared"))
	assert.Equal(t, "shared-value", commandCtx.GetCache("shared"))
}

func TestContextVisited(t *testing.T) {
	ctx := NewContext(earthfile.NewEarthfile())

	// Test initial state
	assert.False(t, ctx.HasVisited("node1"))
	assert.False(t, ctx.HasVisited("node2"))

	// Test MarkVisited
	ctx.MarkVisited("node1")
	assert.True(t, ctx.HasVisited("node1"))
	assert.False(t, ctx.HasVisited("node2"))

	// Test visited sharing between contexts
	targetCtx := NewTargetContext(ctx, &earthfile.Target{Name: "test"})
	commandCtx := NewCommandContext(targetCtx, &earthfile.Command{Name: "RUN"})

	// Visited marks should be shared
	assert.True(t, targetCtx.HasVisited("node1"))
	assert.True(t, commandCtx.HasVisited("node1"))

	commandCtx.MarkVisited("node2")
	assert.True(t, ctx.HasVisited("node2"))
	assert.True(t, targetCtx.HasVisited("node2"))

	// Test ClearVisited
	ctx.ClearVisited("node1")
	assert.False(t, ctx.HasVisited("node1"))
	assert.False(t, targetCtx.HasVisited("node1"))
	assert.False(t, commandCtx.HasVisited("node1"))
	assert.True(t, ctx.HasVisited("node2")) // Other marks unaffected
}

func TestWalkTargets(t *testing.T) {
	// Test with empty Earthfile (no targets)
	ef := earthfile.NewEarthfile()
	ctx := NewContext(ef)

	callCount := 0
	err := ctx.WalkTargets(func(targetCtx *Context) error {
		callCount++
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 0, callCount) // No targets, so function should not be called
}

func TestWalkCommands(t *testing.T) {
	ef := earthfile.NewEarthfile()
	target := &earthfile.Target{
		Name: "test-target",
		Commands: []*earthfile.Command{
			{Name: "RUN", Type: earthfile.CommandTypeRun, Args: []string{"echo", "hello"}},
			{Name: "COPY", Type: earthfile.CommandTypeCopy, Args: []string{"src", "dst"}},
		},
	}

	rootCtx := NewContext(ef)
	targetCtx := NewTargetContext(rootCtx, target)

	var walkedCommands []*earthfile.Command
	err := targetCtx.WalkCommands(func(commandCtx *Context) error {
		walkedCommands = append(walkedCommands, commandCtx.Command)
		assert.True(t, commandCtx.IsCommandLevel())
		assert.Equal(t, ef, commandCtx.File)
		assert.Equal(t, target, commandCtx.Target)
		assert.Equal(t, targetCtx, commandCtx.Parent)
		return nil
	})

	require.NoError(t, err)
	assert.Len(t, walkedCommands, 2)
	assert.Equal(t, target.Commands[0], walkedCommands[0])
	assert.Equal(t, target.Commands[1], walkedCommands[1])
}

func TestWalkCommandsNilTarget(t *testing.T) {
	ctx := NewContext(earthfile.NewEarthfile())

	err := ctx.WalkCommands(func(commandCtx *Context) error {
		t.Fatal("Should not be called")
		return nil
	})

	assert.NoError(t, err) // No error when target is nil
}

func TestWalkAll(t *testing.T) {
	// Test with empty Earthfile (no targets)
	ef := earthfile.NewEarthfile()
	ctx := NewContext(ef)

	var contexts []*Context
	err := ctx.WalkAll(func(walkCtx *Context) error {
		contexts = append(contexts, walkCtx)
		return nil
	})

	require.NoError(t, err)
	assert.Len(t, contexts, 1) // Only file-level context

	// Check file-level context
	assert.True(t, contexts[0].IsFileLevel())
}

func TestWalkAllWithError(t *testing.T) {
	// Test with empty Earthfile
	ef := earthfile.NewEarthfile()
	ctx := NewContext(ef)

	expectedError := assert.AnError

	err := ctx.WalkAll(func(walkCtx *Context) error {
		// Error immediately on file-level context
		return expectedError
	})

	assert.Equal(t, expectedError, err)
}
