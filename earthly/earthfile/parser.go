package earthfile

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/Masterminds/semver/v3"
	"github.com/earthly/earthly/ast"
	"github.com/earthly/earthly/ast/spec"
	fs "github.com/input-output-hk/catalyst-forge-libs/fs"
	"github.com/input-output-hk/catalyst-forge-libs/fs/billy"
)

// MaxSupportedVersion is the maximum Earthfile version we support
const MaxSupportedVersion = "0.8"

// ParseOptions provides options for parsing Earthfiles.
type ParseOptions struct {
	// EnableSourceMap enables source location tracking.
	// Note: This is currently not supported when using filesystem abstraction
	// due to limitations in the underlying AST parser's FromReader approach.
	EnableSourceMap bool
	// StrictMode enables strict validation rules
	StrictMode bool
	// Filesystem allows injecting a custom filesystem implementation.
	// If nil, defaults to billy.NewBaseOSFS()
	Filesystem fs.Filesystem
}

// Parse parses an Earthfile from the given file path.
func Parse(path string) (*Earthfile, error) {
	return ParseContext(context.Background(), path)
}

// ParseWithOptions parses an Earthfile with custom options.
func ParseWithOptions(path string, opts *ParseOptions) (*Earthfile, error) {
	return ParseWithOptionsContext(context.Background(), path, opts)
}

// ParseContext parses an Earthfile with cancellation support.
func ParseContext(ctx context.Context, path string) (*Earthfile, error) {
	return ParseWithOptionsContext(ctx, path, nil)
}

// ParseWithOptionsContext parses an Earthfile with custom options and cancellation support.
func ParseWithOptionsContext(ctx context.Context, path string, opts *ParseOptions) (*Earthfile, error) {
	// Check context cancellation first
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	// Use default options if nil
	if opts == nil {
		opts = &ParseOptions{}
	}

	// Use default filesystem if nil
	filesystem := opts.Filesystem
	if filesystem == nil {
		filesystem = billy.NewBaseOSFS()
	}

	// Read file using filesystem abstraction
	content, err := filesystem.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	// Create a named reader
	reader := newNamedReader(content, path)

	// Parse using ast.FromReader
	// Note: Source maps are not currently supported when using FromReader approach
	// This is a limitation of the underlying AST parser
	astEarthfile, err := ast.ParseOpts(ctx, ast.FromReader(reader))
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	// Convert AST to our domain model
	return convertASTToDomain(&astEarthfile, opts)
}

// ParseString parses an Earthfile from a string.
func ParseString(content string) (*Earthfile, error) {
	return ParseStringWithOptions(content, nil)
}

// ParseStringWithOptions parses an Earthfile from a string with options.
func ParseStringWithOptions(content string, opts *ParseOptions) (*Earthfile, error) {
	ctx := context.Background()

	// Use default options if nil
	if opts == nil {
		opts = &ParseOptions{}
	}

	// Create a named reader from the content
	reader := newNamedReader([]byte(content), "Earthfile")

	// Parse using ast.FromReader
	astEarthfile, err := ast.ParseOpts(ctx, ast.FromReader(reader))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Earthfile from string: %w", err)
	}

	// Convert AST to our domain model
	return convertASTToDomain(&astEarthfile, opts)
}

// ParseReader parses an Earthfile from an io.Reader.
func ParseReader(reader io.Reader, name string) (*Earthfile, error) {
	return ParseReaderWithOptions(reader, name, nil)
}

// ParseReaderWithOptions parses an Earthfile from an io.Reader with options.
func ParseReaderWithOptions(reader io.Reader, name string, opts *ParseOptions) (*Earthfile, error) {
	ctx := context.Background()

	// Use default options if nil
	if opts == nil {
		opts = &ParseOptions{}
	}

	// Read all content first
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read from reader: %w", err)
	}

	// Create a named reader
	namedReader := newNamedReader(content, name)

	// Parse using ast.FromReader
	astEarthfile, err := ast.ParseOpts(ctx, ast.FromReader(namedReader))
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", name, err)
	}

	// Convert AST to our domain model
	return convertASTToDomain(&astEarthfile, opts)
}

