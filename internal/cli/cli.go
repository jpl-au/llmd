package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/version"
)

// Exit codes per CLI specification.
const (
	ExitSuccess = 0
	ExitError   = 1
	ExitUsage   = 2
)

// CLI is the command-line interface handler.
type CLI struct {
	host   *host.Host
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// New creates a new CLI instance.
func New(h *host.Host) *CLI {
	return &CLI{
		host:   h,
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// Run parses arguments and executes the appropriate command.
//
// Returns an exit code: 0 for success, 1 for command errors, 2 for usage errors.
func (c *CLI) Run(ctx context.Context, args []string) int {
	// First pass: parse without command flags to get command name
	result, err := Parse(args, nil)
	if err != nil {
		c.writeError(err, OutputText)
		return ExitUsage
	}

	// No command → root help
	if result.Command == "" {
		fmt.Fprint(c.stdout, RootHelp(c.host.Commands()))
		return ExitSuccess
	}

	// Check if command exists (built-in or plugin)
	cmd := c.host.Commands()[result.Command]
	isBuiltin := IsBuiltin(result.Command)

	if cmd == nil && !isBuiltin {
		c.writeError(fmt.Errorf("unknown command: %s", result.Command), result.Output)
		return ExitUsage
	}

	// Re-parse with command flags if it's a plugin command
	if cmd != nil {
		result, err = Parse(args, cmd.Flags)
		if err != nil {
			c.writeError(err, result.Output)
			return ExitUsage
		}
	}

	// Handle --help for the command
	if result.Help {
		if isBuiltin {
			fmt.Fprint(c.stdout, BuiltinHelp(result.Command))
		} else {
			fmt.Fprint(c.stdout, CommandHelp(cmd))
		}
		return ExitSuccess
	}

	// Execute built-in commands
	if isBuiltin {
		return c.runBuiltin(ctx, result)
	}

	// Load config
	cfg, _ := config.Load() // Errors are non-fatal (file may not exist)

	// Determine author: flag > config > error
	author := result.Author
	if author == "" && cfg != nil {
		author = cfg.Author
	}
	if author == "" {
		c.writeError(errors.New("author not configured\n\nSet your author name:\n  llmd config author \"Your Name\"           # global (all projects)\n  llmd config --local author \"Your Name\"   # local (this project only)"), result.Output)
		return ExitUsage
	}

	// Read stdin if piped
	var stdin []byte
	if f, ok := c.stdin.(*os.File); ok {
		if stat, err := f.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			stdin, _ = io.ReadAll(f)
		}
	}

	// Execute plugin command
	output, err := c.host.ExecuteCommand(ctx, result.Command, result.Args, result.Flags, stdin, author)
	if err != nil {
		c.writeError(err, result.Output)
		return ExitError
	}

	fmt.Fprintln(c.stdout, output)
	return ExitSuccess
}

// runBuiltin executes a built-in command.
func (c *CLI) runBuiltin(ctx context.Context, result *ParseResult) int {
	switch result.Command {
	case "version":
		version.Print()
		return ExitSuccess

	case "plugins":
		return c.runPlugins(ctx, result)

	case "config":
		return c.runConfig(ctx, result)

	default:
		c.writeError(fmt.Errorf("unknown built-in command: %s", result.Command), result.Output)
		return ExitUsage
	}
}

// writeError writes an error to stderr in the appropriate format.
func (c *CLI) writeError(err error, format OutputFormat) {
	switch format {
	case OutputJSON:
		json.NewEncoder(c.stderr).Encode(map[string]string{"error": err.Error()})
	default:
		fmt.Fprintf(c.stderr, "error: %v\n", err)
	}
}
