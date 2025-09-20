package earthfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEarthfile_Target(t *testing.T) {
	ef := &Earthfile{
		targets: map[string]*Target{
			"build": {Name: "build"},
			"test":  {Name: "test"},
		},
	}

	tests := []struct {
		name       string
		targetName string
		want       *Target
	}{
		{
			name:       "existing target",
			targetName: "build",
			want:       ef.targets["build"],
		},
		{
			name:       "non-existing target",
			targetName: "deploy",
			want:       nil,
		},
		{
			name:       "empty string",
			targetName: "",
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ef.Target(tt.targetName)
			assert.Equal(t, tt.want, got, "Target() mismatch")
		})
	}
}

func TestEarthfile_Targets(t *testing.T) {
	target1 := &Target{Name: "build"}
	target2 := &Target{Name: "test"}

	ef := &Earthfile{
		targets: map[string]*Target{
			"build": target1,
			"test":  target2,
		},
	}

	targets := ef.Targets()
	assert.Len(t, targets, 2, "Targets() should return exactly 2 targets")

	// Check that both targets are in the result
	targetNames := make([]string, len(targets))
	for i, target := range targets {
		targetNames[i] = target.Name
	}

	assert.Contains(t, targetNames, "build", "Targets() should contain 'build' target")
	assert.Contains(t, targetNames, "test", "Targets() should contain 'test' target")
}

func TestEarthfile_TargetNames(t *testing.T) {
	ef := &Earthfile{
		targets: map[string]*Target{
			"build": {Name: "build"},
			"test":  {Name: "test"},
			"lint":  {Name: "lint"},
		},
	}

	names := ef.TargetNames()
	assert.Len(t, names, 3, "TargetNames() should return exactly 3 names")

	// Check that all expected names are present
	expected := []string{"build", "test", "lint"}
	for _, exp := range expected {
		assert.Contains(t, names, exp, "TargetNames() should contain '%s'", exp)
	}
}

func TestEarthfile_HasTarget(t *testing.T) {
	ef := &Earthfile{
		targets: map[string]*Target{
			"build": {Name: "build"},
			"test":  {Name: "test"},
		},
	}

	tests := []struct {
		name       string
		targetName string
		want       bool
	}{
		{
			name:       "existing target",
			targetName: "build",
			want:       true,
		},
		{
			name:       "non-existing target",
			targetName: "deploy",
			want:       false,
		},
		{
			name:       "empty string",
			targetName: "",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ef.HasTarget(tt.targetName)
			assert.Equal(t, tt.want, got, "HasTarget() mismatch")
		})
	}
}

func TestEarthfile_Function(t *testing.T) {
	func1 := &Function{Name: "helper"}
	func2 := &Function{Name: "common"}

	ef := &Earthfile{
		functions: map[string]*Function{
			"helper": func1,
			"common": func2,
		},
	}

	tests := []struct {
		name     string
		funcName string
		want     *Function
	}{
		{
			name:     "existing function",
			funcName: "helper",
			want:     func1,
		},
		{
			name:     "non-existing function",
			funcName: "missing",
			want:     nil,
		},
		{
			name:     "empty string",
			funcName: "",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ef.Function(tt.funcName)
			assert.Equal(t, tt.want, got, "Function() mismatch")
		})
	}
}

func TestEarthfile_Functions(t *testing.T) {
	func1 := &Function{Name: "helper"}
	func2 := &Function{Name: "common"}

	ef := &Earthfile{
		functions: map[string]*Function{
			"helper": func1,
			"common": func2,
		},
	}

	functions := ef.Functions()
	assert.Len(t, functions, 2, "Functions() should return exactly 2 functions")

	// Check that both functions are in the result
	functionNames := make([]string, len(functions))
	for i, fn := range functions {
		functionNames[i] = fn.Name
	}

	assert.Contains(t, functionNames, "helper", "Functions() should contain 'helper' function")
	assert.Contains(t, functionNames, "common", "Functions() should contain 'common' function")
}

func TestEarthfile_BaseCommands_Already_Defined(t *testing.T) {
	// BaseCommands is already implemented in types.go
	cmd1 := &Command{Name: "ARG", Type: CommandTypeArg}
	cmd2 := &Command{Name: "FROM", Type: CommandTypeFrom}

	ef := &Earthfile{
		baseCommands: []*Command{cmd1, cmd2},
	}

	commands := ef.BaseCommands()
	assert.Len(t, commands, 2, "BaseCommands() should return exactly 2 commands")
	assert.Equal(t, cmd1, commands[0], "BaseCommands()[0] should be cmd1")
	assert.Equal(t, cmd2, commands[1], "BaseCommands()[1] should be cmd2")
}

func TestEarthfile_Version_Already_Defined(t *testing.T) {
	// Version() and HasVersion() are already implemented in types.go
	tests := []struct {
		name    string
		version string
		want    string
		hasVer  bool
	}{
		{
			name:    "with version",
			version: "0.7",
			want:    "0.7",
			hasVer:  true,
		},
		{
			name:    "empty version",
			version: "",
			want:    "",
			hasVer:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ef := &Earthfile{version: tt.version}

			got := ef.Version()
			assert.Equal(t, tt.want, got, "Version() mismatch")

			gotHasVer := ef.HasVersion()
			assert.Equal(t, tt.hasVer, gotHasVer, "HasVersion() mismatch")
		})
	}
}

func TestEarthfile_AST_Already_Defined(t *testing.T) {
	// AST() is already implemented in types.go
	// This test verifies it returns the stored AST
	ef := &Earthfile{
		ast: nil, // We'll use nil for simplicity
	}

	got := ef.AST()
	assert.Nil(t, got, "AST() should return nil")
}

func TestEarthfile_Dependencies_Already_Defined(t *testing.T) {
	// Dependencies() now implements lazy loading from AST
	// Test that pre-populated dependencies are returned correctly
	deps := []Dependency{
		{Target: "./lib+build", Local: true, Source: "build"},
		{Target: "github.com/example/repo+test", Local: false, Source: "test"},
	}

	ef := &Earthfile{
		dependencies:       deps,
		dependenciesLoaded: true, // Mark as already loaded to bypass parsing
	}

	got := ef.Dependencies()
	assert.Len(t, got, 2, "Dependencies() should return exactly 2 dependencies")
	assert.Equal(t, deps, got, "Dependencies() should return the exact dependencies")
}