// ParseVersion parses only the VERSION from an Earthfile (lightweight operation).
func ParseVersion(path string) (string, error) {
	return ParseVersionWithFilesystem(path, nil)
}

// ParseVersionWithFilesystem parses only the VERSION from an Earthfile using the provided filesystem.
func ParseVersionWithFilesystem(path string, filesystem fs.Filesystem) (string, error) {
	// Use default filesystem if nil
	if filesystem == nil {
		filesystem = billy.NewBaseOSFS()
	}

	// Read file using filesystem abstraction
	content, err := filesystem.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", path, err)
	}

	// Parse VERSION from content manually using scanner for efficiency
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Bytes()
		trimmed := bytes.TrimSpace(line)

		// Skip empty lines and comments
		if len(trimmed) == 0 || bytes.HasPrefix(trimmed, []byte("#")) {
			continue
		}

		// Check if this is a VERSION command
		if bytes.HasPrefix(trimmed, []byte("VERSION")) {
			// Extract version number (last token after VERSION)
			parts := bytes.Fields(trimmed)
			if len(parts) >= 2 {
				return string(parts[len(parts)-1]), nil
			}
		}

		// Stop after first non-comment, non-empty line that isn't VERSION
		// VERSION must be the first non-comment statement in an Earthfile
		break
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error scanning file: %w", err)
	}

	return "", nil
}

// convertASTToDomain converts the AST representation to our domain model
func convertASTToDomain(astEf *spec.Earthfile, opts *ParseOptions) (*Earthfile, error) {
	ef := NewEarthfile()

	// Store the original AST
	ef.ast = astEf

	// Extract version if present
	if astEf.Version != nil && len(astEf.Version.Args) > 0 {
		ef.version = astEf.Version.Args[len(astEf.Version.Args)-1]
	}

	// Convert base recipe commands
	ef.baseCommands = convertBlock(astEf.BaseRecipe, opts.EnableSourceMap)

	// Convert targets
	for _, astTarget := range astEf.Targets {
		target := &Target{
			Name:     astTarget.Name,
			Commands: convertBlock(astTarget.Recipe, opts.EnableSourceMap),
		}
		ef.targets[astTarget.Name] = target
	}

	// Convert user-defined commands (functions)
	for _, astUserCmd := range astEf.Functions {
		function := &Function{
			Name:     astUserCmd.Name,
			Commands: convertBlock(astUserCmd.Recipe, opts.EnableSourceMap),
		}
		ef.functions[astUserCmd.Name] = function
	}

	// Apply strict mode validation if enabled
	if opts.StrictMode {
		if err := validateStrict(ef); err != nil {
			return nil, fmt.Errorf("strict validation failed: %w", err)
		}
	}

	return ef, nil
}

// convertBlock converts a spec.Block to a slice of Commands
func convertBlock(block spec.Block, enableSourceMap bool) []*Command {
	var commands []*Command

	for _, stmt := range block {
		cmds := convertStatement(stmt, enableSourceMap)
		commands = append(commands, cmds...)
	}

	return commands
}

