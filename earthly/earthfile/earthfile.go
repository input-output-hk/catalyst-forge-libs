// Package earthfile provides a high-level Go API for parsing and analyzing Earthfiles.
// It wraps the low-level Earthbuild AST parser with an ergonomic interface optimized for tooling development.
package earthfile

import (
	"errors"

	"github.com/earthly/earthly/ast/spec"
)

// Walk traverses the Earthfile AST with the given visitor.
// The visitor methods are called for each node in the AST.
// Returns the first error returned by any visitor method.
//
//nolint:wrapcheck // Visitor errors should be returned as-is
func (ef *Earthfile) Walk(v Visitor) error {
	if ef == nil || ef.ast == nil {
		return nil
	}

	// Visit base commands
	if err := walkBlock(ef.ast.BaseRecipe, v, true); err != nil {
		return err
	}

	// Visit targets
	for _, astTarget := range ef.ast.Targets {
		target := ef.Target(astTarget.Name)
		if target == nil {
			continue
		}

		if err := v.VisitTarget(target); err != nil {
			return err
		}

		// Walk the target's recipe
		if err := walkBlock(astTarget.Recipe, v, false); err != nil {
			return err
		}
	}

	// Visit functions
	for _, astFunc := range ef.ast.Functions {
		function := ef.Function(astFunc.Name)
		if function == nil {
			continue
		}

		if err := v.VisitFunction(function); err != nil {
			return err
		}

		// Walk the function's recipe
		if err := walkBlock(astFunc.Recipe, v, false); err != nil {
			return err
		}
	}

	return nil
}

// walkBlock walks through a block of statements
func walkBlock(block spec.Block, v Visitor, isBase bool) error {
	for _, stmt := range block {
		if err := walkStatement(stmt, v, isBase); err != nil {
			return err
		}
	}
	return nil
}

// walkStatement walks through a single statement
//
//nolint:cyclop,nestif,wrapcheck,funlen // Statement type dispatch is naturally complex, visitor errors returned as-is
func walkStatement(stmt spec.Statement, v Visitor, isBase bool) error {
	// Handle regular command
	if stmt.Command != nil {
		cmd := convertCommand(stmt.Command, false)
		if cmd != nil {
			if isBase {
				if err := v.VisitBaseCommand(cmd); err != nil {
					return err
				}
			}
			if err := v.VisitCommand(cmd); err != nil {
				return err
			}
		}
	}

	// Handle IF statement
	if stmt.If != nil {
		// Convert blocks to command slices for visitor
		thenCommands := convertBlock(stmt.If.IfBody, false)
		var elseCommands []*Command
		if stmt.If.ElseBody != nil {
			elseCommands = convertBlock(*stmt.If.ElseBody, false)
		}

		if err := v.VisitIfStatement(stmt.If.Expression, thenCommands, elseCommands); err != nil {
			return err
		}

		// Walk through IF body
		if err := walkBlock(stmt.If.IfBody, v, false); err != nil {
			return err
		}

		// Walk through ELSE IF branches
		for _, elseIf := range stmt.If.ElseIf {
			if err := walkBlock(elseIf.Body, v, false); err != nil {
				return err
			}
		}

		// Walk through ELSE body
		if stmt.If.ElseBody != nil {
			if err := walkBlock(*stmt.If.ElseBody, v, false); err != nil {
				return err
			}
		}
	}

	// Handle FOR statement
	if stmt.For != nil {
		bodyCommands := convertBlock(stmt.For.Body, false)
		if err := v.VisitForStatement(stmt.For.Args, bodyCommands); err != nil {
			return err
		}

		// Walk through FOR body
		if err := walkBlock(stmt.For.Body, v, false); err != nil {
			return err
		}
	}

	// Handle WITH statement
	if stmt.With != nil {
		withCmd := &Command{
			Name: "WITH",
			Type: CommandTypeWith,
			Args: []string{},
		}
		// WITH command has its args directly in the spec
		if stmt.With.Command.Name != "" {
			withCmd.Name = stmt.With.Command.Name
			withCmd.Args = stmt.With.Command.Args
		}

		bodyCommands := convertBlock(stmt.With.Body, false)
		if err := v.VisitWithStatement(withCmd, bodyCommands); err != nil {
			return err
		}

		// Walk through WITH body
		if err := walkBlock(stmt.With.Body, v, false); err != nil {
			return err
		}
	}

	// Handle TRY statement
	if stmt.Try != nil {
		tryCommands := convertBlock(stmt.Try.TryBody, false)
		var catchCommands []*Command
		if stmt.Try.CatchBody != nil {
			catchCommands = convertBlock(*stmt.Try.CatchBody, false)
		}
		var finallyCommands []*Command
		if stmt.Try.FinallyBody != nil {
			finallyCommands = convertBlock(*stmt.Try.FinallyBody, false)
		}

		if err := v.VisitTryStatement(tryCommands, catchCommands, finallyCommands); err != nil {
			return err
		}

		// Walk through TRY body
		if err := walkBlock(stmt.Try.TryBody, v, false); err != nil {
			return err
		}

		// Walk through CATCH body
		if stmt.Try.CatchBody != nil {
			if err := walkBlock(*stmt.Try.CatchBody, v, false); err != nil {
				return err
			}
		}

		// Walk through FINALLY body
		if stmt.Try.FinallyBody != nil {
			if err := walkBlock(*stmt.Try.FinallyBody, v, false); err != nil {
				return err
			}
		}
	}

	// Handle WAIT statement
	if stmt.Wait != nil {
		bodyCommands := convertBlock(stmt.Wait.Body, false)
		if err := v.VisitWaitStatement(bodyCommands); err != nil {
			return err
		}

		// Walk through WAIT body
		if err := walkBlock(stmt.Wait.Body, v, false); err != nil {
			return err
		}
	}

	return nil
}

