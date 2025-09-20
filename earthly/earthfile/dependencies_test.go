package earthfile

import (
	"testing"
)

func TestDependencies(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []Dependency
	}{
		{
			name: "no dependencies",
			content: `VERSION 0.7

build:
	FROM alpine:3.14
	RUN echo "hello"
`,
			expected: []Dependency{},
		},
		{
			name: "local BUILD dependency",
			content: `VERSION 0.7

build:
	FROM alpine:3.14
	BUILD +test

test:
	FROM alpine:3.14
	RUN echo "test"
`,
			expected: []Dependency{
				{Target: "+test", Local: true, Source: "build"},
			},
		},
		{
			name: "multiple local BUILD dependencies",
			content: `VERSION 0.7

build:
	BUILD +test1
	BUILD +test2

test1:
	FROM alpine:3.14

test2:
	FROM alpine:3.14
`,
			expected: []Dependency{
				{Target: "+test1", Local: true, Source: "build"},
				{Target: "+test2", Local: true, Source: "build"},
			},
		},
		{
			name: "remote BUILD dependency",
			content: `VERSION 0.7

build:
	BUILD github.com/earthly/earthly+test
`,
			expected: []Dependency{
				{Target: "github.com/earthly/earthly+test", Local: false, Source: "build"},
			},
		},
		{
			name: "FROM local dependency",
			content: `VERSION 0.7

build:
	FROM +other

other:
	FROM alpine:3.14
`,
			expected: []Dependency{
				{Target: "+other", Local: true, Source: "build"},
			},
		},
		{
			name: "FROM remote dependency",
			content: `VERSION 0.7

build:
	FROM github.com/earthly/earthly+docker
`,
			expected: []Dependency{
				{Target: "github.com/earthly/earthly+docker", Local: false, Source: "build"},
			},
		},
		{
			name: "COPY artifact dependency",
			content: `VERSION 0.7

app:
	COPY +build/app /usr/local/bin/

build:
	FROM golang:1.19
	RUN go build -o app
	SAVE ARTIFACT app
`,
			expected: []Dependency{
				{Target: "+build", Local: true, Source: "app"},
			},
		},
		{
			name: "COPY remote artifact dependency",
			content: `VERSION 0.7

build:
	COPY github.com/example/project+build/binary /usr/local/bin/
`,
			expected: []Dependency{
				{Target: "github.com/example/project+build", Local: false, Source: "build"},
			},
		},
		{
			name: "mixed dependencies",
			content: `VERSION 0.7

main:
	FROM +deps
	COPY +build/app /app
	BUILD +test

deps:
	FROM alpine:3.14

build:
	FROM golang:1.19
	RUN go build -o app
	SAVE ARTIFACT app

test:
	FROM +deps
	COPY +build/app /test-app
`,
			expected: []Dependency{
				{Target: "+deps", Local: true, Source: "main"},
				{Target: "+build", Local: true, Source: "main"},
				{Target: "+test", Local: true, Source: "main"},
				{Target: "+deps", Local: true, Source: "test"},
				{Target: "+build", Local: true, Source: "test"},
			},
		},
		{
			name: "dependencies in base recipe",
			content: `VERSION 0.7

BUILD +setup

build:
	FROM +deps

setup:
	FROM alpine:3.14

deps:
	FROM alpine:3.14
`,
			expected: []Dependency{
				{Target: "+setup", Local: true, Source: ""},
				{Target: "+deps", Local: true, Source: "build"},
			},
		},
		{
			name: "nested dependencies in control flow",
			content: `VERSION 0.7

build:
	IF [ "$ENV" = "prod" ]
		BUILD +prod
	ELSE
		BUILD +dev
	END

	FOR target IN test1 test2
		BUILD +$target
	END

prod:
	FROM alpine:3.14

dev:
	FROM alpine:3.14

test1:
	FROM alpine:3.14

test2:
	FROM alpine:3.14
`,
			expected: []Dependency{
				{Target: "+prod", Local: true, Source: "build"},
				{Target: "+dev", Local: true, Source: "build"},
				// Note: FOR loop variables are not expanded at parse time
				{Target: "+$target", Local: true, Source: "build"},
			},
		},
		{
			name: "COPY with --from flag",
			content: `VERSION 0.7

app:
	COPY --from=+build /app/binary /usr/local/bin/

build:
	FROM golang:1.19
	RUN go build -o /app/binary
`,
			expected: []Dependency{
				{Target: "+build", Local: true, Source: "app"},
			},
		},
		{
			name: "relative path dependencies",
			content: `VERSION 0.7

build:
	BUILD ./subdir+target
	FROM ../other+deps
	COPY ./another+build/artifact /app/
`,
			expected: []Dependency{
				{Target: "./subdir+target", Local: true, Source: "build"},
				{Target: "../other+deps", Local: true, Source: "build"},
				{Target: "./another+build", Local: true, Source: "build"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef, err := ParseString(tt.content)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			// Call Dependencies() multiple times to test lazy loading
			deps1 := ef.Dependencies()
			deps2 := ef.Dependencies()

			// Should return the same instance (lazy loaded)
			if len(deps1) != len(deps2) {
				t.Errorf("Lazy loading not working: first call returned %d deps, second returned %d",
					len(deps1), len(deps2))
			}

			// Check dependencies count
			if len(deps1) != len(tt.expected) {
				t.Errorf("Expected %d dependencies, got %d", len(tt.expected), len(deps1))
				t.Logf("Expected: %+v", tt.expected)
				t.Logf("Got: %+v", deps1)
				return
			}

			// Check each dependency
			for i, expected := range tt.expected {
				if i >= len(deps1) {
					t.Errorf("Missing dependency at index %d: expected %+v", i, expected)
					continue
				}

				actual := deps1[i]
				if actual.Target != expected.Target {
					t.Errorf("Dependency %d: expected target %q, got %q", i, expected.Target, actual.Target)
				}
				if actual.Local != expected.Local {
					t.Errorf("Dependency %d: expected Local=%v, got %v", i, expected.Local, actual.Local)
				}
				if actual.Source != expected.Source {
					t.Errorf("Dependency %d: expected source %q, got %q", i, expected.Source, actual.Source)
				}
			}
		})
	}
}

