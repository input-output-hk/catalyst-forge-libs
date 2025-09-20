package earthfile

import (
	"strings"
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/fs/billy"
)

func TestParseWithFilesystem(t *testing.T) {
	// Create an in-memory filesystem
	memFS := billy.NewInMemoryFS()

	// Write a test Earthfile
	testContent := `VERSION 0.7

FROM golang:1.21
WORKDIR /app

build:
    RUN go build -o app .
    SAVE ARTIFACT app AS LOCAL ./dist/app

test:
    RUN go test ./...
`
	if err := memFS.WriteFile("Earthfile", []byte(testContent), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Parse using the in-memory filesystem
	opts := &ParseOptions{
		Filesystem:      memFS,
		EnableSourceMap: false,
		StrictMode:      false,
	}

	ef, err := ParseWithOptions("Earthfile", opts)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Verify basic parsing worked
	if ef.Version() != "0.7" {
		t.Errorf("Expected version 0.7, got %s", ef.Version())
	}

	if !ef.HasTarget("build") {
		t.Error("Expected to find 'build' target")
	}

	if !ef.HasTarget("test") {
		t.Error("Expected to find 'test' target")
	}

	// Verify target names
	names := ef.TargetNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(names))
	}
}

func TestParseStringWithoutFilesystem(t *testing.T) {
	// Test that ParseString works without filesystem side effects
	testContent := `VERSION 0.8

deps:
    FROM alpine:latest
    RUN apk add --no-cache curl

deploy:
    FROM +deps
    COPY deploy.sh /usr/local/bin/
    ENTRYPOINT ["/usr/local/bin/deploy.sh"]
`

	ef, err := ParseString(testContent)
	if err != nil {
		t.Fatalf("Failed to parse string: %v", err)
	}

	// Verify parsing worked
	if ef.Version() != "0.8" {
		t.Errorf("Expected version 0.8, got %s", ef.Version())
	}

	if !ef.HasTarget("deps") {
		t.Error("Expected to find 'deps' target")
	}

	if !ef.HasTarget("deploy") {
		t.Error("Expected to find 'deploy' target")
	}
}

func TestParseReaderWithFS(t *testing.T) {
	testContent := `VERSION 0.6

build:
    FROM node:18
    WORKDIR /app
    COPY package.json .
    RUN npm install
    COPY . .
    RUN npm run build
    SAVE ARTIFACT dist AS LOCAL ./dist
`

	reader := strings.NewReader(testContent)
	ef, err := ParseReader(reader, "test.earth")
	if err != nil {
		t.Fatalf("Failed to parse reader: %v", err)
	}

	// Verify parsing worked
	if ef.Version() != "0.6" {
		t.Errorf("Expected version 0.6, got %s", ef.Version())
	}

	if !ef.HasTarget("build") {
		t.Error("Expected to find 'build' target")
	}

	// Check that we have commands in the target
	buildTarget := ef.Target("build")
	if buildTarget == nil {
		t.Fatal("Build target should not be nil")
	}

	if len(buildTarget.Commands) == 0 {
		t.Error("Build target should have commands")
	}
}

func TestParseVersionWithFilesystem(t *testing.T) {
	// Create an in-memory filesystem
	memFS := billy.NewInMemoryFS()

	// Test various VERSION formats
	testCases := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Simple version",
			content:  "VERSION 0.7",
			expected: "0.7",
		},
		{
			name:     "Version with comments",
			content:  "# Comment\n\nVERSION 0.8",
			expected: "0.8",
		},
		{
			name:     "No version",
			content:  "FROM golang:1.21",
			expected: "",
		},
		{
			name:     "Version with extra args",
			content:  "VERSION --explicit-global 0.7",
			expected: "0.7",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write test file
			filename := "test-" + tc.name + ".earth"
			if err := memFS.WriteFile(filename, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Parse version
			version, err := ParseVersionWithFilesystem(filename, memFS)
			if err != nil {
				t.Fatalf("Failed to parse version: %v", err)
			}

			if version != tc.expected {
				t.Errorf("Expected version %q, got %q", tc.expected, version)
			}
		})
	}
}
