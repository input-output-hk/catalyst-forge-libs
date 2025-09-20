package earthfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTarget_FindCommands(t *testing.T) {
	target := &Target{
		Name: "build",
		Commands: []*Command{
			{Name: "FROM", Type: CommandTypeFrom},
			{Name: "RUN", Type: CommandTypeRun},
			{Name: "COPY", Type: CommandTypeCopy},
			{Name: "RUN", Type: CommandTypeRun},
			{Name: "SAVE ARTIFACT", Type: CommandTypeSaveArtifact},
		},
	}

	tests := []struct {
		name        string
		commandType CommandType
		wantCount   int
	}{
		{
			name:        "find RUN commands",
			commandType: CommandTypeRun,
			wantCount:   2,
		},
		{
			name:        "find FROM commands",
			commandType: CommandTypeFrom,
			wantCount:   1,
		},
		{
			name:        "find non-existing command type",
			commandType: CommandTypeBuild,
			wantCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands := target.FindCommands(tt.commandType)
			assert.Len(t, commands, tt.wantCount, "FindCommands() returned wrong number of commands")

			// Verify all returned commands have the correct type
			for _, cmd := range commands {
				assert.Equal(t, tt.commandType, cmd.Type, "FindCommands() returned command with wrong type")
			}
		})
	}
}

func TestTarget_GetFromBase(t *testing.T) {
	fromCmd := &Command{Name: "FROM", Type: CommandTypeFrom}
	target := &Target{
		Name: "build",
		Commands: []*Command{
			fromCmd,
			{Name: "RUN", Type: CommandTypeRun},
			{Name: "COPY", Type: CommandTypeCopy},
		},
	}

	got := target.GetFromBase()
	assert.Equal(t, fromCmd, got, "GetFromBase() returned wrong command")

	// Test with no FROM command
	targetNoFrom := &Target{
		Name: "build",
		Commands: []*Command{
			{Name: "RUN", Type: CommandTypeRun},
			{Name: "COPY", Type: CommandTypeCopy},
		},
	}

	got = targetNoFrom.GetFromBase()
	assert.Nil(t, got, "GetFromBase() should return nil when no FROM command exists")
}

func TestTarget_GetArgs(t *testing.T) {
	arg1 := &Command{Name: "ARG", Type: CommandTypeArg}
	arg2 := &Command{Name: "ARG", Type: CommandTypeArg}

	target := &Target{
		Name: "build",
		Commands: []*Command{
			{Name: "FROM", Type: CommandTypeFrom},
			arg1,
			{Name: "RUN", Type: CommandTypeRun},
			arg2,
			{Name: "COPY", Type: CommandTypeCopy},
		},
	}

	args := target.GetArgs()
	assert.Len(t, args, 2, "GetArgs() returned wrong number of commands")
	assert.Equal(t, arg1, args[0], "GetArgs()[0] returned wrong command")
	assert.Equal(t, arg2, args[1], "GetArgs()[1] returned wrong command")
}

func TestTarget_GetBuilds(t *testing.T) {
	build1 := &Command{Name: "BUILD", Type: CommandTypeBuild}
	build2 := &Command{Name: "BUILD", Type: CommandTypeBuild}

	target := &Target{
		Name: "build",
		Commands: []*Command{
			{Name: "FROM", Type: CommandTypeFrom},
			build1,
			{Name: "RUN", Type: CommandTypeRun},
			build2,
		},
	}

	builds := target.GetBuilds()
	assert.Len(t, builds, 2, "GetBuilds() returned wrong number of commands")
	assert.Equal(t, build1, builds[0], "GetBuilds()[0] returned wrong command")
	assert.Equal(t, build2, builds[1], "GetBuilds()[1] returned wrong command")
}

func TestTarget_GetArtifacts(t *testing.T) {
	artifact1 := &Command{Name: "SAVE ARTIFACT", Type: CommandTypeSaveArtifact}
	artifact2 := &Command{Name: "SAVE ARTIFACT", Type: CommandTypeSaveArtifact}

	target := &Target{
		Name: "build",
		Commands: []*Command{
			{Name: "FROM", Type: CommandTypeFrom},
			artifact1,
			{Name: "RUN", Type: CommandTypeRun},
			artifact2,
		},
	}

	artifacts := target.GetArtifacts()
	assert.Len(t, artifacts, 2, "GetArtifacts() returned wrong number of commands")
	assert.Equal(t, artifact1, artifacts[0], "GetArtifacts()[0] returned wrong command")
	assert.Equal(t, artifact2, artifacts[1], "GetArtifacts()[1] returned wrong command")
}

