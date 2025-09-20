package earthfile

// FindCommands returns all commands of the specified type.
func (t *Target) FindCommands(cmdType CommandType) []*Command {
	// Fast-path using cached index if available
	if t.commandsByType != nil {
		return t.commandsByType[cmdType]
	}
	// Fallback: linear scan (builds a result slice each call)
	var commands []*Command
	for _, cmd := range t.Commands {
		if cmd.Type == cmdType {
			commands = append(commands, cmd)
		}
	}
	return commands
}

// GetFromBase returns the FROM command or nil if not found.
func (t *Target) GetFromBase() *Command {
	for _, cmd := range t.Commands {
		if cmd.Type == CommandTypeFrom {
			return cmd
		}
	}
	return nil
}

// GetArgs returns all ARG commands.
func (t *Target) GetArgs() []*Command {
	return t.FindCommands(CommandTypeArg)
}

// GetBuilds returns all BUILD commands.
func (t *Target) GetBuilds() []*Command {
	return t.FindCommands(CommandTypeBuild)
}

// GetArtifacts returns all SAVE ARTIFACT commands.
func (t *Target) GetArtifacts() []*Command {
	return t.FindCommands(CommandTypeSaveArtifact)
}

// GetImages returns all SAVE IMAGE commands.
func (t *Target) GetImages() []*Command {
	return t.FindCommands(CommandTypeSaveImage)
}

// HasCommand returns true if the target has at least one command of the specified type.
func (t *Target) HasCommand(cmdType CommandType) bool {
	if t.commandsByType != nil {
		return len(t.commandsByType[cmdType]) > 0
	}
	for _, cmd := range t.Commands {
		if cmd.Type == cmdType {
			return true
		}
	}
	return false
}

// WalkFunc represents a function to be called for each command during traversal.
type WalkFunc func(*Command, int) error

// WalkCommands traverses all commands in the target, calling fn for each.
// If fn returns an error, traversal stops and the error is returned.
func (t *Target) WalkCommands(fn WalkFunc) error {
	return t.walkCommands(t.Commands, 0, fn)
}

// walkCommands recursively walks commands (helper for nested blocks in future).
func (t *Target) walkCommands(commands []*Command, depth int, fn WalkFunc) error {
	for _, cmd := range commands {
		if err := fn(cmd, depth); err != nil {
			return err
		}
	}
	return nil
}

// Walk traverses the target's AST with the given visitor.
// The visitor methods are called for each node in the target's recipe.
//
//nolint:wrapcheck // Visitor errors should be returned as-is
func (t *Target) Walk(v Visitor) error {
	if t.recipe == nil {
		// Fallback to just visiting the flattened commands if no AST
		for _, cmd := range t.Commands {
			if err := v.VisitCommand(cmd); err != nil {
				return err
			}
		}
		return nil
	}

	// Walk the raw AST recipe
	return walkBlock(t.recipe, v, false)
}
