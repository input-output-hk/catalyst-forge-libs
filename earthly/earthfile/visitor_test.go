package earthfile

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisitorInterface(t *testing.T) {
	t.Run("visitor receives all targets", func(t *testing.T) {
		content := `
VERSION 0.6
ARG BASE_IMAGE=alpine

target1:
    FROM alpine
    RUN echo "hello"

target2:
    FROM ubuntu
    COPY . /app
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &targetCollector{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		assert.Len(t, collector.targets, 2)
		assert.Equal(t, "target1", collector.targets[0].Name)
		assert.Equal(t, "target2", collector.targets[1].Name)
	})

	t.Run("visitor receives functions", func(t *testing.T) {
		content := `
VERSION 0.6

MY_FUNCTION:
    FUNCTION
    ARG msg
    RUN echo "$msg"

ANOTHER_FUNC:
    FUNCTION
    RUN ls -la
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &functionCollector{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		assert.Len(t, collector.functions, 2)
		assert.Equal(t, "MY_FUNCTION", collector.functions[0].Name)
		assert.Equal(t, "ANOTHER_FUNC", collector.functions[1].Name)
	})

	t.Run("visitor receives all commands", func(t *testing.T) {
		content := `
VERSION 0.6

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

		collector := &commandCollector{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		// Should have FROM, RUN (deps), FROM, COPY, RUN, SAVE ARTIFACT (build)
		assert.GreaterOrEqual(t, len(collector.commands), 6)

		// Check command names are captured
		names := make([]string, len(collector.commands))
		for i, cmd := range collector.commands {
			names[i] = cmd.Name
		}
		assert.Contains(t, names, "FROM")
		assert.Contains(t, names, "RUN")
		assert.Contains(t, names, "COPY")
		assert.Contains(t, names, "SAVE ARTIFACT")
	})

	t.Run("visitor handles IF statements", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
    ARG ENABLE_DEBUG=false
    IF [ "$ENABLE_DEBUG" = "true" ]
        RUN echo "Debug mode"
    ELSE
        RUN echo "Release mode"
    END
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &ifStatementCollector{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		assert.Len(t, collector.ifStatements, 1)
		assert.Equal(t, []string{"[", "\"$ENABLE_DEBUG\"", "=", "\"true\"", "]"}, collector.ifStatements[0].condition)
		assert.Len(t, collector.ifStatements[0].thenCommands, 1)
		assert.Len(t, collector.ifStatements[0].elseCommands, 1)
	})

	t.Run("visitor handles FOR loops", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
    FOR file IN src/*.go
        COPY $file /app/
    END
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &forStatementCollector{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		assert.Len(t, collector.forStatements, 1)
		assert.Equal(t, []string{"file", "IN", "src/*.go"}, collector.forStatements[0].args)
		assert.Len(t, collector.forStatements[0].bodyCommands, 1)
	})

	t.Run("visitor handles WITH statements", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
    WITH DOCKER
        RUN docker build .
    END
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &withStatementCollector{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		assert.Len(t, collector.withStatements, 1)
		assert.Equal(t, "DOCKER", collector.withStatements[0].command.Name)
		assert.Equal(t, []string{}, collector.withStatements[0].command.Args)
		assert.Len(t, collector.withStatements[0].bodyCommands, 1)
	})

	t.Run("visitor handles TRY statements", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
    TRY
        RUN exit 1
    CATCH
        RUN echo "failed"
    FINALLY
        RUN echo "cleanup"
    END
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &tryStatementCollector{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		assert.Len(t, collector.tryStatements, 1)
		assert.Len(t, collector.tryStatements[0].tryCommands, 1)
		assert.Len(t, collector.tryStatements[0].catchCommands, 1)
		assert.Len(t, collector.tryStatements[0].finallyCommands, 1)
	})

	t.Run("visitor handles WAIT statements", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
    BUILD +target1
    BUILD +target2
    WAIT
        BUILD +slow-target
    END
    RUN echo "done"
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &waitStatementCollector{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		assert.Len(t, collector.waitStatements, 1)
		assert.Len(t, collector.waitStatements[0].bodyCommands, 1)
	})

	t.Run("visitor handles nested blocks", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
    FOR i IN 1 2 3
        IF [ "$i" = "2" ]
            WITH DOCKER
                RUN echo "nested: $i"
            END
        END
    END
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &nestedBlockCollector{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		// Should have visited FOR, IF, WITH, and the nested RUN command
		assert.Equal(t, 1, collector.forCount)
		assert.Equal(t, 1, collector.ifCount)
		assert.Equal(t, 1, collector.withCount)
		assert.Equal(t, 1, collector.nestedCommandCount)
	})

	t.Run("visitor can terminate early with error", func(t *testing.T) {
		content := `
VERSION 0.6

target1:
    FROM alpine
    RUN echo "1"

target2:
    FROM ubuntu
    RUN echo "2"

target3:
    FROM debian
    RUN echo "3"
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &earlyTerminationVisitor{stopAt: "target2"}
		err = ef.Walk(collector)
		assert.Error(t, err)
		assert.Equal(t, errStopVisitor, err)

		// Should have visited target1 and target2, but not target3
		assert.Len(t, collector.visitedTargets, 2)
		assert.Equal(t, "target1", collector.visitedTargets[0])
		assert.Equal(t, "target2", collector.visitedTargets[1])
	})

	t.Run("visitor receives base commands", func(t *testing.T) {
		content := `
VERSION 0.6

ARG GLOBAL_ARG=value
FROM alpine AS foundation

target:
    FROM foundation
    RUN echo "hello"
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &baseCommandCollector{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		// Should have ARG and FROM from base recipe (global commands before targets)
		assert.Len(t, collector.baseCommands, 2)
		assert.Equal(t, "ARG", collector.baseCommands[0].Name)
		assert.Equal(t, "FROM", collector.baseCommands[1].Name)
	})

	t.Run("visitor traversal order is correct", func(t *testing.T) {
		content := `
VERSION 0.6

ARG BASE=alpine

target1:
    FROM $BASE
    RUN echo "1"

target2:
    FROM ubuntu
    RUN echo "2"
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		collector := &orderTracker{}
		err = ef.Walk(collector)
		require.NoError(t, err)

		// Should visit: base commands, then targets in order
		expected := []string{
			"command:ARG",
			"target:target1",
			"command:FROM",
			"command:RUN",
			"target:target2",
			"command:FROM",
			"command:RUN",
		}
		assert.Equal(t, expected, collector.visitOrder)
	})
}

func TestTargetWalk(t *testing.T) {
	t.Run("target walk visits all commands", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
    COPY src/ /app/
    RUN make build
    SAVE ARTIFACT /app/binary
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		target := ef.Target("build")
		require.NotNil(t, target)

		collector := &commandCollector{}
		err = target.Walk(collector)
		require.NoError(t, err)

		assert.Len(t, collector.commands, 4)
		assert.Equal(t, "FROM", collector.commands[0].Name)
		assert.Equal(t, "COPY", collector.commands[1].Name)
		assert.Equal(t, "RUN", collector.commands[2].Name)
		assert.Equal(t, "SAVE ARTIFACT", collector.commands[3].Name)
	})

	t.Run("target walk handles nested statements", func(t *testing.T) {
		content := `
VERSION 0.6

build:
    FROM alpine
    IF [ -f "config.json" ]
        COPY config.json /etc/
        FOR env IN dev prod
            RUN echo "Building for $env"
        END
    END
`
		ef, err := ParseString(content)
		require.NoError(t, err)

		target := ef.Target("build")
		require.NotNil(t, target)

		collector := &fullTraversalCollector{}
		err = target.Walk(collector)
		require.NoError(t, err)

		// Should visit FROM, IF, COPY, FOR, RUN
		assert.Equal(t, 1, collector.commandCount["FROM"])
		assert.Equal(t, 1, collector.commandCount["COPY"])
		assert.Equal(t, 1, collector.commandCount["RUN"])
		assert.Equal(t, 1, collector.ifCount)
		assert.Equal(t, 1, collector.forCount)
	})
}

