package earthfile

import (
	"testing"
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
			if len(commands) != tt.wantCount {
				t.Errorf("FindCommands() returned %d commands, want %d", len(commands), tt.wantCount)
			}

			// Verify all returned commands have the correct type
			for _, cmd := range commands {
				if cmd.Type != tt.commandType {
					t.Errorf("FindCommands() returned command with type %v, want %v", cmd.Type, tt.commandType)
				}
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
	if got != fromCmd {
		t.Errorf("GetFromBase() = %v, want %v", got, fromCmd)
	}

	// Test with no FROM command
	targetNoFrom := &Target{
		Name: "build",
		Commands: []*Command{
			{Name: "RUN", Type: CommandTypeRun},
			{Name: "COPY", Type: CommandTypeCopy},
		},
	}

	if got := targetNoFrom.GetFromBase(); got != nil {
		t.Errorf("GetFromBase() = %v, want nil", got)
	}
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
	if len(args) != 2 {
		t.Errorf("GetArgs() returned %d commands, want 2", len(args))
	}
	if args[0] != arg1 {
		t.Errorf("GetArgs()[0] = %v, want %v", args[0], arg1)
	}
	if args[1] != arg2 {
		t.Errorf("GetArgs()[1] = %v, want %v", args[1], arg2)
	}
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
	if len(builds) != 2 {
		t.Errorf("GetBuilds() returned %d commands, want 2", len(builds))
	}
	if builds[0] != build1 {
		t.Errorf("GetBuilds()[0] = %v, want %v", builds[0], build1)
	}
	if builds[1] != build2 {
		t.Errorf("GetBuilds()[1] = %v, want %v", builds[1], build2)
	}
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
	if len(artifacts) != 2 {
		t.Errorf("GetArtifacts() returned %d commands, want 2", len(artifacts))
	}
	if artifacts[0] != artifact1 {
		t.Errorf("GetArtifacts()[0] = %v, want %v", artifacts[0], artifact1)
	}
	if artifacts[1] != artifact2 {
		t.Errorf("GetArtifacts()[1] = %v, want %v", artifacts[1], artifact2)
	}
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
	if len(images) != 2 {
		t.Errorf("GetImages() returned %d commands, want 2", len(images))
	}
	if images[0] != image1 {
		t.Errorf("GetImages()[0] = %v, want %v", images[0], image1)
	}
	if images[1] != image2 {
		t.Errorf("GetImages()[1] = %v, want %v", images[1], image2)
	}
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
			if got != tt.want {
				t.Errorf("HasCommand() = %v, want %v", got, tt.want)
			}
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

	err := target.Walk(func(cmd *Command, depth int) error {
		visited = append(visited, cmd)
		depths = append(depths, depth)
		return nil
	})
	if err != nil {
		t.Errorf("Walk() returned error: %v", err)
	}

	if len(visited) != 3 {
		t.Errorf("Walk() visited %d commands, want 3", len(visited))
	}

	// Verify commands are visited in order
	for i, cmd := range target.Commands {
		if visited[i] != cmd {
			t.Errorf("Walk() visited[%d] = %v, want %v", i, visited[i], cmd)
		}
		if depths[i] != 0 {
			t.Errorf("Walk() depth[%d] = %d, want 0", i, depths[i])
		}
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

	err := target.Walk(func(cmd *Command, depth int) error {
		visited = append(visited, cmd)
		if cmd.Type == CommandTypeRun {
			return stopErr
		}
		return nil
	})

	if err != stopErr { //nolint:errorlint // comparing exact error instance
		t.Errorf("Walk() returned %v, want %v", err, stopErr)
	}

	if len(visited) != 2 {
		t.Errorf("Walk() visited %d commands, want 2 (should stop after RUN)", len(visited))
	}
}

// Define an error for testing early termination
var errStopWalk = &walkError{msg: "stop walking"}

type walkError struct {
	msg string
}

func (e *walkError) Error() string {
	return e.msg
}
