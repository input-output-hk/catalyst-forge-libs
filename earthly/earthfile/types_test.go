package earthfile

import (
	"testing"
)

func TestEarthfileStruct(t *testing.T) {
	ef := NewEarthfile()

	if ef.targets == nil {
		t.Error("Earthfile.targets map should be initialized")
	}

	if ef.functions == nil {
		t.Error("Earthfile.functions map should be initialized")
	}

	if ef.Version() != "" {
		t.Error("Earthfile.Version() should return empty string when not set")
	}

	if ef.HasVersion() {
		t.Error("Earthfile.HasVersion() should return false when version not set")
	}
}

func TestTargetStruct(t *testing.T) {
	target := &Target{
		Name: "build",
		Docs: "Build target",
		Commands: []*Command{
			{
				Name: "RUN",
				Type: CommandTypeRun,
				Args: []string{"echo hello"},
			},
		},
	}

	if target.Name != "build" {
		t.Errorf("Expected target name 'build', got '%s'", target.Name)
	}

	if target.Docs != "Build target" {
		t.Errorf("Expected docs 'Build target', got '%s'", target.Docs)
	}

	if len(target.Commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(target.Commands))
	}
}

func TestFunctionStruct(t *testing.T) {
	fn := &Function{
		Name: "MY_FUNC",
		Commands: []*Command{
			{
				Name: "ARG",
				Type: CommandTypeArg,
				Args: []string{"param"},
			},
		},
	}

	if fn.Name != "MY_FUNC" {
		t.Errorf("Expected function name 'MY_FUNC', got '%s'", fn.Name)
	}

	if len(fn.Commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(fn.Commands))
	}
}

func TestCommandStruct(t *testing.T) {
	cmd := &Command{
		Name: "COPY",
		Type: CommandTypeCopy,
		Args: []string{"src/", "dest/"},
		Location: &SourceLocation{
			File:        "Earthfile",
			StartLine:   10,
			StartColumn: 0,
			EndLine:     10,
			EndColumn:   20,
		},
	}

	if cmd.Name != "COPY" {
		t.Errorf("Expected command name 'COPY', got '%s'", cmd.Name)
	}

	if cmd.Type != CommandTypeCopy {
		t.Errorf("Expected command type CommandTypeCopy, got %v", cmd.Type)
	}

	if len(cmd.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(cmd.Args))
	}

	if cmd.Location == nil {
		t.Error("Expected source location to be set")
	}
}

func TestCommandType(t *testing.T) {
	tests := []struct {
		cmdType CommandType
		name    string
	}{
		{CommandTypeFrom, "FROM"},
		{CommandTypeRun, "RUN"},
		{CommandTypeCopy, "COPY"},
		{CommandTypeBuild, "BUILD"},
		{CommandTypeArg, "ARG"},
		{CommandTypeSaveArtifact, "SAVE ARTIFACT"},
		{CommandTypeSaveImage, "SAVE IMAGE"},
		{CommandTypeCmd, "CMD"},
		{CommandTypeEntrypoint, "ENTRYPOINT"},
		{CommandTypeExpose, "EXPOSE"},
		{CommandTypeVolume, "VOLUME"},
		{CommandTypeEnv, "ENV"},
		{CommandTypeWorkdir, "WORKDIR"},
		{CommandTypeUser, "USER"},
		{CommandTypeGitClone, "GIT CLONE"},
		{CommandTypeAdd, "ADD"},
		{CommandTypeStopsignal, "STOPSIGNAL"},
		{CommandTypeOnbuild, "ONBUILD"},
		{CommandTypeHealthcheck, "HEALTHCHECK"},
		{CommandTypeShell, "SHELL"},
		{CommandTypeDo, "DO"},
		{CommandTypeCommand, "COMMAND"},
		{CommandTypeImport, "IMPORT"},
		{CommandTypeVersion, "VERSION"},
		{CommandTypeFromDockerfile, "FROM DOCKERFILE"},
		{CommandTypeLocally, "LOCALLY"},
		{CommandTypeHost, "HOST"},
		{CommandTypeProject, "PROJECT"},
		{CommandTypeCache, "CACHE"},
		{CommandTypeSet, "SET"},
		{CommandTypeLet, "LET"},
		{CommandTypeTry, "TRY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cmdType.String() != tt.name {
				t.Errorf("CommandType.String() = %v, want %v", tt.cmdType.String(), tt.name)
			}
		})
	}
}

func TestSourceLocation(t *testing.T) {
	loc := &SourceLocation{
		File:        "Earthfile",
		StartLine:   10,
		StartColumn: 5,
		EndLine:     12,
		EndColumn:   15,
	}

	if loc.File != "Earthfile" {
		t.Errorf("Expected file 'Earthfile', got '%s'", loc.File)
	}

	if loc.StartLine != 10 {
		t.Errorf("Expected start line 10, got %d", loc.StartLine)
	}
}
