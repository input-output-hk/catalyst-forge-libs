package earthfile

import (
	"fmt"
	"strings"
)

// GetFlag returns the value of the specified flag and whether it was found.
// Flags are expected to start with -- (e.g., --mount, --from).
func (c *Command) GetFlag(name string) (string, bool) {
	prefix := "--" + name
	for _, arg := range c.Args {
		if !strings.HasPrefix(arg, "--") {
			continue
		}

		// Handle --flag=value format
		if strings.HasPrefix(arg, prefix+"=") {
			return strings.TrimPrefix(arg, prefix+"="), true
		}

		// Handle --flag format (no value)
		if arg == prefix {
			return "", true
		}
	}
	return "", false
}

// GetPositionalArgs returns all non-flag arguments.
// Arguments after -- are considered positional even if they start with --.
func (c *Command) GetPositionalArgs() []string {
	var positional []string
	foundDoubleDash := false

	for _, arg := range c.Args {
		if arg == "--" {
			foundDoubleDash = true
			continue
		}

		if foundDoubleDash || !strings.HasPrefix(arg, "--") {
			positional = append(positional, arg)
		}
	}

	return positional
}

// IsRemoteReference returns true if the command references a remote target.
// Remote references typically contain domain names or protocols.
func (c *Command) IsRemoteReference() bool {
	if len(c.Args) == 0 {
		return false
	}

	firstArg := c.Args[0]

	// Check for protocol prefix
	if strings.HasPrefix(firstArg, "http://") || strings.HasPrefix(firstArg, "https://") {
		return true
	}

	// Check for domain-like patterns (e.g., github.com/...)
	if strings.Contains(firstArg, ".com/") || strings.Contains(firstArg, ".org/") ||
		strings.Contains(firstArg, ".net/") || strings.Contains(firstArg, ".io/") {
		return true
	}

	// Check if it starts with a domain name pattern
	parts := strings.Split(firstArg, "/")
	if len(parts) >= 2 {
		firstPart := parts[0]
		// Domain names contain dots but don't start with dots (which would be local paths)
		if strings.Contains(firstPart, ".") && !strings.HasPrefix(firstPart, ".") {
			return true
		}
	}

	return false
}

// GetReference parses a target reference from the command arguments.
// Returns an error if no valid reference is found.
func (c *Command) GetReference() (*Reference, error) {
	if len(c.Args) == 0 {
		return nil, fmt.Errorf("no arguments to parse reference from")
	}

	arg := c.Args[0]

	// Remove artifact suffix if present (e.g., +artifact/file.txt)
	if idx := strings.Index(arg, "+"); idx > 0 {
		arg = arg[:idx]
	}

	// Check if this looks like a docker image (not an Earthfile reference)
	// Docker images typically don't have path separators before the colon
	if !strings.Contains(arg, "/") && strings.Contains(arg, ":") {
		// This looks like "alpine:3.18" rather than a reference
		parts := strings.Split(arg, ":")
		if len(parts) == 2 && !strings.HasPrefix(parts[0], ".") && !strings.HasPrefix(parts[0], "+") {
			// It's likely a docker image, not a reference
			return nil, fmt.Errorf("not an Earthfile reference: %s", arg)
		}
	}

	ref := &Reference{}

	// Parse the reference
	if strings.HasPrefix(arg, "+") {
		// Current directory reference (e.g., +test)
		ref.Target = strings.TrimPrefix(arg, "+")
		ref.Local = true
		ref.Remote = false
		ref.Path = "."
	} else if idx := strings.LastIndex(arg, ":"); idx > 0 {
		// Has explicit target (e.g., ./lib:build or github.com/repo:target)
		ref.Path = arg[:idx]
		ref.Target = arg[idx+1:]

		// Determine if local or remote
		if c.IsRemoteReference() {
			ref.Local = false
			ref.Remote = true
		} else {
			ref.Local = true
			ref.Remote = false
		}
	} else {
		// No colon means no valid reference
		return nil, fmt.Errorf("invalid reference format: %s", arg)
	}

	return ref, nil
}

// SourceLocation returns the source location of this command.
func (c *Command) SourceLocation() *SourceLocation {
	return c.Location
}
