package earthfile

import "github.com/earthly/earthly/ast/spec"

// Earthfile represents a parsed Earthfile with indexed access to targets and functions.
type Earthfile struct {
	// Pre-computed maps for O(1) lookups
	targets   map[string]*Target
	functions map[string]*Function

	// Cached computed properties
	dependencies []Dependency // Lazy-loaded
	version      string       // Cached from AST

	// Raw commands before any target
	baseCommands []*Command

	// Original AST for advanced operations
	ast *spec.Earthfile
}

// BaseCommands returns the commands that appear before any target.
func (ef *Earthfile) BaseCommands() []*Command {
	return ef.baseCommands
}

// AST returns the underlying AST for advanced operations.
func (ef *Earthfile) AST() *spec.Earthfile {
	return ef.ast
}

// Dependencies returns the list of dependencies (lazy-loaded).
func (ef *Earthfile) Dependencies() []Dependency {
	return ef.dependencies
}

// Target represents a build target with associated commands and metadata.
type Target struct {
	Name     string     // Target name (e.g., "build", "test")
	Docs     string     // Documentation/comments before target
	Commands []*Command // List of commands in this target
}

// Function represents a user-defined function with reusable commands.
type Function struct {
	Name     string     // Function name
	Commands []*Command // List of commands in this function
}

// Command represents an individual instruction with type, arguments, and position.
type Command struct {
	Name     string          // Command name (e.g., "RUN", "COPY", "FROM")
	Type     CommandType     // Enumerated command type
	Args     []string        // Command arguments
	Location *SourceLocation // Source file location (if source map enabled)
}

// SourceLocation represents a position in the source file.
type SourceLocation struct {
	File        string // Source file path
	StartLine   int    // 1-based line number where element starts
	StartColumn int    // 0-based column where element starts
	EndLine     int    // 1-based line number where element ends
	EndColumn   int    // 0-based column where element ends
}

// Dependency represents a dependency on another target.
type Dependency struct {
	Target string // Target name (e.g., "./other:target" or "github.com/org/repo:target")
	Local  bool   // True if dependency is in same repo
	Source string // Source target that has this dependency
}

// Reference represents a parsed reference to another target.
type Reference struct {
	Target string // Target name
	Local  bool   // True if reference is local to this repo
	Remote bool   // True if reference is remote
	Path   string // Path component (for local refs)
}

// NewEarthfile creates a new Earthfile with initialized maps.
func NewEarthfile() *Earthfile {
	return &Earthfile{
		targets:   make(map[string]*Target),
		functions: make(map[string]*Function),
	}
}

// Version returns the Earthfile version string.
func (ef *Earthfile) Version() string {
	return ef.version
}

// HasVersion returns true if the Earthfile has a VERSION command.
func (ef *Earthfile) HasVersion() bool {
	return ef.version != ""
}

// CommandType represents the type of an Earthfile command.
type CommandType int

// Command types enumeration
const (
	CommandTypeUnknown CommandType = iota
	CommandTypeFrom
	CommandTypeRun
	CommandTypeCopy
	CommandTypeBuild
	CommandTypeArg
	CommandTypeSaveArtifact
	CommandTypeSaveImage
	CommandTypeCmd
	CommandTypeEntrypoint
	CommandTypeExpose
	CommandTypeVolume
	CommandTypeEnv
	CommandTypeWorkdir
	CommandTypeUser
	CommandTypeGitClone
	CommandTypeAdd
	CommandTypeStopsignal
	CommandTypeOnbuild
	CommandTypeHealthcheck
	CommandTypeShell
	CommandTypeDo
	CommandTypeCommand
	CommandTypeImport
	CommandTypeVersion
	CommandTypeFromDockerfile
	CommandTypeLocally
	CommandTypeHost
	CommandTypeProject
	CommandTypeCache
	CommandTypeSet
	CommandTypeLet
	CommandTypeTry
	CommandTypeWith
	CommandTypeIf
	CommandTypeFor
	CommandTypeWait
)

// commandNames maps CommandType to string representation
var commandNames = map[CommandType]string{
	CommandTypeFrom:           "FROM",
	CommandTypeRun:            "RUN",
	CommandTypeCopy:           "COPY",
	CommandTypeBuild:          "BUILD",
	CommandTypeArg:            "ARG",
	CommandTypeSaveArtifact:   "SAVE ARTIFACT",
	CommandTypeSaveImage:      "SAVE IMAGE",
	CommandTypeCmd:            "CMD",
	CommandTypeEntrypoint:     "ENTRYPOINT",
	CommandTypeExpose:         "EXPOSE",
	CommandTypeVolume:         "VOLUME",
	CommandTypeEnv:            "ENV",
	CommandTypeWorkdir:        "WORKDIR",
	CommandTypeUser:           "USER",
	CommandTypeGitClone:       "GIT CLONE",
	CommandTypeAdd:            "ADD",
	CommandTypeStopsignal:     "STOPSIGNAL",
	CommandTypeOnbuild:        "ONBUILD",
	CommandTypeHealthcheck:    "HEALTHCHECK",
	CommandTypeShell:          "SHELL",
	CommandTypeDo:             "DO",
	CommandTypeCommand:        "COMMAND",
	CommandTypeImport:         "IMPORT",
	CommandTypeVersion:        "VERSION",
	CommandTypeFromDockerfile: "FROM DOCKERFILE",
	CommandTypeLocally:        "LOCALLY",
	CommandTypeHost:           "HOST",
	CommandTypeProject:        "PROJECT",
	CommandTypeCache:          "CACHE",
	CommandTypeSet:            "SET",
	CommandTypeLet:            "LET",
	CommandTypeTry:            "TRY",
	CommandTypeWith:           "WITH",
	CommandTypeIf:             "IF",
	CommandTypeFor:            "FOR",
	CommandTypeWait:           "WAIT",
}

// String returns the string representation of the command type.
func (ct CommandType) String() string {
	if name, ok := commandNames[ct]; ok {
		return name
	}
	return "UNKNOWN"
}
