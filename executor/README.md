# Command Executor for Go

A minimal, flexible, and testable command execution wrapper for Go with zero dependencies.

## Features

✅ **Interface-based API** - Easy mocking for unit tests
✅ **Flexible output handling** - Capture stdout, stderr, or combined output
✅ **Console redirection** - Show output while capturing it
✅ **Simultaneous capture and display** - Get the best of both worlds
✅ **Retry support** - Built-in retry mechanism with customizable conditions
✅ **Program wrappers** - Clean interface for specific programs
✅ **Zero dependencies** - No external dependencies required

## Installation

```bash
go get github.com/yourusername/executor
```

## Quick Start

### Simple Command Execution

```go
import "github.com/yourusername/executor"

// Execute a simple command
cmd := executor.New("echo", "Hello, World!")
result, err := cmd.Execute(context.Background())
if err != nil {
    log.Fatal(err)
}
fmt.Println(result.Stdout) // "Hello, World!"
```

### Wrapped Executors

```go
// Create a git wrapper
git := executor.NewWrappedExecutor("git")

// Simple execution
result, err := git.ExecuteSimple("status", "--short")

// With options
result, err = git.Execute(
    context.Background(),
    []string{"commit", "-m", "Initial commit"},
    executor.CaptureAll(), // Capture and display output
)
```

### Output Handling Options

```go
// Capture output silently
cmd.Execute(ctx, executor.SilentMode())

// Display on console only (no capture)
cmd.Execute(ctx, executor.ConsoleOnly())

// Both capture AND display (useful for CI/CD)
cmd.Execute(ctx, executor.CaptureAll())

// Custom configuration
cmd.Execute(ctx,
    executor.WithCapture(true, true, false), // stdout, stderr, not combined
    executor.WithConsoleRedirect(true),
)
```

### Retry Support

```go
cmd := executor.New("curl", "https://api.example.com")
result, err := cmd.Execute(
    context.Background(),
    executor.WithRetry(3, 2*time.Second),
    executor.WithRetryCondition(func(err error) bool {
        // Retry on any network error
        return err != nil && strings.Contains(err.Error(), "network")
    }),
)
```

### Input Support

```go
// Send input to stdin
cmd := executor.New("cat")
result, err := cmd.ExecuteWithInput(
    context.Background(),
    "Hello from stdin",
    executor.SilentMode(),
)
fmt.Println(result.Stdout) // "Hello from stdin"
```

### Environment and Working Directory

```go
cmd := executor.New("env")
result, err := cmd.Execute(
    context.Background(),
    executor.WithWorkingDir("/tmp"),
    executor.WithEnvVar("CUSTOM_VAR", "value"),
    executor.WithEnv(map[string]string{
        "VAR1": "value1",
        "VAR2": "value2",
    }),
)
```

## Testing with Mocks

The interface-based design makes testing easy:

```go
type MockExecutor struct {
    ExecuteFunc func(ctx context.Context, opts ...executor.Option) (*executor.Result, error)
}

func (m *MockExecutor) Execute(ctx context.Context, opts ...executor.Option) (*executor.Result, error) {
    if m.ExecuteFunc != nil {
        return m.ExecuteFunc(ctx, opts...)
    }
    return &executor.Result{
        Stdout: "mock output",
        ExitCode: 0,
    }, nil
}

// In your test
mock := &MockExecutor{
    ExecuteFunc: func(ctx context.Context, opts ...executor.Option) (*executor.Result, error) {
        return &executor.Result{
            Stdout: "expected output",
        }, nil
    },
}
```

## Advanced Usage

### Custom Program Wrapper

```go
type DockerClient struct {
    executor *executor.WrappedExecutor
}

func NewDockerClient() *DockerClient {
    return &DockerClient{
        executor: executor.NewWrappedExecutor("docker"),
    }
}

func (d *DockerClient) ListContainers() ([]string, error) {
    result, err := d.executor.Execute(
        context.Background(),
        []string{"ps", "--format", "{{.Names}}"},
        executor.SilentMode(),
    )
    if err != nil {
        return nil, err
    }

    containers := strings.Split(strings.TrimSpace(result.Stdout), "\n")
    return containers, nil
}

func (d *DockerClient) RunContainer(image string, command []string) error {
    args := append([]string{"run", "--rm", image}, command...)
    _, err := d.executor.Execute(
        context.Background(),
        args,
        executor.CaptureAll(), // Show progress and capture output
        executor.WithRetry(2, time.Second),
    )
    return err
}
```

### Pipeline Processing

```go
// Step 1: Find files
find := executor.New("find", ".", "-name", "*.txt")
result, err := find.Execute(context.Background(), executor.SilentMode())
if err != nil {
    return err
}

files := strings.Split(result.Stdout, "\n")

// Step 2: Process each file
for _, file := range files {
    grep := executor.New("grep", "pattern", file)
    result, err := grep.Execute(context.Background())
    if err == nil && result.Stdout != "" {
        fmt.Printf("Found in %s: %s\n", file, result.Stdout)
    }
}
```

## API Reference

### Core Types

- `Executor` - Main interface for command execution
- `CommandExecutor` - Standard implementation
- `WrappedExecutor` - Program-specific wrapper
- `Result` - Execution result with outputs and exit code
- `Options` - Configuration for command execution

### Option Functions

- `WithCapture(stdout, stderr, combined bool)` - Configure output capture
- `WithConsoleRedirect(bool)` - Enable/disable console output
- `WithRetry(maxRetries int, delay time.Duration)` - Set retry parameters
- `WithRetryCondition(func(error) bool)` - Custom retry logic
- `WithWorkingDir(string)` - Set working directory
- `WithEnv(map[string]string)` - Add environment variables
- `WithEnvVar(key, value string)` - Add single environment variable
- `WithStdoutWriter(io.Writer)` - Custom stdout handler
- `WithStderrWriter(io.Writer)` - Custom stderr handler

### Convenience Options

- `CaptureAll()` - Capture and display simultaneously
- `SilentMode()` - Capture without console output
- `ConsoleOnly()` - Display without capture

## Design Decisions

1. **Zero Dependencies**: The module has no external dependencies, making it lightweight and easy to integrate.

2. **Interface-Based**: The `Executor` interface allows for easy mocking and testing.

3. **Functional Options**: The option pattern provides flexible, extensible configuration without breaking changes.

4. **io.MultiWriter**: Uses Go's standard `io.MultiWriter` for simultaneous capture and display.

5. **Context Support**: All operations support context for cancellation and timeouts.

6. **Thread Safety**: Each execution creates its own buffers and state, making it safe for concurrent use.

## License

MIT License - see LICENSE file for details