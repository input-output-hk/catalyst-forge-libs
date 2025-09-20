package earthfile

import (
	"strings"

	"github.com/earthly/earthly/ast/spec"
)

// Command name constants to avoid repetition
const (
	cmdFrom  = "FROM"
	cmdBuild = "BUILD"
	cmdCopy  = "COPY"
)

// Dependencies returns the list of dependencies (lazy-loaded).
// Dependencies are extracted from BUILD, FROM, and COPY commands
// throughout the Earthfile, including within control flow statements.
func (ef *Earthfile) Dependencies() []Dependency {
	if ef == nil {
		return nil
	}

	// Lazy load dependencies
	ef.initDependencies()
	return ef.dependencies
}

// initDependencies initializes the dependencies list if not already done
func (ef *Earthfile) initDependencies() {
	// Check if already initialized
	if ef.dependenciesLoaded {
		return
	}

	ef.dependenciesLoaded = true
	ef.dependencies = ef.parseDependencies()
}

// parseDependencies extracts all dependencies from the Earthfile
func (ef *Earthfile) parseDependencies() []Dependency {
	// If AST is nil, return empty list
	if ef.ast == nil {
		return nil
	}

	var deps []Dependency
	seen := make(map[string]bool) // Track unique dependencies

	// Helper to add dependency if not seen
	addDep := func(target, source string) {
		if target == "" {
			return
		}
		dep := Dependency{
			Target: target,
			Local:  isLocalReference(target),
			Source: source,
		}
		key := dep.Source + ":" + dep.Target
		if !seen[key] {
			seen[key] = true
			deps = append(deps, dep)
		}
	}

	// Process a command for dependencies
	processCommand := func(cmd *spec.Command, source string) {
		if cmd == nil {
			return
		}

		switch cmd.Name {
		case cmdBuild:
			processBuildCommand(cmd, source, addDep)
		case cmdFrom:
			processFromCommand(cmd, source, addDep)
		case cmdCopy:
			processCopyCommand(cmd, source, addDep)
		}
	}

	// Process statements recursively
	var processBlock func(block spec.Block, source string)
	processBlock = func(block spec.Block, source string) {
		for _, stmt := range block {
			processStatement(stmt, source, processCommand, processBlock)
		}
	}

	// Process base recipe (commands before any target)
	processBlock(ef.ast.BaseRecipe, "")

	// Process each target
	for _, target := range ef.ast.Targets {
		processBlock(target.Recipe, target.Name)
	}

	// Note: Functions are not processed as they are only templates
	// and dependencies only matter when they are called

	return deps
}

// processStatement processes a single statement for dependencies
func processStatement(stmt spec.Statement, source string,
	processCmd func(*spec.Command, string),
	processBlk func(spec.Block, string),
) {
	if stmt.Command != nil {
		processCmd(stmt.Command, source)
	}
	if stmt.If != nil {
		processIfStatement(stmt.If, source, processBlk)
	}
	if stmt.For != nil {
		// For dynamic targets in FOR loops, we need to parse the actual command
		// For now, we'll process the body which may have BUILD commands
		processBlk(stmt.For.Body, source)
	}
	if stmt.With != nil {
		processBlk(stmt.With.Body, source)
	}
	if stmt.Try != nil {
		processTryStatement(stmt.Try, source, processBlk)
	}
	if stmt.Wait != nil {
		processBlk(stmt.Wait.Body, source)
	}
}

// processIfStatement handles IF statement blocks
func processIfStatement(ifStmt *spec.IfStatement, source string, processBlk func(spec.Block, string)) {
	processBlk(ifStmt.IfBody, source)
	for _, elseIf := range ifStmt.ElseIf {
		processBlk(elseIf.Body, source)
	}
	if ifStmt.ElseBody != nil {
		processBlk(*ifStmt.ElseBody, source)
	}
}

