package earthfile

import (
	"testing"
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
			if got != tt.want {
				t.Errorf("Target() = %v, want %v", got, tt.want)
			}
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
	if len(targets) != 2 {
		t.Errorf("Targets() returned %d targets, want 2", len(targets))
	}

	// Check that both targets are in the result
	found := make(map[string]bool)
	for _, target := range targets {
		found[target.Name] = true
	}

	if !found["build"] {
		t.Error("Targets() missing 'build' target")
	}
	if !found["test"] {
		t.Error("Targets() missing 'test' target")
	}
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
	if len(names) != 3 {
		t.Errorf("TargetNames() returned %d names, want 3", len(names))
	}

	// Check that all names are present
	found := make(map[string]bool)
	for _, name := range names {
		found[name] = true
	}

	expected := []string{"build", "test", "lint"}
	for _, exp := range expected {
		if !found[exp] {
			t.Errorf("TargetNames() missing '%s'", exp)
		}
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
			if got != tt.want {
				t.Errorf("HasTarget() = %v, want %v", got, tt.want)
			}
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
			if got != tt.want {
				t.Errorf("Function() = %v, want %v", got, tt.want)
			}
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
	if len(functions) != 2 {
		t.Errorf("Functions() returned %d functions, want 2", len(functions))
	}

	// Check that both functions are in the result
	found := make(map[string]bool)
	for _, fn := range functions {
		found[fn.Name] = true
	}

	if !found["helper"] {
		t.Error("Functions() missing 'helper' function")
	}
	if !found["common"] {
		t.Error("Functions() missing 'common' function")
	}
}

func TestEarthfile_BaseCommands_Already_Defined(t *testing.T) {
	// BaseCommands is already implemented in types.go
	cmd1 := &Command{Name: "ARG", Type: CommandTypeArg}
	cmd2 := &Command{Name: "FROM", Type: CommandTypeFrom}

	ef := &Earthfile{
		baseCommands: []*Command{cmd1, cmd2},
	}

	commands := ef.BaseCommands()
	if len(commands) != 2 {
		t.Errorf("BaseCommands() returned %d commands, want 2", len(commands))
	}
	if commands[0] != cmd1 {
		t.Errorf("BaseCommands()[0] = %v, want %v", commands[0], cmd1)
	}
	if commands[1] != cmd2 {
		t.Errorf("BaseCommands()[1] = %v, want %v", commands[1], cmd2)
	}
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

			if got := ef.Version(); got != tt.want {
				t.Errorf("Version() = %v, want %v", got, tt.want)
			}

			if got := ef.HasVersion(); got != tt.hasVer {
				t.Errorf("HasVersion() = %v, want %v", got, tt.hasVer)
			}
		})
	}
}

func TestEarthfile_AST_Already_Defined(t *testing.T) {
	// AST() is already implemented in types.go
	// This test verifies it returns the stored AST
	ef := &Earthfile{
		ast: nil, // We'll use nil for simplicity
	}

	if got := ef.AST(); got != nil {
		t.Errorf("AST() = %v, want nil", got)
	}
}

func TestEarthfile_Dependencies_Already_Defined(t *testing.T) {
	// Dependencies() is already implemented in types.go
	deps := []Dependency{
		{Target: "./lib:build", Local: true, Source: "build"},
		{Target: "github.com/example/repo:test", Local: false, Source: "test"},
	}

	ef := &Earthfile{
		dependencies: deps,
	}

	got := ef.Dependencies()
	if len(got) != 2 {
		t.Errorf("Dependencies() returned %d dependencies, want 2", len(got))
	}
	for i, dep := range got {
		if dep != deps[i] {
			t.Errorf("Dependencies()[%d] = %v, want %v", i, dep, deps[i])
		}
	}
}