func TestTarget_GetImages(t *testing.T) {
	image1 := &Command{Name: "SAVE IMAGE", Type: CommandTypeSaveImage}
	image2 := &Command{Name: "SAVE IMAGE", Type: CommandTypeSaveImage}

	target := &Target{
		Name: "build",
		Commands: []*Command{
			{Name: "FROM", Type: CommandTypeFrom},
			image1,
			{Name: "RUN", Type: CommandTypeRun},
			image2,
		},
	}

	images := target.GetImages()
	assert.Len(t, images, 2, "GetImages() returned wrong number of commands")
	assert.Equal(t, image1, images[0], "GetImages()[0] returned wrong command")
	assert.Equal(t, image2, images[1], "GetImages()[1] returned wrong command")
}

func TestTarget_HasCommand(t *testing.T) {
	target := &Target{
		Name: "build",
		Commands: []*Command{
			{Name: "FROM", Type: CommandTypeFrom},
			{Name: "RUN", Type: CommandTypeRun},
			{Name: "COPY", Type: CommandTypeCopy},
			{Name: "RUN", Type: CommandTypeRun},
			{Name: "SAVE ARTIFACT", Type: CommandTypeSaveArtifact},
		},
	}

	tests := []struct {
		name        string
		commandType CommandType
		want        bool
	}{
		{
			name:        "has FROM command",
			commandType: CommandTypeFrom,
			want:        true,
		},
		{
			name:        "has RUN command",
			commandType: CommandTypeRun,
			want:        true,
		},
		{
			name:        "does not have BUILD command",
			commandType: CommandTypeBuild,
			want:        false,
		},
		{
			name:        "does not have SAVE IMAGE command",
			commandType: CommandTypeSaveImage,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := target.HasCommand(tt.commandType)
			assert.Equal(t, tt.want, got, "HasCommand() returned wrong result")
		})
	}
}

func TestTarget_Walk(t *testing.T) {
	target := &Target{
		Name: "build",
		Commands: []*Command{
			{Name: "FROM", Type: CommandTypeFrom},
			{Name: "RUN", Type: CommandTypeRun},
			{Name: "COPY", Type: CommandTypeCopy},
		},
	}

	var visited []*Command
	var depths []int

	err := target.WalkCommands(func(cmd *Command, depth int) error {
		visited = append(visited, cmd)
		depths = append(depths, depth)
		return nil
	})
	require.NoError(t, err, "Walk() should not return error")

	assert.Len(t, visited, 3, "Walk() visited wrong number of commands")

	// Verify commands are visited in order
	for i, cmd := range target.Commands {
		assert.Equal(t, cmd, visited[i], "Walk() visited wrong command at index %d", i)
		assert.Equal(t, 0, depths[i], "Walk() depth should be 0 at index %d", i)
	}
}

func TestTarget_Walk_EarlyTermination(t *testing.T) {
	target := &Target{
		Name: "build",
		Commands: []*Command{
			{Name: "FROM", Type: CommandTypeFrom},
			{Name: "RUN", Type: CommandTypeRun},
			{Name: "COPY", Type: CommandTypeCopy},
		},
	}

	var visited []*Command
	stopErr := errStopWalk

	err := target.WalkCommands(func(cmd *Command, depth int) error {
		visited = append(visited, cmd)
		if cmd.Type == CommandTypeRun {
			return stopErr
		}
		return nil
	})

	assert.Equal(
		t,
		stopErr,
		err,
		"Walk() should return early termination error",
	)

	assert.Len(t, visited, 2, "Walk() should visit 2 commands before early termination")
}

// Define an error for testing early termination
var errStopWalk = &walkError{msg: "stop walking"}

type walkError struct {
	msg string
}

func (e *walkError) Error() string {
	return e.msg
}
