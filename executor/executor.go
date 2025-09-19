// Package executor provides a flexible and powerful way to execute external commands
// with features like retry logic, output capture, environment variable management,
// and context support for proper cancellation and timeouts.
package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Result holds the output and error from a command execution
type Result struct {
	Stdout   string
	Stderr   string
	Combined string
	ExitCode int
	Err      error
}

// Executor defines the interface for command execution
type Executor interface {
	// Execute runs a command with the given options
	Execute(ctx context.Context, opts ...Option) (*Result, error)

	// ExecuteWithInput runs a command with stdin input
	ExecuteWithInput(ctx context.Context, input string, opts ...Option) (*Result, error)
}

// CommandExecutor implements the Executor interface
type CommandExecutor struct {
	program string
	args    []string
	options *Options
}

// Options configures command execution behavior
type Options struct {
	// Output handling
	CaptureStdout     bool
	CaptureStderr     bool
	CaptureCombined   bool
	RedirectToConsole bool

	// Retry configuration
	MaxRetries int
	RetryDelay time.Duration
	RetryOn    func(error) bool // Custom retry condition

	// Working directory
	WorkingDir string

	// Environment variables (appended to current env)
	Env map[string]string

	// Custom stdout/stderr writers (for advanced use cases)
	StdoutWriter io.Writer
	StderrWriter io.Writer
}

// Option is a function that modifies Options
type Option func(*Options)

// DefaultOptions returns default execution options
func DefaultOptions() *Options {
	return &Options{
		CaptureStdout:     true,
		CaptureStderr:     true,
		CaptureCombined:   false,
		RedirectToConsole: false,
		MaxRetries:        0,
		RetryDelay:        time.Second,
		RetryOn:           nil,
		Env:               make(map[string]string),
	}
}

// New creates a new CommandExecutor
func New(program string, args ...string) *CommandExecutor {
	return &CommandExecutor{
		program: program,
		args:    args,
		options: DefaultOptions(),
	}
}

// NewWrappedExecutor creates an executor for a specific program
func NewWrappedExecutor(program string) *WrappedExecutor {
	return &WrappedExecutor{
		program: program,
		options: DefaultOptions(),
	}
}

// WrappedExecutor provides a clean interface for a specific program
type WrappedExecutor struct {
	program string
	options *Options
}

// Command creates a new executor for the wrapped program with specific arguments
func (w *WrappedExecutor) Command(args ...string) *CommandExecutor {
	return &CommandExecutor{
		program: w.program,
		args:    args,
		options: w.options,
	}
}

// Execute runs the command with the wrapped program
func (w *WrappedExecutor) Execute(
	ctx context.Context,
	args []string,
	opts ...Option,
) (*Result, error) {
	result, err := w.Command(args...).Execute(ctx, opts...)
	if err != nil {
		return result, fmt.Errorf("failed to execute command with args %v: %w", args, err)
	}
	return result, nil
}

// ExecuteSimple is a convenience method for simple command execution
func (w *WrappedExecutor) ExecuteSimple(args ...string) (*Result, error) {
	result, err := w.Command(args...).Execute(context.Background())
	if err != nil {
		return result, fmt.Errorf("failed to execute simple command with args %v: %w", args, err)
	}
	return result, nil
}

// Execute implements the Executor interface
func (c *CommandExecutor) Execute(ctx context.Context, opts ...Option) (*Result, error) {
	return c.ExecuteWithInput(ctx, "", opts...)
}

// ExecuteWithInput implements the Executor interface with stdin support
func (c *CommandExecutor) ExecuteWithInput(
	ctx context.Context,
	input string,
	opts ...Option,
) (*Result, error) {
	// Apply options
	options := c.mergeOptions(opts...)

	// Setup retry logic
	maxAttempts := options.MaxRetries + 1
	var lastResult *Result

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := c.executeOnce(ctx, input, options)
		lastResult = result

		// Success or non-retryable error
		if err == nil || attempt == maxAttempts {
			return result, err
		}

		// Check if we should retry
		if options.RetryOn != nil && !options.RetryOn(err) {
			return result, err
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(options.RetryDelay):
			// Continue to next attempt
		}
	}

	return lastResult, lastResult.Err
}