// convertStatement converts a spec.Statement to Commands
//
//nolint:cyclop // High complexity is inherent to AST statement type dispatch
func convertStatement(stmt spec.Statement, enableSourceMap bool) []*Command {
	var commands []*Command

	// Handle regular command
	if stmt.Command != nil {
		cmd := convertCommand(stmt.Command, enableSourceMap)
		if cmd != nil {
			commands = append(commands, cmd)
		}
	}

	// Handle WITH statement
	if stmt.With != nil {
		// Add the WITH command itself
		withCmd := &Command{
			Name: "WITH",
			Type: CommandTypeWith,
			Args: []string{},
		}
		if enableSourceMap && stmt.With.SourceLocation != nil {
			withCmd.Location = convertSourceLocation(stmt.With.SourceLocation)
		}
		commands = append(commands, withCmd)

		// Process nested commands in WITH body
		nestedCmds := convertBlock(stmt.With.Body, enableSourceMap)
		commands = append(commands, nestedCmds...)
	}

	// Handle IF statement
	if stmt.If != nil {
		// Create an IF command
		cmd := &Command{
			Name: "IF",
			Type: CommandTypeIf,
			Args: stmt.If.Expression,
		}
		if enableSourceMap && stmt.If.SourceLocation != nil {
			cmd.Location = convertSourceLocation(stmt.If.SourceLocation)
		}
		commands = append(commands, cmd)

		// Process IF body
		nestedCmds := convertBlock(stmt.If.IfBody, enableSourceMap)
		commands = append(commands, nestedCmds...)

		// Process ELSE IF branches
		for _, elseIf := range stmt.If.ElseIf {
			elseIfCmd := &Command{
				Name: "ELSE IF",
				Type: CommandTypeIf,
				Args: elseIf.Expression,
			}
			if enableSourceMap && elseIf.SourceLocation != nil {
				elseIfCmd.Location = convertSourceLocation(elseIf.SourceLocation)
			}
			commands = append(commands, elseIfCmd)

			nestedCmds := convertBlock(elseIf.Body, enableSourceMap)
			commands = append(commands, nestedCmds...)
		}

		// Process ELSE body
		if stmt.If.ElseBody != nil {
			elseCmd := &Command{
				Name: "ELSE",
				Type: CommandTypeIf,
				Args: []string{},
			}
			commands = append(commands, elseCmd)

			nestedCmds := convertBlock(*stmt.If.ElseBody, enableSourceMap)
			commands = append(commands, nestedCmds...)
		}
	}

	// Handle FOR statement
	if stmt.For != nil {
		cmd := &Command{
			Name: "FOR",
			Type: CommandTypeFor,
			Args: stmt.For.Args,
		}
		if enableSourceMap && stmt.For.SourceLocation != nil {
			cmd.Location = convertSourceLocation(stmt.For.SourceLocation)
		}
		commands = append(commands, cmd)

		// Process FOR body
		nestedCmds := convertBlock(stmt.For.Body, enableSourceMap)
		commands = append(commands, nestedCmds...)
	}

	// Handle WAIT statement
	if stmt.Wait != nil {
		// Process WAIT body
		nestedCmds := convertBlock(stmt.Wait.Body, enableSourceMap)
		commands = append(commands, nestedCmds...)

		// Add END command for WAIT
		cmd := &Command{
			Name: "END",
			Type: CommandTypeWait,
			Args: []string{},
		}
		if enableSourceMap && stmt.Wait.SourceLocation != nil {
			cmd.Location = convertSourceLocation(stmt.Wait.SourceLocation)
		}
		commands = append(commands, cmd)
	}

	return commands
}

// convertCommand converts a spec.Command to our Command type
func convertCommand(specCmd *spec.Command, enableSourceMap bool) *Command {
	if specCmd == nil {
		return nil
	}

	cmd := &Command{
		Name: specCmd.Name,
		Args: specCmd.Args,
		Type: getCommandType(specCmd.Name),
	}

	if enableSourceMap && specCmd.SourceLocation != nil {
		cmd.Location = convertSourceLocation(specCmd.SourceLocation)
	}

	return cmd
}

// convertSourceLocation converts spec.SourceLocation to our SourceLocation
func convertSourceLocation(specLoc *spec.SourceLocation) *SourceLocation {
	if specLoc == nil {
		return nil
	}

	return &SourceLocation{
		File:        specLoc.File,
		StartLine:   specLoc.StartLine,
		StartColumn: specLoc.StartColumn,
		EndLine:     specLoc.EndLine,
		EndColumn:   specLoc.EndColumn,
	}
}

