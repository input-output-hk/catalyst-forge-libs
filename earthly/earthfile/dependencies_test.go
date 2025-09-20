package earthfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			expected: nil,
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
			require.NoError(t, err, "ParseString should not return an error")

			// Call Dependencies() multiple times to test lazy loading
			deps1 := ef.Dependencies()
			deps2 := ef.Dependencies()

			// Should return the same instance (lazy loaded)
			assert.Equal(t, len(deps1), len(deps2), "Lazy loading should return consistent dependency count")

			// Check dependencies
			assert.Equal(t, tt.expected, deps1, "Dependencies should match expected")
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
	require.NoError(t, err, "ParseString should not return an error")

	// Dependencies should be nil initially
	assert.Nil(t, ef.dependencies, "Dependencies should not be initialized before first call")

	// First call should initialize
	deps := ef.Dependencies()
	assert.NotNil(t, ef.dependencies, "Dependencies should be initialized after first call")

	// Should have the expected dependency
	assert.Len(t, deps, 1, "Should have exactly 1 dependency")

	// Second call should return cached result
	deps2 := ef.Dependencies()
	assert.Len(t, deps2, 1, "Cached result should have exactly 1 dependency")
}

func TestDependencies_EmptyEarthfile(t *testing.T) {
	content := `VERSION 0.7`

	ef, err := ParseString(content)
	require.NoError(t, err, "ParseString should not return an error")

	deps := ef.Dependencies()
	assert.Len(t, deps, 0, "Empty Earthfile should have no dependencies")
}

func TestDependencies_ComplexTargetReference(t *testing.T) {
	content := `VERSION 0.7

build:
	BUILD github.com/earthly/earthly/examples/go+docker
	BUILD ./../../other/project+target
	COPY github.com/org/repo+build/artifact/nested/path /dst/
`

	ef, err := ParseString(content)
	require.NoError(t, err, "ParseString should not return an error")

	deps := ef.Dependencies()
	require.Len(t, deps, 3, "Should have exactly 3 dependencies")

	// Check remote dependency
	assert.Equal(t, "github.com/earthly/earthly/examples/go+docker", deps[0].Target, "First dependency target mismatch")
	assert.False(t, deps[0].Local, "First dependency should be remote")

	// Check relative local dependency
	assert.Equal(t, "./../../other/project+target", deps[1].Target, "Second dependency target mismatch")
	assert.True(t, deps[1].Local, "Second dependency should be local")

	// Check COPY with artifact path
	assert.Equal(t, "github.com/org/repo+build", deps[2].Target, "Third dependency target mismatch")
	assert.False(t, deps[2].Local, "Third dependency should be remote (extracted from COPY)")
}
