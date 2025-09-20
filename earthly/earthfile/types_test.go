package earthfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEarthfileStruct(t *testing.T) {
	ef := NewEarthfile()

	assert.NotNil(t, ef.targets, "Earthfile.targets map should be initialized")
	assert.NotNil(t, ef.functions, "Earthfile.functions map should be initialized")
	assert.Equal(t, "", ef.Version(), "Earthfile.Version() should return empty string when not set")
	assert.False(t, ef.HasVersion(), "Earthfile.HasVersion() should return false when version not set")
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

	assert.Equal(t, "build", target.Name, "target name should match")
	assert.Equal(t, "Build target", target.Docs, "target docs should match")
	assert.Len(t, target.Commands, 1, "target should have 1 command")
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

	assert.Equal(t, "MY_FUNC", fn.Name, "function name should match")
	assert.Len(t, fn.Commands, 1, "function should have 1 command")
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

	assert.Equal(t, "COPY", cmd.Name, "command name should match")
	assert.Equal(t, CommandTypeCopy, cmd.Type, "command type should match")
	assert.Len(t, cmd.Args, 2, "command should have 2 args")
	assert.NotNil(t, cmd.Location, "source location should be set")
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
			assert.Equal(t, tt.name, tt.cmdType.String(), "CommandType.String() should match expected name")
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

	assert.Equal(t, "Earthfile", loc.File, "source location file should match")
	assert.Equal(t, 10, loc.StartLine, "source location start line should match")
}