// getCommandType returns the CommandType for a given command name
//
//nolint:cyclop,funlen // High complexity and length required to map all command types
func getCommandType(name string) CommandType {
	// Map command names to types
	switch name {
	case "FROM":
		return CommandTypeFrom
	case "RUN":
		return CommandTypeRun
	case "COPY":
		return CommandTypeCopy
	case "BUILD":
		return CommandTypeBuild
	case "ARG":
		return CommandTypeArg
	case "SAVE ARTIFACT":
		return CommandTypeSaveArtifact
	case "SAVE IMAGE":
		return CommandTypeSaveImage
	case "CMD":
		return CommandTypeCmd
	case "ENTRYPOINT":
		return CommandTypeEntrypoint
	case "EXPOSE":
		return CommandTypeExpose
	case "VOLUME":
		return CommandTypeVolume
	case "ENV":
		return CommandTypeEnv
	case "WORKDIR":
		return CommandTypeWorkdir
	case "USER":
		return CommandTypeUser
	case "GIT CLONE":
		return CommandTypeGitClone
	case "ADD":
		return CommandTypeAdd
	case "STOPSIGNAL":
		return CommandTypeStopsignal
	case "ONBUILD":
		return CommandTypeOnbuild
	case "HEALTHCHECK":
		return CommandTypeHealthcheck
	case "SHELL":
		return CommandTypeShell
	case "DO":
		return CommandTypeDo
	case "COMMAND":
		return CommandTypeCommand
	case "IMPORT":
		return CommandTypeImport
	case "VERSION":
		return CommandTypeVersion
	case "FROM DOCKERFILE":
		return CommandTypeFromDockerfile
	case "LOCALLY":
		return CommandTypeLocally
	case "HOST":
		return CommandTypeHost
	case "PROJECT":
		return CommandTypeProject
	case "CACHE":
		return CommandTypeCache
	case "SET":
		return CommandTypeSet
	case "LET":
		return CommandTypeLet
	case "TRY":
		return CommandTypeTry
	case "WITH":
		return CommandTypeWith
	case "IF", "ELSE IF", "ELSE":
		return CommandTypeIf
	case "FOR":
		return CommandTypeFor
	case "WAIT", "END":
		return CommandTypeWait
	default:
		return CommandTypeUnknown
	}
}

// validateStrict performs strict validation on the parsed Earthfile.
func validateStrict(ef *Earthfile) error {
	// Note: The AST parser already validates that target names can't be "base"
	// and other reserved keywords, so we don't need to check that here

	// Validate VERSION format if present
	if ef.HasVersion() {
		version := ef.Version()

		// Parse the version
		v, err := semver.NewVersion(version)
		if err != nil {
			return fmt.Errorf("invalid VERSION format %q: %w", version, err)
		}

		// Check if version is supported (must be <= MaxSupportedVersion)
		maxVersion, err := semver.NewVersion(MaxSupportedVersion)
		if err != nil {
			// This should never happen since MaxSupportedVersion is a constant
			return fmt.Errorf("internal error: invalid max version constant: %w", err)
		}

		if v.GreaterThan(maxVersion) {
			return fmt.Errorf(
				"VERSION %s is not supported (maximum supported version is %s)",
				version, MaxSupportedVersion,
			)
		}
	}

	return nil
}

// Earthfile method additions

// TargetNames returns a list of all target names.
func (ef *Earthfile) TargetNames() []string {
	names := make([]string, 0, len(ef.targets))
	for name := range ef.targets {
		names = append(names, name)
	}
	return names
}

// HasTarget returns true if the Earthfile has the specified target.
func (ef *Earthfile) HasTarget(name string) bool {
	_, exists := ef.targets[name]
	return exists
}

// Target returns the specified target or nil if not found.
func (ef *Earthfile) Target(name string) *Target {
	return ef.targets[name]
}

// Targets returns all targets.
func (ef *Earthfile) Targets() []*Target {
	targets := make([]*Target, 0, len(ef.targets))
	for _, target := range ef.targets {
		targets = append(targets, target)
	}
	return targets
}

// Function returns the specified function or nil if not found.
func (ef *Earthfile) Function(name string) *Function {
	return ef.functions[name]
}

// Functions returns all functions.
func (ef *Earthfile) Functions() []*Function {
	funcs := make([]*Function, 0, len(ef.functions))
	for _, fn := range ef.functions {
		funcs = append(funcs, fn)
	}
	return funcs
}
