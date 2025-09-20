package earthfile

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	// Create a test Earthfile
	content := `VERSION 0.8

FROM golang:1.21

build:
	RUN echo "Building..."
	SAVE ARTIFACT ./bin/app

test:
	FROM +build
	RUN go test ./...
`
	tmpFile, err := os.CreateTemp("", "Earthfile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, writeErr := tmpFile.WriteString(content); writeErr != nil {
		t.Fatal(writeErr)
	}
	tmpFile.Close()

	// Test Parse
	ef, err := Parse(tmpFile.Name())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if ef == nil {
		t.Fatal("Parse() returned nil Earthfile")
	}

	if ef.Version() != "0.8" {
		t.Errorf("Expected version 0.8, got %s", ef.Version())
	}

	if !ef.HasVersion() {
		t.Error("Expected HasVersion() to return true")
	}

	// Check targets were parsed
	targetNames := ef.TargetNames()
	expectedTargets := []string{"build", "test"}
	if len(targetNames) != len(expectedTargets) {
		t.Errorf("Expected %d targets, got %d", len(expectedTargets), len(targetNames))
	}
}

func TestParseString(t *testing.T) {
	content := `VERSION 0.7

deps:
	FROM alpine:3.18
	RUN apk add --no-cache git

build:
	FROM +deps
	COPY . .
	RUN make build
`

	ef, err := ParseString(content)
	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if ef == nil {
		t.Fatal("ParseString() returned nil Earthfile")
	}

	if ef.Version() != "0.7" {
		t.Errorf("Expected version 0.7, got %s", ef.Version())
	}

	// Check we have deps and build targets
	if !ef.HasTarget("deps") {
		t.Error("Expected to have 'deps' target")
	}

	if !ef.HasTarget("build") {
		t.Error("Expected to have 'build' target")
	}
}

func TestParseReader(t *testing.T) {
	content := `VERSION 0.8

test:
	FROM golang:1.21
	RUN go test -v
`

	reader := strings.NewReader(content)

	ef, err := ParseReader(reader, "TestEarthfile")
	if err != nil {
		t.Fatalf("ParseReader() error = %v", err)
	}

	if ef == nil {
		t.Fatal("ParseReader() returned nil Earthfile")
	}

	if ef.Version() != "0.8" {
		t.Errorf("Expected version 0.8, got %s", ef.Version())
	}

	target := ef.Target("test")
	if target == nil {
		t.Fatal("Expected to have 'test' target")
	}
}

func TestParseContext(t *testing.T) {
	// Create a test Earthfile
	content := `VERSION 0.7

build:
	FROM golang:1.21
	RUN go build
`
	tmpFile, err := os.CreateTemp("", "Earthfile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, writeErr := tmpFile.WriteString(content); writeErr != nil {
		t.Fatal(writeErr)
	}
	tmpFile.Close()

	// Test with context
	ctx := context.Background()
	ef, err := ParseContext(ctx, tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseContext() error = %v", err)
	}

	if ef == nil {
		t.Fatal("ParseContext() returned nil Earthfile")
	}

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = ParseContext(ctx, tmpFile.Name())
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestParseErrors(t *testing.T) {
	// Test file not found
	_, err := Parse("/non/existent/file")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Test invalid Earthfile syntax
	invalidContent := `INVALID SYNTAX
	This is not valid
`
	_, err = ParseString(invalidContent)
	if err == nil {
		t.Error("Expected error for invalid syntax")
	}

	// Test empty content
	ef, err := ParseString("")
	if err != nil {
		t.Fatalf("Expected no error for empty content, got %v", err)
	}
	if ef == nil {
		t.Fatal("Expected non-nil Earthfile for empty content")
	}
	if ef.HasVersion() {
		t.Error("Empty Earthfile should not have version")
	}
}

func TestParseWithBaseCommands(t *testing.T) {
	content := `VERSION 0.8
ARG GOLANG_VERSION=1.21

FROM golang:${GOLANG_VERSION}

build:
	RUN go build ./...
`

	ef, err := ParseString(content)
	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	baseCommands := ef.BaseCommands()
	if len(baseCommands) < 2 {
		t.Errorf("Expected at least 2 base commands, got %d", len(baseCommands))
	}

	// Should have ARG and FROM as base commands
	foundArg := false
	foundFrom := false
	for _, cmd := range baseCommands {
		if cmd.Name == "ARG" {
			foundArg = true
		}
		if cmd.Name == "FROM" {
			foundFrom = true
		}
	}

	if !foundArg {
		t.Error("Expected ARG in base commands")
	}
	if !foundFrom {
		t.Error("Expected FROM in base commands")
	}
}
