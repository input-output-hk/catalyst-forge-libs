# Catalyst Forge Libraries

A collection of Go libraries for the Catalyst Forge platform.

## Go Workspace Setup

This repository is configured as a Go workspace, which allows all the modules to reference each other locally without needing to publish changes first. This is particularly useful for development and testing.

### Workspace Structure

The workspace includes the following modules:

- `cue/` - CUE language support
- `domain/` - Domain models and entities
- `errors/` - Error handling and classification
- `fs/billy/` - Billy filesystem adapters
- `fs/core/` - Core filesystem interfaces
- `fs/fstest/` - Filesystem testing utilities
- `schemas/` - Schema definitions and validation

### How It Works

The `go.work` file at the root of the repository tells Go to use the local versions of these modules instead of fetching them from remote sources. This means:

1. **Simplified local development** - the workspace coordinates all modules together
2. **Changes are immediately visible** across modules without needing to commit/push
3. **Testing is simplified** - you can test changes across multiple modules simultaneously

**Note:** For unpublished modules (those not yet available remotely), `replace` directives in `go.mod` files are still required alongside the workspace configuration. For example, `fs/fstest` depends on `fs/core` and uses a replace directive to ensure the local version is used.

### Common Commands

```bash
# Sync the workspace (updates go.work.sum if needed)
go work sync

# Run tests across all modules
go test ./...

# Run tests in a specific module
go test ./errors/...

# Build all modules
go build ./...

# See which modules are in the workspace
go work use
```

### Development Workflow

1. Make changes to any module(s)
2. Tests in other modules will automatically use the local version
3. Run `go work sync` if you've added new dependencies
4. Commit changes when ready

### Note on Version Control

- `go.work` **is** checked into version control so all developers use the workspace
- `go.work.sum` **is not** checked in (see `.gitignore`)

### Individual Module Development

If you need to work on a single module in isolation (without the workspace):

```bash
# Temporarily disable the workspace
GOWORK=off go test ./...

# Or work outside this directory structure
```

## License

See [LICENSE](LICENSE) for details.