func TestDependencies_LazyLoading(t *testing.T) {
	content := `VERSION 0.7

build:
	BUILD +test

test:
	FROM alpine:3.14
`

	ef, err := ParseString(content)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Dependencies should be nil initially
	if ef.dependencies != nil {
		t.Error("Dependencies should not be initialized before first call")
	}

	// First call should initialize
	deps := ef.Dependencies()
	if ef.dependencies == nil {
		t.Error("Dependencies should be initialized after first call")
	}

	// Should have the expected dependency
	if len(deps) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(deps))
	}

	// Second call should return cached result
	deps2 := ef.Dependencies()
	if len(deps2) != 1 {
		t.Errorf("Expected cached result with 1 dependency, got %d", len(deps2))
	}
}

func TestDependencies_EmptyEarthfile(t *testing.T) {
	content := `VERSION 0.7`

	ef, err := ParseString(content)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	deps := ef.Dependencies()
	if len(deps) != 0 {
		t.Errorf("Expected no dependencies for empty Earthfile, got %d", len(deps))
	}
}

func TestDependencies_ComplexTargetReference(t *testing.T) {
	content := `VERSION 0.7

build:
	BUILD github.com/earthly/earthly/examples/go+docker
	BUILD ./../../other/project+target
	COPY github.com/org/repo+build/artifact/nested/path /dst/
`

	ef, err := ParseString(content)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	deps := ef.Dependencies()
	if len(deps) != 3 {
		t.Errorf("Expected 3 dependencies, got %d", len(deps))
	}

	if len(deps) >= 1 {
		// Check remote dependency
		if deps[0].Target != "github.com/earthly/earthly/examples/go+docker" || deps[0].Local {
			t.Errorf("First dependency should be remote: %+v", deps[0])
		}
	}

	if len(deps) >= 2 {
		// Check relative local dependency
		if deps[1].Target != "./../../other/project+target" || !deps[1].Local {
			t.Errorf("Second dependency should be local: %+v", deps[1])
		}
	}

	if len(deps) >= 3 {
		// Check COPY with artifact path
		if deps[2].Target != "github.com/org/repo+build" || deps[2].Local {
			t.Errorf("Third dependency should be remote (extracted from COPY): %+v", deps[2])
		}
	}
}