// Test visitor implementations

var errStopVisitor = errors.New("stop visitor walk")

type targetCollector struct {
	targets []*Target
}

func (tc *targetCollector) VisitTarget(t *Target) error {
	tc.targets = append(tc.targets, t)
	return nil
}

func (tc *targetCollector) VisitFunction(f *Function) error {
	return nil
}

func (tc *targetCollector) VisitCommand(c *Command) error {
	return nil
}

func (tc *targetCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	return nil
}

func (tc *targetCollector) VisitForStatement(args []string, body []*Command) error {
	return nil
}

func (tc *targetCollector) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

func (tc *targetCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (tc *targetCollector) VisitWaitStatement(body []*Command) error {
	return nil
}

func (tc *targetCollector) VisitBaseCommand(c *Command) error {
	return nil
}

type functionCollector struct {
	functions []*Function
}

func (fc *functionCollector) VisitTarget(t *Target) error {
	return nil
}

func (fc *functionCollector) VisitFunction(f *Function) error {
	fc.functions = append(fc.functions, f)
	return nil
}

func (fc *functionCollector) VisitCommand(c *Command) error {
	return nil
}

func (fc *functionCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	return nil
}

func (fc *functionCollector) VisitForStatement(args []string, body []*Command) error {
	return nil
}

func (fc *functionCollector) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

func (fc *functionCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (fc *functionCollector) VisitWaitStatement(body []*Command) error {
	return nil
}

func (fc *functionCollector) VisitBaseCommand(c *Command) error {
	return nil
}

type commandCollector struct {
	commands []*Command
}

func (cc *commandCollector) VisitTarget(t *Target) error {
	return nil
}

func (cc *commandCollector) VisitFunction(f *Function) error {
	return nil
}

func (cc *commandCollector) VisitCommand(c *Command) error {
	cc.commands = append(cc.commands, c)
	return nil
}

func (cc *commandCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	return nil
}

func (cc *commandCollector) VisitForStatement(args []string, body []*Command) error {
	return nil
}

func (cc *commandCollector) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

func (cc *commandCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (cc *commandCollector) VisitWaitStatement(body []*Command) error {
	return nil
}

func (cc *commandCollector) VisitBaseCommand(c *Command) error {
	return nil
}

type ifStatementCollector struct {
	ifStatements []struct {
		condition    []string
		thenCommands []*Command
		elseCommands []*Command
	}
}

func (ic *ifStatementCollector) VisitTarget(t *Target) error {
	return nil
}

func (ic *ifStatementCollector) VisitFunction(f *Function) error {
	return nil
}

func (ic *ifStatementCollector) VisitCommand(c *Command) error {
	return nil
}

func (ic *ifStatementCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	ic.ifStatements = append(ic.ifStatements, struct {
		condition    []string
		thenCommands []*Command
		elseCommands []*Command
	}{
		condition:    condition,
		thenCommands: thenBlock,
		elseCommands: elseBlock,
	})
	return nil
}

func (ic *ifStatementCollector) VisitForStatement(args []string, body []*Command) error {
	return nil
}

func (ic *ifStatementCollector) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

func (ic *ifStatementCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (ic *ifStatementCollector) VisitWaitStatement(body []*Command) error {
	return nil
}

func (ic *ifStatementCollector) VisitBaseCommand(c *Command) error {
	return nil
}

type forStatementCollector struct {
	forStatements []struct {
		args         []string
		bodyCommands []*Command
	}
}

func (fc *forStatementCollector) VisitTarget(t *Target) error {
	return nil
}

func (fc *forStatementCollector) VisitFunction(f *Function) error {
	return nil
}

func (fc *forStatementCollector) VisitCommand(c *Command) error {
	return nil
}

func (fc *forStatementCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	return nil
}

func (fc *forStatementCollector) VisitForStatement(args []string, body []*Command) error {
	fc.forStatements = append(fc.forStatements, struct {
		args         []string
		bodyCommands []*Command
	}{
		args:         args,
		bodyCommands: body,
	})
	return nil
}

func (fc *forStatementCollector) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

func (fc *forStatementCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (fc *forStatementCollector) VisitWaitStatement(body []*Command) error {
	return nil
}

func (fc *forStatementCollector) VisitBaseCommand(c *Command) error {
	return nil
}

type withStatementCollector struct {
	withStatements []struct {
		command      *Command
		bodyCommands []*Command
	}
}

func (wc *withStatementCollector) VisitTarget(t *Target) error {
	return nil
}

func (wc *withStatementCollector) VisitFunction(f *Function) error {
	return nil
}

func (wc *withStatementCollector) VisitCommand(c *Command) error {
	return nil
}

func (wc *withStatementCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	return nil
}

func (wc *withStatementCollector) VisitForStatement(args []string, body []*Command) error {
	return nil
}

func (wc *withStatementCollector) VisitWithStatement(command *Command, body []*Command) error {
	wc.withStatements = append(wc.withStatements, struct {
		command      *Command
		bodyCommands []*Command
	}{
		command:      command,
		bodyCommands: body,
	})
	return nil
}

func (wc *withStatementCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (wc *withStatementCollector) VisitWaitStatement(body []*Command) error {
	return nil
}

func (wc *withStatementCollector) VisitBaseCommand(c *Command) error {
	return nil
}

type tryStatementCollector struct {
	tryStatements []struct {
		tryCommands     []*Command
		catchCommands   []*Command
		finallyCommands []*Command
	}
}

func (tc *tryStatementCollector) VisitTarget(t *Target) error {
	return nil
}

func (tc *tryStatementCollector) VisitFunction(f *Function) error {
	return nil
}

func (tc *tryStatementCollector) VisitCommand(c *Command) error {
	return nil
}

func (tc *tryStatementCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	return nil
}

func (tc *tryStatementCollector) VisitForStatement(args []string, body []*Command) error {
	return nil
}

func (tc *tryStatementCollector) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

func (tc *tryStatementCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	tc.tryStatements = append(tc.tryStatements, struct {
		tryCommands     []*Command
		catchCommands   []*Command
		finallyCommands []*Command
	}{
		tryCommands:     tryBlock,
		catchCommands:   catchBlock,
		finallyCommands: finallyBlock,
	})
	return nil
}

func (tc *tryStatementCollector) VisitWaitStatement(body []*Command) error {
	return nil
}

func (tc *tryStatementCollector) VisitBaseCommand(c *Command) error {
	return nil
}

type waitStatementCollector struct {
	waitStatements []struct {
		bodyCommands []*Command
	}
}

func (wc *waitStatementCollector) VisitTarget(t *Target) error {
	return nil
}

func (wc *waitStatementCollector) VisitFunction(f *Function) error {
	return nil
}

func (wc *waitStatementCollector) VisitCommand(c *Command) error {
	return nil
}

func (wc *waitStatementCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	return nil
}

func (wc *waitStatementCollector) VisitForStatement(args []string, body []*Command) error {
	return nil
}

func (wc *waitStatementCollector) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

func (wc *waitStatementCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (wc *waitStatementCollector) VisitWaitStatement(body []*Command) error {
	wc.waitStatements = append(wc.waitStatements, struct {
		bodyCommands []*Command
	}{
		bodyCommands: body,
	})
	return nil
}

func (wc *waitStatementCollector) VisitBaseCommand(c *Command) error {
	return nil
}

type nestedBlockCollector struct {
	forCount           int
	ifCount            int
	withCount          int
	nestedCommandCount int
}

func (nc *nestedBlockCollector) VisitTarget(t *Target) error {
	return nil
}

func (nc *nestedBlockCollector) VisitFunction(f *Function) error {
	return nil
}

func (nc *nestedBlockCollector) VisitCommand(c *Command) error {
	if c.Name == "RUN" && len(c.Args) >= 2 {
		// Check if it's the nested echo command (args are split)
		if c.Args[0] == "echo" && c.Args[1] == "\"nested: $i\"" {
			nc.nestedCommandCount++
		}
	}
	return nil
}

func (nc *nestedBlockCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	nc.ifCount++
	return nil
}

func (nc *nestedBlockCollector) VisitForStatement(args []string, body []*Command) error {
	nc.forCount++
	return nil
}

func (nc *nestedBlockCollector) VisitWithStatement(command *Command, body []*Command) error {
	nc.withCount++
	return nil
}

func (nc *nestedBlockCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (nc *nestedBlockCollector) VisitWaitStatement(body []*Command) error {
	return nil
}

func (nc *nestedBlockCollector) VisitBaseCommand(c *Command) error {
	return nil
}

type earlyTerminationVisitor struct {
	stopAt         string
	visitedTargets []string
}

func (et *earlyTerminationVisitor) VisitTarget(t *Target) error {
	et.visitedTargets = append(et.visitedTargets, t.Name)
	if t.Name == et.stopAt {
		return errStopVisitor
	}
	return nil
}

func (et *earlyTerminationVisitor) VisitFunction(f *Function) error {
	return nil
}

func (et *earlyTerminationVisitor) VisitCommand(c *Command) error {
	return nil
}

func (et *earlyTerminationVisitor) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	return nil
}

func (et *earlyTerminationVisitor) VisitForStatement(args []string, body []*Command) error {
	return nil
}

func (et *earlyTerminationVisitor) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

func (et *earlyTerminationVisitor) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (et *earlyTerminationVisitor) VisitWaitStatement(body []*Command) error {
	return nil
}

func (et *earlyTerminationVisitor) VisitBaseCommand(c *Command) error {
	return nil
}

type baseCommandCollector struct {
	baseCommands []*Command
}

func (bc *baseCommandCollector) VisitTarget(t *Target) error {
	return nil
}

func (bc *baseCommandCollector) VisitFunction(f *Function) error {
	return nil
}

func (bc *baseCommandCollector) VisitCommand(c *Command) error {
	// Collect only base commands (commands not in targets/functions)
	// This is handled by the Walk implementation
	return nil
}

func (bc *baseCommandCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	return nil
}

func (bc *baseCommandCollector) VisitForStatement(args []string, body []*Command) error {
	return nil
}

func (bc *baseCommandCollector) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

func (bc *baseCommandCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (bc *baseCommandCollector) VisitWaitStatement(body []*Command) error {
	return nil
}

func (bc *baseCommandCollector) VisitBaseCommand(c *Command) error {
	bc.baseCommands = append(bc.baseCommands, c)
	return nil
}

type orderTracker struct {
	visitOrder []string
}

func (ot *orderTracker) VisitTarget(t *Target) error {
	ot.visitOrder = append(ot.visitOrder, "target:"+t.Name)
	return nil
}

func (ot *orderTracker) VisitFunction(f *Function) error {
	ot.visitOrder = append(ot.visitOrder, "function:"+f.Name)
	return nil
}

func (ot *orderTracker) VisitCommand(c *Command) error {
	ot.visitOrder = append(ot.visitOrder, "command:"+c.Name)
	return nil
}

func (ot *orderTracker) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	ot.visitOrder = append(ot.visitOrder, "if")
	return nil
}

func (ot *orderTracker) VisitForStatement(args []string, body []*Command) error {
	ot.visitOrder = append(ot.visitOrder, "for")
	return nil
}

func (ot *orderTracker) VisitWithStatement(command *Command, body []*Command) error {
	ot.visitOrder = append(ot.visitOrder, "with")
	return nil
}

func (ot *orderTracker) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	ot.visitOrder = append(ot.visitOrder, "try")
	return nil
}

func (ot *orderTracker) VisitWaitStatement(body []*Command) error {
	ot.visitOrder = append(ot.visitOrder, "wait")
	return nil
}

func (ot *orderTracker) VisitBaseCommand(c *Command) error {
	return nil
}

type fullTraversalCollector struct {
	commandCount map[string]int
	ifCount      int
	forCount     int
}

func (ft *fullTraversalCollector) VisitTarget(t *Target) error {
	return nil
}

func (ft *fullTraversalCollector) VisitFunction(f *Function) error {
	return nil
}

func (ft *fullTraversalCollector) VisitCommand(c *Command) error {
	if ft.commandCount == nil {
		ft.commandCount = make(map[string]int)
	}
	ft.commandCount[c.Name]++
	return nil
}

func (ft *fullTraversalCollector) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	ft.ifCount++
	return nil
}

func (ft *fullTraversalCollector) VisitForStatement(args []string, body []*Command) error {
	ft.forCount++
	return nil
}

func (ft *fullTraversalCollector) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

func (ft *fullTraversalCollector) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

func (ft *fullTraversalCollector) VisitWaitStatement(body []*Command) error {
	return nil
}

func (ft *fullTraversalCollector) VisitBaseCommand(c *Command) error {
	return nil
}
