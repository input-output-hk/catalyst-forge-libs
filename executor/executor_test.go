package executor_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/input-output-hk/catalyst-forge-libs/executor"
)

// MockExecutor implements the Executor interface for testing
type MockExecutor struct {
	ExecuteFunc          func(ctx context.Context, opts ...executor.Option) (*executor.Result, error)
	ExecuteWithInputFunc func(ctx context.Context, input string, opts ...executor.Option) (*executor.Result, error)
	CallCount            int
}

func (m *MockExecutor) Execute(ctx context.Context, opts ...executor.Option) (*executor.Result, error) {
	m.CallCount++
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, opts...)
	}
	return &executor.Result{
		Stdout:   "mock stdout",
		Stderr:   "mock stderr",
		ExitCode: 0,
	}, nil
}

func (m *MockExecutor) ExecuteWithInput(
	ctx context.Context,
	input string,
	opts ...executor.Option,
) (*executor.Result, error) {
	m.CallCount++
	if m.ExecuteWithInputFunc != nil {
		return m.ExecuteWithInputFunc(ctx, input, opts...)
	}
	return m.Execute(ctx, opts...)
}

func TestBasicExecution(t *testing.T) {
	// Test basic command execution
	cmd := executor.New("echo", "hello", "world")
	result, err := cmd.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Stdout, "hello world") {
		t.Errorf("expected stdout to contain 'hello world', got: %s", result.Stdout)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got: %d", result.ExitCode)
	}
}

func TestWrappedExecutor(t *testing.T) {
	// Create a wrapped git executor
	git := executor.NewWrappedExecutor("git")

	// Execute git version
	result, err := git.ExecuteSimple("version")
	if err != nil {
		t.Skipf("git not available: %v", err)
	}

	if !strings.Contains(result.Stdout, "git version") {
		t.Errorf("expected git version output, got: %s", result.Stdout)
	}
}

func TestCaptureAndRedirect(t *testing.T) {
	// Test simultaneous capture and console redirect
	cmd := executor.New("echo", "test output")
	result, err := cmd.Execute(context.Background(), executor.CaptureAll())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Stdout, "test output") {
		t.Errorf("expected captured stdout, got: %s", result.Stdout)
	}
}

func TestCombinedOutput(t *testing.T) {
	// Test combined stdout/stderr capture
	cmd := executor.New("sh", "-c", "echo stdout && echo stderr >&2")
	result, err := cmd.Execute(
		context.Background(),
		executor.WithCapture(false, false, true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	combined := result.Combined
	if !strings.Contains(combined, "stdout") || !strings.Contains(combined, "stderr") {
		t.Errorf("expected combined output, got: %s", combined)
	}
}

func TestRetryMechanism(t *testing.T) {
	attemptCount := 0

	// Create a mock that fails twice then succeeds
	mock := &MockExecutor{
		ExecuteFunc: func(ctx context.Context, opts ...executor.Option) (*executor.Result, error) {
			attemptCount++
			if attemptCount < 3 {
				return &executor.Result{
					Stderr:   fmt.Sprintf("attempt %d failed", attemptCount),
					ExitCode: 1,
				}, fmt.Errorf("command failed")
			}
			return &executor.Result{
				Stdout:   "success",
				ExitCode: 0,
			}, nil
		},
	}

	// In real usage, you would use the actual executor with retry options
	// For testing, we demonstrate the mock pattern
	ctx := context.Background()
	result, err := mock.Execute(ctx)

	// Simulate retry logic
	maxRetries := 3
	for i := 1; i < maxRetries && err != nil; i++ {
		time.Sleep(10 * time.Millisecond)
		result, err = mock.Execute(ctx)
	}

	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}

	if result.Stdout != "success" {
		t.Errorf("expected success output, got: %s", result.Stdout)
	}

	if attemptCount != 3 {
		t.Errorf("expected 3 attempts, got: %d", attemptCount)
	}
}

func TestWithInput(t *testing.T) {
	// Test command with stdin input
	cmd := executor.New("cat")
	input := "hello from stdin"

	result, err := cmd.ExecuteWithInput(
		context.Background(),
		input,
		executor.SilentMode(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.TrimSpace(result.Stdout) != input {
		t.Errorf("expected stdout to match input, got: %s", result.Stdout)
	}
}

func TestWorkingDirectory(t *testing.T) {
	// Test execution with custom working directory
	cmd := executor.New("pwd")
	result, err := cmd.Execute(
		context.Background(),
		executor.WithWorkingDir("/tmp"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Stdout, "/tmp") {
		t.Errorf("expected /tmp in output, got: %s", result.Stdout)
	}
}

func TestEnvironmentVariables(t *testing.T) {
	// Test with custom environment variables
	cmd := executor.New("sh", "-c", "echo $CUSTOM_VAR")
	result, err := cmd.Execute(
		context.Background(),
		executor.WithEnvVar("CUSTOM_VAR", "test_value"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Stdout, "test_value") {
		t.Errorf("expected env var value in output, got: %s", result.Stdout)
	}
}

func TestContextCancellation(t *testing.T) {
	// Test context cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	cmd := executor.New("sleep", "1")
	_, err := cmd.Execute(ctx)

	if err == nil {
		t.Error("expected context cancellation error")
	}
}

// Example usage demonstrations
func ExampleNew() {
	// Simple command execution
	cmd := executor.New("echo", "Hello, World!")
	result, err := cmd.Execute(context.Background())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Output: %s\n", result.Stdout)
}

func ExampleNewWrappedExecutor() {
	// Create a git wrapper
	git := executor.NewWrappedExecutor("git")

	// Use it for various git commands
	result, err := git.ExecuteSimple("status", "--short")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Git status: %s\n", result.Stdout)

	// With options
	result, err = git.Execute(
		context.Background(),
		[]string{"log", "--oneline", "-5"},
		executor.SilentMode(),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Recent commits: %s\n", result.Stdout)
}

func ExampleCommandExecutor_Execute_withRetry() {
	// Command with retry logic
	cmd := executor.New("curl", "https://api.example.com/data")

	result, err := cmd.Execute(
		context.Background(),
		executor.WithRetry(3, 2*time.Second),
		executor.WithRetryCondition(func(err error) bool {
			// Retry on any error (customize as needed)
			return err != nil
		}),
	)
	if err != nil {
		fmt.Printf("Failed after retries: %v\n", err)
		return
	}
	fmt.Printf("Response: %s\n", result.Stdout)
}

func ExampleCommandExecutor_Execute_captureAndDisplay() {
	// Both capture output and show on console
	cmd := executor.New("ls", "-la")

	result, err := cmd.Execute(
		context.Background(),
		executor.CaptureAll(), // Captures AND displays
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Output was displayed on console during execution
	// AND is available in result
	fmt.Printf("Captured %d bytes of output\n", len(result.Stdout))
}
