package earthfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommand_GetFlag(t *testing.T) {
	tests := []struct {
		name     string
		command  *Command
		flagName string
		want     string
		wantOk   bool
	}{
		{
			name: "flag exists",
			command: &Command{
				Name: "RUN",
				Args: []string{"--mount=type=cache,target=/cache", "echo", "hello"},
			},
			flagName: "mount",
			want:     "type=cache,target=/cache",
			wantOk:   true,
		},
		{
			name: "flag does not exist",
			command: &Command{
				Name: "RUN",
				Args: []string{"echo", "hello"},
			},
			flagName: "mount",
			want:     "",
			wantOk:   false,
		},
		{
			name: "flag with equals sign",
			command: &Command{
				Name: "COPY",
				Args: []string{"--from=builder", "src/", "dest/"},
			},
			flagName: "from",
			want:     "builder",
			wantOk:   true,
		},
		{
			name: "flag without value",
			command: &Command{
				Name: "RUN",
				Args: []string{"--verbose", "echo", "hello"},
			},
			flagName: "verbose",
			want:     "",
			wantOk:   true,
		},
		{
			name: "multiple flags",
			command: &Command{
				Name: "BUILD",
				Args: []string{"--platform=linux/amd64", "--push", "./target"},
			},
			flagName: "platform",
			want:     "linux/amd64",
			wantOk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.command.GetFlag(tt.flagName)
			assert.Equal(t, tt.want, got, "GetFlag() value mismatch")
			assert.Equal(t, tt.wantOk, ok, "GetFlag() ok mismatch")
		})
	}
}

func TestCommand_GetPositionalArgs(t *testing.T) {
	tests := []struct {
		name    string
		command *Command
		want    []string
	}{
		{
			name: "no flags",
			command: &Command{
				Name: "RUN",
				Args: []string{"echo", "hello", "world"},
			},
			want: []string{"echo", "hello", "world"},
		},
		{
			name: "with flags",
			command: &Command{
				Name: "RUN",
				Args: []string{"--mount=type=cache", "echo", "hello"},
			},
			want: []string{"echo", "hello"},
		},
		{
			name: "multiple flags",
			command: &Command{
				Name: "BUILD",
				Args: []string{"--platform=linux/amd64", "--push", "./target"},
			},
			want: []string{"./target"},
		},
		{
			name: "empty args",
			command: &Command{
				Name: "FROM",
				Args: []string{},
			},
			want: nil,
		},
		{
			name: "only flags",
			command: &Command{
				Name: "SAVE IMAGE",
				Args: []string{"--push", "--cache-from=type=registry"},
			},
			want: nil,
		},
		{
			name: "mixed flags and args",
			command: &Command{
				Name: "COPY",
				Args: []string{"--from=builder", "--chown=1000:1000", "src/", "dest/"},
			},
			want: []string{"src/", "dest/"},
		},
		{
			name: "double dash separator",
			command: &Command{
				Name: "RUN",
				Args: []string{"--mount=cache", "--", "--flag-like-arg", "value"},
			},
			want: []string{"--flag-like-arg", "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.command.GetPositionalArgs()
			assert.Equal(t, tt.want, got, "GetPositionalArgs() mismatch")
		})
	}
}

func TestCommand_IsRemoteReference(t *testing.T) {
	tests := []struct {
		name    string
		command *Command
		want    bool
	}{
		{
			name: "local reference",
			command: &Command{
				Name: "BUILD",
				Args: []string{"./lib:build"},
			},
			want: false,
		},
		{
			name: "remote reference",
			command: &Command{
				Name: "FROM",
				Args: []string{"github.com/example/repo:image"},
			},
			want: true,
		},
		{
			name: "local relative reference",
			command: &Command{
				Name: "COPY",
				Args: []string{"../other:target+artifact/*", "."},
			},
			want: false,
		},
		{
			name: "remote with protocol",
			command: &Command{
				Name: "FROM",
				Args: []string{"https://github.com/example/repo:base"},
			},
			want: true,
		},
		{
			name: "no reference",
			command: &Command{
				Name: "RUN",
				Args: []string{"echo", "hello"},
			},
			want: false,
		},
		{
			name: "docker image",
			command: &Command{
				Name: "FROM",
				Args: []string{"alpine:3.18"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.command.IsRemoteReference()
			assert.Equal(t, tt.want, got, "IsRemoteReference() mismatch")
		})
	}
}

func compareReference(t *testing.T, got, want *Reference) {
	t.Helper()
	if got == nil && want != nil {
		t.Errorf("GetReference() = nil, want %+v", want)
		return
	}
	if got != nil && want == nil {
		t.Errorf("GetReference() = %+v, want nil", got)
		return
	}
	if got != nil && want != nil {
		assert.Equal(t, want.Target, got.Target, "GetReference().Target mismatch")
		assert.Equal(t, want.Local, got.Local, "GetReference().Local mismatch")
		assert.Equal(t, want.Remote, got.Remote, "GetReference().Remote mismatch")
		assert.Equal(t, want.Path, got.Path, "GetReference().Path mismatch")
	}
}

func TestCommand_GetReference(t *testing.T) {
	tests := []struct {
		name    string
		command *Command
		want    *Reference
		wantErr bool
	}{
		{
			name: "local BUILD reference",
			command: &Command{
				Name: "BUILD",
				Args: []string{"./lib:build"},
			},
			want: &Reference{
				Target: "build",
				Local:  true,
				Remote: false,
				Path:   "./lib",
			},
			wantErr: false,
		},
		{
			name: "remote FROM reference",
			command: &Command{
				Name: "FROM",
				Args: []string{"github.com/example/repo:base"},
			},
			want: &Reference{
				Target: "base",
				Local:  false,
				Remote: true,
				Path:   "github.com/example/repo",
			},
			wantErr: false,
		},
		{
			name: "local COPY with artifact",
			command: &Command{
				Name: "COPY",
				Args: []string{"../other:target+artifact/file.txt", "."},
			},
			want: &Reference{
				Target: "target",
				Local:  true,
				Remote: false,
				Path:   "../other",
			},
			wantErr: false,
		},
		{
			name: "no reference",
			command: &Command{
				Name: "RUN",
				Args: []string{"echo", "hello"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "docker image not a reference",
			command: &Command{
				Name: "FROM",
				Args: []string{"alpine:3.18"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "current directory reference",
			command: &Command{
				Name: "BUILD",
				Args: []string{"+test"},
			},
			want: &Reference{
				Target: "test",
				Local:  true,
				Remote: false,
				Path:   ".",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.command.GetReference()
			if tt.wantErr {
				require.Error(t, err, "GetReference() should return an error")
				return
			}

			require.NoError(t, err, "GetReference() should not return an error")
			compareReference(t, got, tt.want)
		})
	}
}

func TestCommand_SourceLocation(t *testing.T) {
	loc := &SourceLocation{
		File:        "Earthfile",
		StartLine:   10,
		StartColumn: 0,
		EndLine:     10,
		EndColumn:   20,
	}

	cmd := &Command{
		Name:     "RUN",
		Location: loc,
	}

	got := cmd.SourceLocation()
	assert.Equal(t, loc, got, "SourceLocation() with location should return the location")

	// Test with nil location
	cmdNoLoc := &Command{
		Name:     "RUN",
		Location: nil,
	}

	gotNil := cmdNoLoc.SourceLocation()
	assert.Nil(t, gotNil, "SourceLocation() with nil location should return nil")
}
