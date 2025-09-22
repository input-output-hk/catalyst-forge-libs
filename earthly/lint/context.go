// Package lint provides a flexible, rule-based linting framework for Earthfiles.
// It enables developers to enforce coding standards, security policies, and best practices
// through composable linting rules.
package lint

import (
	"github.com/input-output-hk/catalyst-forge-libs/earthly/earthfile"
)

// Context provides rules with contextual information about the current
// position in the Earthfile being linted. It supports hierarchical navigation
// through the Earthfile structure and provides caching for performance.
type Context struct {
	// File is the Earthfile being linted.
	File *earthfile.Earthfile

	// Target is the current target being examined (nil for global context).
	Target *earthfile.Target

	// Command is the current command being examined (nil for target-level context).
	Command *earthfile.Command

	// Parent provides access to the parent context in the hierarchy.
	// This enables rules to navigate up the context tree.
	Parent *Context

	// cache stores rule-specific data to avoid recomputation.
	// Keys should be prefixed with the rule name to avoid conflicts.
	cache map[string]interface{}

	// visited tracks which nodes have been visited to prevent infinite recursion.
	visited map[string]bool
}

// NewContext creates a new root Context for an Earthfile.
// This context represents the file-level scope.
func NewContext(ef *earthfile.Earthfile) *Context {
	return &Context{
		File:    ef,
		cache:   make(map[string]interface{}),
		visited: make(map[string]bool),
	}
}

// NewTargetContext creates a new Context for a specific target.
// This inherits cache and visited state from the parent context.
func NewTargetContext(parent *Context, target *earthfile.Target) *Context {
	return &Context{
		File:    parent.File,
		Target:  target,
		Parent:  parent,
		cache:   parent.cache,   // Shared cache for performance
		visited: parent.visited, // Shared visited map
	}
}

// NewCommandContext creates a new Context for a specific command.
// This inherits cache and visited state from the parent context.
func NewCommandContext(parent *Context, command *earthfile.Command) *Context {
	return &Context{
		File:    parent.File,
		Target:  parent.Target,
		Command: command,
		Parent:  parent,
		cache:   parent.cache,   // Shared cache for performance
		visited: parent.visited, // Shared visited map
	}
}

// IsFileLevel returns true if this context is at the file level (no target or command).
func (ctx *Context) IsFileLevel() bool {
	return ctx.Target == nil && ctx.Command == nil
}

// IsTargetLevel returns true if this context is at the target level (has target but no command).
func (ctx *Context) IsTargetLevel() bool {
	return ctx.Target != nil && ctx.Command == nil
}

// IsCommandLevel returns true if this context is at the command level (has command).
func (ctx *Context) IsCommandLevel() bool {
	return ctx.Command != nil
}

// GetCache retrieves a cached value by key.
// Returns nil if the key doesn't exist.
func (ctx *Context) GetCache(key string) interface{} {
	return ctx.cache[key]
}

// SetCache stores a value in the cache with the given key.
func (ctx *Context) SetCache(key string, value interface{}) {
	ctx.cache[key] = value
}

// HasVisited returns true if the given key has been marked as visited.
func (ctx *Context) HasVisited(key string) bool {
	return ctx.visited[key]
}

// MarkVisited marks the given key as visited.
func (ctx *Context) MarkVisited(key string) {
	ctx.visited[key] = true
}

// ClearVisited removes the visited mark for the given key.
func (ctx *Context) ClearVisited(key string) {
	delete(ctx.visited, key)
}

// GetRootContext returns the root context (file-level) by traversing up the parent chain.
func (ctx *Context) GetRootContext() *Context {
	current := ctx
	for current.Parent != nil {
		current = current.Parent
	}
	return current
}

// GetTargetContext returns the nearest target context by traversing up the parent chain.
// Returns nil if no target context is found.
func (ctx *Context) GetTargetContext() *Context {
	current := ctx
	for current != nil {
		if current.IsTargetLevel() {
			return current
		}
		current = current.Parent
	}
	return nil
}

// WalkTargets executes the provided function for each target in the Earthfile.
// The function receives a new target context for each target.
// Walking stops if the function returns an error.
func (ctx *Context) WalkTargets(fn func(targetCtx *Context) error) error {
	if ctx.File == nil {
		return nil
	}

	// Walk through targets
	for _, target := range ctx.File.Targets() {
		targetCtx := NewTargetContext(ctx, target)
		if err := fn(targetCtx); err != nil {
			return err
		}
	}
	return nil
}

// WalkCommands executes the provided function for each command in the current target.
// The function receives a new command context for each command.
// Walking stops if the function returns an error.
func (ctx *Context) WalkCommands(fn func(commandCtx *Context) error) error {
	if ctx.Target == nil {
		return nil
	}

	// Walk through commands in the target
	for _, command := range ctx.Target.Commands {
		commandCtx := NewCommandContext(ctx, command)
		if err := fn(commandCtx); err != nil {
			return err
		}
	}
	return nil
}

// WalkAll executes the provided function for all contexts in the Earthfile.
// The function is called for file-level, target-level, and command-level contexts.
// Walking stops if the function returns an error.
func (ctx *Context) WalkAll(fn func(walkCtx *Context) error) error {
	// File-level context
	if err := fn(ctx); err != nil {
		return err
	}

	// Target-level contexts
	return ctx.WalkTargets(func(targetCtx *Context) error {
		// Target-level context
		if err := fn(targetCtx); err != nil {
			return err
		}

		// Command-level contexts
		return targetCtx.WalkCommands(func(commandCtx *Context) error {
			return fn(commandCtx)
		})
	})
}
