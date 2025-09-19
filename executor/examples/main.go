// Package main demonstrates various usage patterns of the executor package.
// It includes examples of simple command execution, wrapped executors for common tools,
// retry logic, pipeline operations, and interactive commands with input.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/input-output-hk/catalyst-forge-libs/executor"
)

func main() {
	// Example 1: Simple command execution
	simpleExample()

	// Example 2: Git wrapper
	gitWrapperExample()

	// Example 3: Docker wrapper with retry
	dockerWrapperExample()

	// Example 4: Pipeline execution
	pipelineExample()

	// Example 5: Interactive command with input
	interactiveExample()
}

func simpleExample() {
	fmt.Println("=== Simple Command Example ===")

	// Execute a simple command
	cmd := executor.New("date")
	result, err := cmd.Execute(context.Background())
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Current date: %s\n", result.Stdout)
}

func gitWrapperExample() {
	fmt.Println("\n=== Git Wrapper Example ===")

	// Create a git wrapper
	git := executor.NewWrappedExecutor("git")

	// Check git version
	result, err := git.ExecuteSimple("version")
	if err != nil {
		log.Printf("Git not available: %v\n", err)
		return
	}
	fmt.Printf("Git version: %s\n", strings.TrimSpace(result.Stdout))

	// Get current branch with silent mode
	result, err = git.Execute(
		context.Background(),
		[]string{"branch", "--show-current"},
		executor.SilentMode(),
	)
	if err == nil {
		fmt.Printf("Current branch: %s\n", strings.TrimSpace(result.Stdout))
	}

	// Get status with console output
	fmt.Println("Git status (with console output):")
	result, err = git.Execute(
		context.Background(),
		[]string{"status", "--short"},
		executor.CaptureAll(), // Both capture and display
	)
	if err != nil {
		log.Printf("Error getting status: %v\n", err)
	}

	fmt.Println("Git status:")
	fmt.Print(result.Stdout)
}

func dockerWrapperExample() {
	fmt.Println("\n=== Docker Wrapper Example ===")

	// Create a docker wrapper with retry support
	docker := executor.NewWrappedExecutor("docker")

	// List containers with retry (useful for transient Docker daemon issues)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := docker.Execute(
		ctx,
		[]string{"ps", "--format", "table {{.Names}}\t{{.Status}}"},
		executor.WithRetry(3, 2*time.Second),
		executor.WithRetryCondition(func(err error) bool {
			// Retry on connection errors
			return err != nil && strings.Contains(err.Error(), "connection")
		}),
	)
	if err != nil {
		log.Printf("Docker not available or error: %v\n", err)
		return
	}

	fmt.Println("Docker containers:")
	fmt.Print(result.Stdout)
}

func pipelineExample() {
	fmt.Println("\n=== Pipeline Example ===")

	// Example: Find Go files and count lines
	// Equivalent to: find . -name "*.go" | head -5

	// Step 1: Find Go files
	find := executor.New("find", ".", "-name", "*.go", "-type", "f")
	result, err := find.Execute(
		context.Background(),
		executor.SilentMode(),
	)
	if err != nil {
		log.Printf("Find failed: %v\n", err)
		return
	}

	// Step 2: Process the output (take first 5 files)
	files := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	maxFiles := 5
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}

	fmt.Printf("First %d Go files found:\n", len(files))
	for _, file := range files {
		if file != "" {
			fmt.Printf("  - %s\n", file)
		}
	}

	// Step 3: Count lines in each file
	for _, file := range files {
		if file == "" {
			continue
		}

		wc := executor.New("wc", "-l", file)
		result, err := wc.Execute(
			context.Background(),
			executor.SilentMode(),
		)
		if err == nil {
			fmt.Printf("    %s", result.Stdout)
		}
	}
}

func interactiveExample() {
	fmt.Println("\n=== Interactive Command Example ===")

	// Example: Use sed to transform input
	sed := executor.New("sed", "s/world/universe/g")

	input := "Hello world! Welcome to the world of Go."
	result, err := sed.ExecuteWithInput(
		context.Background(),
		input,
		executor.SilentMode(),
	)
	if err != nil {
		log.Printf("Sed failed: %v\n", err)
		return
	}

	fmt.Printf("Original: %s\n", input)
	fmt.Printf("Transformed: %s\n", result.Stdout)
}

// Example of a custom command wrapper for your application
type KubectlWrapper struct {
	executor  *executor.WrappedExecutor
	namespace string
}

func NewKubectlWrapper(namespace string) *KubectlWrapper {
	return &KubectlWrapper{
		executor:  executor.NewWrappedExecutor("kubectl"),
		namespace: namespace,
	}
}

func (k *KubectlWrapper) GetPods() ([]string, error) {
	result, err := k.executor.Execute(
		context.Background(),
		[]string{"get", "pods", "-n", k.namespace, "--no-headers", "-o", "custom-columns=:metadata.name"},
		executor.SilentMode(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get pods in namespace %s: %w", k.namespace, err)
	}

	pods := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	return pods, nil
}

func (k *KubectlWrapper) DescribePod(podName string) (*executor.Result, error) {
	result, err := k.executor.Execute(
		context.Background(),
		[]string{"describe", "pod", podName, "-n", k.namespace},
		executor.CaptureAll(), // Show output and capture it
	)
	if err != nil {
		return result, fmt.Errorf("failed to describe pod %s in namespace %s: %w", podName, k.namespace, err)
	}
	return result, nil
}

func (k *KubectlWrapper) ExecInPod(podName string, command []string) (*executor.Result, error) {
	args := append([]string{"exec", "-n", k.namespace, podName, "--"}, command...)

	result, err := k.executor.Execute(
		context.Background(),
		args,
		executor.WithRetry(2, time.Second),
	)
	if err != nil {
		return result, fmt.Errorf("failed to execute command %v in pod %s: %w", command, podName, err)
	}
	return result, nil
}