// setupCommand configures the exec.Cmd with working directory, environment, and input
func (c *CommandExecutor) setupCommand(cmd *exec.Cmd, input string, options *Options) {
	// Set working directory
	if options.WorkingDir != "" {
		cmd.Dir = options.WorkingDir
	}

	// Set environment
	if len(options.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range options.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Setup input
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
}

// setupOutputCapture configures stdout and stderr writers for the command
func (c *CommandExecutor) setupOutputCapture(
	cmd *exec.Cmd,
	options *Options,
) (*bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	var stdoutBuf, stderrBuf, combinedBuf bytes.Buffer

	// Configure stdout
	stdoutWriters := []io.Writer{}
	if options.CaptureStdout || options.CaptureCombined {
		if options.CaptureCombined {
			stdoutWriters = append(stdoutWriters, &combinedBuf)
		} else {
			stdoutWriters = append(stdoutWriters, &stdoutBuf)
		}
	}
	if options.RedirectToConsole {
		stdoutWriters = append(stdoutWriters, os.Stdout)
	}
	if options.StdoutWriter != nil {
		stdoutWriters = append(stdoutWriters, options.StdoutWriter)
	}

	if len(stdoutWriters) > 0 {
		cmd.Stdout = io.MultiWriter(stdoutWriters...)
	}

	// Configure stderr
	stderrWriters := []io.Writer{}
	if options.CaptureStderr || options.CaptureCombined {
		if options.CaptureCombined {
			stderrWriters = append(stderrWriters, &combinedBuf)
		} else {
			stderrWriters = append(stderrWriters, &stderrBuf)
		}
	}
	if options.RedirectToConsole {
		stderrWriters = append(stderrWriters, os.Stderr)
	}
	if options.StderrWriter != nil {
		stderrWriters = append(stderrWriters, options.StderrWriter)
	}

	if len(stderrWriters) > 0 {
		cmd.Stderr = io.MultiWriter(stderrWriters...)
	}

	return &stdoutBuf, &stderrBuf, &combinedBuf
}

// createResult creates a Result from command execution and error
func (c *CommandExecutor) createResult(
	stdoutBuf, stderrBuf, combinedBuf *bytes.Buffer,
	err error,
) *Result {
	result := &Result{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Combined: combinedBuf.String(),
		Err:      err,
	}

	// Get exit code
	var exitErr *exec.ExitError
	switch {
	case err != nil && errors.As(err, &exitErr):
		result.ExitCode = exitErr.ExitCode()
	case err == nil:
		result.ExitCode = 0
	default:
		result.ExitCode = -1
	}

	return result
}

func (c *CommandExecutor) executeOnce(
	ctx context.Context,
	input string,
	options *Options,
) (*Result, error) {
	cmd := exec.CommandContext(ctx, c.program, c.args...)

	c.setupCommand(cmd, input, options)
	stdoutBuf, stderrBuf, combinedBuf := c.setupOutputCapture(cmd, options)

	// Execute command
	err := cmd.Run()

	// Prepare result
	result := c.createResult(stdoutBuf, stderrBuf, combinedBuf, err)

	if err != nil {
		return result, fmt.Errorf("command execution failed: %w", err)
	}
	return result, nil
}

func (c *CommandExecutor) mergeOptions(opts ...Option) *Options {
	// Copy base options
	merged := *c.options

	// Apply option functions
	for _, opt := range opts {
		opt(&merged)
	}

	return &merged
}

// Option functions for fluent configuration

// WithCapture configures output capture
func WithCapture(stdout, stderr, combined bool) Option {
	return func(o *Options) {
		o.CaptureStdout = stdout
		o.CaptureStderr = stderr
		o.CaptureCombined = combined
	}
}

// WithConsoleRedirect enables/disables console output
func WithConsoleRedirect(redirect bool) Option {
	return func(o *Options) {
		o.RedirectToConsole = redirect
	}
}

// WithRetry configures retry behavior
func WithRetry(maxRetries int, delay time.Duration) Option {
	return func(o *Options) {
		o.MaxRetries = maxRetries
		o.RetryDelay = delay
	}
}

// WithRetryCondition sets a custom retry condition
func WithRetryCondition(fn func(error) bool) Option {
	return func(o *Options) {
		o.RetryOn = fn
	}
}

// WithWorkingDir sets the working directory
func WithWorkingDir(dir string) Option {
	return func(o *Options) {
		o.WorkingDir = dir
	}
}

// WithEnv adds environment variables
func WithEnv(env map[string]string) Option {
	return func(o *Options) {
		if o.Env == nil {
			o.Env = make(map[string]string)
		}
		for k, v := range env {
			o.Env[k] = v
		}
	}
}

// WithEnvVar adds a single environment variable
func WithEnvVar(key, value string) Option {
	return func(o *Options) {
		if o.Env == nil {
			o.Env = make(map[string]string)
		}
		o.Env[key] = value
	}
}

// WithStdoutWriter sets a custom stdout writer
func WithStdoutWriter(w io.Writer) Option {
	return func(o *Options) {
		o.StdoutWriter = w
	}
}

// WithStderrWriter sets a custom stderr writer
func WithStderrWriter(w io.Writer) Option {
	return func(o *Options) {
		o.StderrWriter = w
	}
}

// Convenience functions for common patterns

// CaptureAll captures and redirects to console simultaneously
func CaptureAll() Option {
	return func(o *Options) {
		o.CaptureStdout = true
		o.CaptureStderr = true
		o.RedirectToConsole = true
	}
}

// SilentMode captures output without console redirect
func SilentMode() Option {
	return func(o *Options) {
		o.CaptureStdout = true
		o.CaptureStderr = true
		o.RedirectToConsole = false
	}
}

// ConsoleOnly redirects to console without capture
func ConsoleOnly() Option {
	return func(o *Options) {
		o.CaptureStdout = false
		o.CaptureStderr = false
		o.RedirectToConsole = true
	}
}
