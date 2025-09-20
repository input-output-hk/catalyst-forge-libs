package earthfile

// Visitor defines the interface for traversing Earthfile AST nodes.
// Implementations can perform custom operations on each node type.
// Return an error to stop traversal early.
type Visitor interface {
	// VisitTarget is called for each target in the Earthfile.
	VisitTarget(target *Target) error

	// VisitFunction is called for each user-defined function.
	VisitFunction(function *Function) error

	// VisitCommand is called for each command statement.
	VisitCommand(command *Command) error

	// VisitIfStatement is called for IF statements.
	// thenBlock contains commands in the IF body.
	// elseBlock contains commands in the ELSE body (can be nil).
	VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error

	// VisitForStatement is called for FOR loops.
	// args contains the loop variable and range expression.
	// body contains commands in the loop body.
	VisitForStatement(args []string, body []*Command) error

	// VisitWithStatement is called for WITH statements.
	// command is the WITH command itself.
	// body contains commands in the WITH body.
	VisitWithStatement(command *Command, body []*Command) error

	// VisitTryStatement is called for TRY statements.
	// tryBlock contains commands in the TRY body.
	// catchBlock contains commands in the CATCH body (can be nil).
	// finallyBlock contains commands in the FINALLY body (can be nil).
	VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error

	// VisitWaitStatement is called for WAIT statements.
	// body contains commands in the WAIT body.
	VisitWaitStatement(body []*Command) error

	// VisitBaseCommand is called for commands in the base recipe (before any targets).
	// This is an optional method that implementations can override for special handling.
	VisitBaseCommand(command *Command) error
}

// BaseVisitor provides default no-op implementations of all Visitor methods.
// Embed this struct to only override the methods you need.
type BaseVisitor struct{}

// VisitTarget is a no-op implementation.
func (v *BaseVisitor) VisitTarget(target *Target) error {
	return nil
}

// VisitFunction is a no-op implementation.
func (v *BaseVisitor) VisitFunction(function *Function) error {
	return nil
}

// VisitCommand is a no-op implementation.
func (v *BaseVisitor) VisitCommand(command *Command) error {
	return nil
}

// VisitIfStatement is a no-op implementation.
func (v *BaseVisitor) VisitIfStatement(condition []string, thenBlock, elseBlock []*Command) error {
	return nil
}

// VisitForStatement is a no-op implementation.
func (v *BaseVisitor) VisitForStatement(args []string, body []*Command) error {
	return nil
}

// VisitWithStatement is a no-op implementation.
func (v *BaseVisitor) VisitWithStatement(command *Command, body []*Command) error {
	return nil
}

// VisitTryStatement is a no-op implementation.
func (v *BaseVisitor) VisitTryStatement(tryBlock, catchBlock, finallyBlock []*Command) error {
	return nil
}

// VisitWaitStatement is a no-op implementation.
func (v *BaseVisitor) VisitWaitStatement(body []*Command) error {
	return nil
}

// VisitBaseCommand is a no-op implementation.
func (v *BaseVisitor) VisitBaseCommand(command *Command) error {
	return nil
}
