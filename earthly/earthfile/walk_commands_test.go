package earthfile

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalkCommands(t *testing.T) {
	t.Run("earthfile walk commands visits all commands", func(t *testing.T) {
		content := `
VERSION 0.6

ARG GLOBAL=value

deps:
    FROM alpine
    RUN echo "deps"

build:
    FROM +deps
    COPY src/ /app/
    RUN go build
    SAVE ARTIFACT /app/binary
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		var visited []string
		err = ef.WalkCommands(func(cmd *Command, depth int) error {
			visited = append(visited, cmd.Name)
			return nil
		})
		require.NoError(t, err)

		// Should visit: ARG (base), FROM, RUN (deps), FROM, COPY, RUN, SAVE ARTIFACT (build)
		expected := []string{"ARG", "FROM", "RUN", "FROM", "COPY", "RUN", "SAVE ARTIFACT"}
		assert.Equal(t, expected, visited)
	})

	t.Run("earthfile walk commands tracks depth", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
    IF [ -f config ]
        COPY config /etc/
        FOR i IN 1 2 3
            RUN echo $i
        END
    END
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		depths := make(map[string]int)
		err = ef.WalkCommands(func(cmd *Command, depth int) error {
			if _, exists := depths[cmd.Name]; !exists {
				depths[cmd.Name] = depth
			}
			return nil
		})
		require.NoError(t, err)

		// FROM should be at depth 0, COPY at depth 1 (inside IF), RUN at depth 2 (inside IF/FOR)
		assert.Equal(t, 0, depths["FROM"])
		assert.Equal(t, 0, depths["IF"])
		assert.Equal(t, 1, depths["COPY"])
		assert.Equal(t, 1, depths["FOR"])
		assert.Equal(t, 2, depths["RUN"])
	})

	t.Run("target walk commands visits commands in order", func(t *testing.T) {
		content := `
VERSION 0.6

test:
    FROM golang:1.21
    WORKDIR /app
    COPY . .
    RUN go test ./...
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		target := ef.Target("test")
		require.NotNil(t, target)

		var visited []string
		err = target.WalkCommands(func(cmd *Command, depth int) error {
			visited = append(visited, cmd.Name)
			return nil
		})
		require.NoError(t, err)

		expected := []string{"FROM", "WORKDIR", "COPY", "RUN"}
		assert.Equal(t, expected, visited)
	})

	t.Run("walk commands can terminate early", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
    RUN step1
    RUN step2
    RUN step3
    RUN step4
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		stopErr := errors.New("stop at step2")
		var visited []string

		err = ef.WalkCommands(func(cmd *Command, depth int) error {
			if cmd.Name == "RUN" && len(cmd.Args) > 0 && cmd.Args[0] == "step2" {
				return stopErr
			}
			visited = append(visited, cmd.Name)
			return nil
		})

		assert.Equal(t, stopErr, err)
		// Should have visited FROM and RUN step1, but not step2 or beyond
		assert.Equal(t, []string{"FROM", "RUN"}, visited)
	})

	t.Run("walk commands handles nested structures", func(t *testing.T) {
		content := `
VERSION 0.6

complex:
    FROM alpine
    IF [ "$BUILD_TYPE" = "debug" ]
        RUN echo "Debug build"
        WITH DOCKER
            RUN docker build .
        END
    ELSE
        RUN echo "Release build"
    END
    RUN echo "Done"
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		var commands []struct {
			name  string
			depth int
		}

		err = ef.WalkCommands(func(cmd *Command, depth int) error {
			commands = append(commands, struct {
				name  string
				depth int
			}{cmd.Name, depth})
			return nil
		})
		require.NoError(t, err)

		// Check that we have the right commands at the right depths
		assert.Greater(t, len(commands), 5)

		// FROM should be at depth 0
		assert.Equal(t, "FROM", commands[0].name)
		assert.Equal(t, 0, commands[0].depth)

		// Final RUN "Done" should be at depth 0
		lastCmd := commands[len(commands)-1]
		assert.Equal(t, "RUN", lastCmd.name)
		assert.Equal(t, 0, lastCmd.depth)
	})

	t.Run("walk commands visits function commands", func(t *testing.T) {
		content := `
VERSION 0.6

MY_FUNC:
    FUNCTION
    ARG msg
    RUN echo "$msg"

build:
    FROM alpine
    DO +MY_FUNC --msg="Hello"
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		var visited []string
		err = ef.WalkCommands(func(cmd *Command, depth int) error {
			visited = append(visited, cmd.Name)
			return nil
		})
		require.NoError(t, err)

		// Should include both function and target commands
		assert.Contains(t, visited, "FUNCTION")
		assert.Contains(t, visited, "ARG")
		assert.Contains(t, visited, "FROM")
		assert.Contains(t, visited, "DO")
	})

	t.Run("walk commands with nil callback returns error", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		err = ef.WalkCommands(nil)
		assert.Error(t, err)
	})

	t.Run("walk commands on empty earthfile", func(t *testing.T) {
		content := `VERSION 0.6`

		ef, err := ParseString(content)
		require.NoError(t, err)

		count := 0
		err = ef.WalkCommands(func(cmd *Command, depth int) error {
			count++
			return nil
		})
		require.NoError(t, err)

		// Should have no commands to visit (VERSION is not a command in our model)
		assert.Equal(t, 0, count)
	})
}