// processTryStatement handles TRY statement blocks
func processTryStatement(tryStmt *spec.TryStatement, source string, processBlk func(spec.Block, string)) {
	processBlk(tryStmt.TryBody, source)
	if tryStmt.CatchBody != nil {
		processBlk(*tryStmt.CatchBody, source)
	}
	if tryStmt.FinallyBody != nil {
		processBlk(*tryStmt.FinallyBody, source)
	}
}

// processBuildCommand handles BUILD command dependencies
func processBuildCommand(cmd *spec.Command, source string, addDep func(string, string)) {
	// BUILD commands always have the target as the first argument
	if len(cmd.Args) > 0 {
		addDep(cmd.Args[0], source)
	}
}

// processFromCommand handles FROM command dependencies
func processFromCommand(cmd *spec.Command, source string, addDep func(string, string)) {
	// FROM commands can reference other targets
	if len(cmd.Args) > 0 {
		arg := cmd.Args[0]
		// Check if it's a target reference (starts with + or contains +)
		if isTargetReference(arg) {
			addDep(arg, source)
		}
	}
}

// processCopyCommand handles COPY command dependencies
func processCopyCommand(cmd *spec.Command, source string, addDep func(string, string)) {
	// COPY commands can reference artifacts from other targets
	// Format: COPY [--from=target] source... dest
	// Or: COPY target/artifact dest
	for _, arg := range cmd.Args {
		// Check for --from flag
		if strings.HasPrefix(arg, "--from=") {
			target := strings.TrimPrefix(arg, "--from=")
			if isTargetReference(target) {
				addDep(target, source)
			}
		} else if isTargetWithArtifact(arg) {
			// Extract target from artifact path (e.g., "+build/app" -> "+build")
			if target := extractTargetFromArtifact(arg); target != "" {
				addDep(target, source)
			}
		}
	}
}

// isTargetReference checks if a string is a reference to another target
func isTargetReference(s string) bool {
	// Target references start with + or contain +
	// Examples: "+build", "./other+target", "github.com/org/repo+target"
	return strings.Contains(s, "+")
}

// isLocalReference checks if a target reference is local
func isLocalReference(target string) bool {
	// Local references:
	// - Start with + (e.g., "+build")
	// - Start with ./ or ../ (e.g., "./subdir+target")
	// Remote references contain domain names (e.g., "github.com/...")
	if strings.HasPrefix(target, "+") {
		return true
	}
	if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") {
		return true
	}
	// If it contains a domain-like pattern, it's remote
	if strings.Contains(target, ".com/") || strings.Contains(target, ".org/") || strings.Contains(target, ".io/") {
		return false
	}
	// Default to local if it has a + but no domain
	return strings.Contains(target, "+")
}

// isTargetWithArtifact checks if a string contains a target with an artifact path
func isTargetWithArtifact(s string) bool {
	// Check if it's a target reference followed by a path
	// Examples: "+build/app", "github.com/org/repo+build/artifact"
	if !strings.Contains(s, "+") {
		return false
	}
	// Check if there's a path separator after the +
	parts := strings.SplitN(s, "+", 2)
	if len(parts) == 2 {
		// Check if the part after + contains a /
		return strings.Contains(parts[1], "/")
	}
	return false
}

// extractTargetFromArtifact extracts the target from an artifact path
func extractTargetFromArtifact(s string) string {
	// Extract target from paths like "+build/app" -> "+build"
	// or "github.com/org/repo+build/artifact" -> "github.com/org/repo+build"
	if !strings.Contains(s, "+") {
		return ""
	}

	// Find the position of + and the next /
	plusIndex := strings.Index(s, "+")
	if plusIndex == -1 {
		return ""
	}
	afterPlus := s[plusIndex:]
	slashIndex := strings.Index(afterPlus, "/")

	if slashIndex > 0 {
		// Return everything up to the slash after +
		return s[:plusIndex+slashIndex]
	}

	// No slash after +, might not be an artifact reference
	return ""
}