// WalkCommands traverses all commands in the Earthfile, calling fn for each.
// The depth parameter indicates the nesting level (0 for top-level commands).
// If fn returns an error, traversal stops and the error is returned.
func (ef *Earthfile) WalkCommands(fn WalkFunc) error {
	if fn == nil {
		return errors.New("WalkFunc cannot be nil")
	}

	if ef == nil || ef.ast == nil {
		return nil
	}

	// Walk base commands
	if err := walkCommandsInBlock(ef.ast.BaseRecipe, 0, fn); err != nil {
		return err
	}

	// Walk targets
	for _, target := range ef.ast.Targets {
		if err := walkCommandsInBlock(target.Recipe, 0, fn); err != nil {
			return err
		}
	}

	// Walk functions
	for _, function := range ef.ast.Functions {
		if err := walkCommandsInBlock(function.Recipe, 0, fn); err != nil {
			return err
		}
	}

	return nil
}

// walkCommandsInBlock walks commands in a block, tracking depth
func walkCommandsInBlock(block spec.Block, depth int, fn WalkFunc) error {
	for _, stmt := range block {
		if err := walkCommandsInStatement(stmt, depth, fn); err != nil {
			return err
		}
	}
	return nil
}

// walkCommandsInStatement walks commands in a statement
//
//nolint:cyclop,nestif,funlen // Statement type dispatch is naturally complex
func walkCommandsInStatement(stmt spec.Statement, depth int, fn WalkFunc) error {
	// Handle regular command
	if stmt.Command != nil {
		cmd := convertCommand(stmt.Command, false)
		if cmd != nil {
			if err := fn(cmd, depth); err != nil {
				return err
			}
		}
	}

	// Handle IF statement
	if stmt.If != nil {
		// Visit the IF command itself
		ifCmd := &Command{
			Name: internCommandName("IF"),
			Type: CommandTypeIf,
			Args: stmt.If.Expression,
		}
		if err := fn(ifCmd, depth); err != nil {
			return err
		}

		// Walk IF body
		if err := walkCommandsInBlock(stmt.If.IfBody, depth+1, fn); err != nil {
			return err
		}

		// Walk ELSE IF branches
		for _, elseIf := range stmt.If.ElseIf {
			elseIfCmd := &Command{
				Name: internCommandName("ELSE IF"),
				Type: CommandTypeIf,
				Args: elseIf.Expression,
			}
			if err := fn(elseIfCmd, depth); err != nil {
				return err
			}
			if err := walkCommandsInBlock(elseIf.Body, depth+1, fn); err != nil {
				return err
			}
		}

		// Walk ELSE body
		if stmt.If.ElseBody != nil {
			elseCmd := &Command{
				Name: internCommandName("ELSE"),
				Type: CommandTypeIf,
				Args: []string{},
			}
			if err := fn(elseCmd, depth); err != nil {
				return err
			}
			if err := walkCommandsInBlock(*stmt.If.ElseBody, depth+1, fn); err != nil {
				return err
			}
		}
	}

	// Handle FOR statement
	if stmt.For != nil {
		forCmd := &Command{
			Name: internCommandName("FOR"),
			Type: CommandTypeFor,
			Args: stmt.For.Args,
		}
		if err := fn(forCmd, depth); err != nil {
			return err
		}

		// Walk FOR body
		if err := walkCommandsInBlock(stmt.For.Body, depth+1, fn); err != nil {
			return err
		}
	}

	// Handle WITH statement
	if stmt.With != nil {
		withCmd := &Command{
			Name: internCommandName(stmt.With.Command.Name),
			Type: CommandTypeWith,
			Args: stmt.With.Command.Args,
		}
		if err := fn(withCmd, depth); err != nil {
			return err
		}

		// Walk WITH body
		if err := walkCommandsInBlock(stmt.With.Body, depth+1, fn); err != nil {
			return err
		}
	}

	// Handle TRY statement
	if stmt.Try != nil {
		tryCmd := &Command{
			Name: internCommandName("TRY"),
			Type: CommandTypeTry,
			Args: []string{},
		}
		if err := fn(tryCmd, depth); err != nil {
			return err
		}

		// Walk TRY body
		if err := walkCommandsInBlock(stmt.Try.TryBody, depth+1, fn); err != nil {
			return err
		}

		// Walk CATCH body
		if stmt.Try.CatchBody != nil {
			catchCmd := &Command{
				Name: internCommandName("CATCH"),
				Type: CommandTypeTry,
				Args: []string{},
			}
			if err := fn(catchCmd, depth); err != nil {
				return err
			}
			if err := walkCommandsInBlock(*stmt.Try.CatchBody, depth+1, fn); err != nil {
				return err
			}
		}

		// Walk FINALLY body
		if stmt.Try.FinallyBody != nil {
			finallyCmd := &Command{
				Name: internCommandName("FINALLY"),
				Type: CommandTypeTry,
				Args: []string{},
			}
			if err := fn(finallyCmd, depth); err != nil {
				return err
			}
			if err := walkCommandsInBlock(*stmt.Try.FinallyBody, depth+1, fn); err != nil {
				return err
			}
		}
	}

	// Handle WAIT statement
	if stmt.Wait != nil {
		waitCmd := &Command{
			Name: internCommandName("WAIT"),
			Type: CommandTypeWait,
			Args: []string{},
		}
		if err := fn(waitCmd, depth); err != nil {
			return err
		}

		// Walk WAIT body
		if err := walkCommandsInBlock(stmt.Wait.Body, depth+1, fn); err != nil {
			return err
		}
	}

	return nil
}
