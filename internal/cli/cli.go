package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/debug"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/llmd"
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
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// New creates a new CLI instance.
func New() *CLI {
	return &CLI{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// Run parses arguments and executes the appropriate command.
//
// Commands are loaded in phases based on their requirements:
//   - Meta commands (version, config, init): no resources needed
//   - Discovery commands (help, plugins): plugins only, no store
//   - Maintenance commands (vacuum): store only, no plugins
//   - Functional commands (ls, cat, write, etc.): store and plugins
//
// Returns an exit code: 0 for success, 1 for command errors, 2 for usage errors.
func (c *CLI) Run(ctx context.Context, args []string) int {
	result, err := Parse(args)
	if err != nil {
		c.writeError(err, OutputText)
		return ExitUsage
	}

	// Meta commands - no resources needed
	switch result.Command {
	case "version":
		version.Print()
		return ExitSuccess
	case "config":
		if result.Help {
			fmt.Fprint(c.stdout, BuiltinHelp("config"))
			return ExitSuccess
		}
		return c.runConfig(ctx, result)
	case "init":
		if result.Help {
			fmt.Fprint(c.stdout, BuiltinHelp("init"))
			return ExitSuccess
		}
		return c.runInit(ctx, result)
	}

	// Discovery commands - plugins only, no store
	// This includes: no command (root help), --help flag, or "plugins" command
	if result.Command == "" || result.Help || result.Command == "plugins" {
		h, err := host.New(ctx, nil)
		if err != nil {
			c.writeError(err, result.Output)
			return ExitError
		}
		defer h.Close(ctx)
		if err := h.LoadPlugins(ctx); err != nil {
			c.writeError(err, result.Output)
			return ExitError
		}
		return c.runDiscovery(ctx, result, h)
	}

	// Maintenance commands - store only, no plugins
	if result.Command == "vacuum" {
		if result.Help {
			fmt.Fprint(c.stdout, BuiltinHelp("vacuum"))
			return ExitSuccess
		}
		store, err := llmd.Open("")
		if err != nil {
			c.writeError(err, result.Output)
			return ExitError
		}
		defer store.Close()
		return c.runVacuum(ctx, result, store)
	}

	// Functional commands - store and plugins
	debug.Log("Run", "step", "opening store")
	store, err := llmd.Open("")
	if err != nil {
		c.writeError(err, result.Output)
		return ExitError
	}
	defer store.Close()
	debug.Log("Run", "step", "store opened")

	h, err := host.New(ctx, store)
	if err != nil {
		c.writeError(err, result.Output)
		return ExitError
	}
	defer h.Close(ctx)
	debug.Log("Run", "step", "loading plugins")
	if err := h.LoadPlugins(ctx); err != nil {
		c.writeError(err, result.Output)
		return ExitError
	}
	debug.Log("Run", "step", "plugins loaded")

	// Check if command exists
	cmd := h.Commands()[result.Command]
	if cmd == nil {
		c.writeError(fmt.Errorf("unknown command: %s", result.Command), result.Output)
		return ExitUsage
	}
	debug.Log("Run", "step", "command found", "command", result.Command)

	// Author check - only for write commands
	author := c.resolveAuthor(result)
	if author == "" && needsAuthor(result.Command) {
		c.writeError(errors.New("author not configured\n\nSet your author name:\n  llmd config author \"Your Name\"           # global (all projects)\n  llmd config --local author \"Your Name\"   # local (this project only)"), result.Output)
		return ExitUsage
	}

	debug.Log("Run", "step", "calling runCommand")
	return c.runCommand(ctx, result, h, author)
}

// runDiscovery handles discovery commands that need plugins but not the store.
func (c *CLI) runDiscovery(ctx context.Context, result *ParseResult, h *host.Host) int {
	// No command → root help
	if result.Command == "" {
		fmt.Fprint(c.stdout, RootHelp(h.Commands()))
		return ExitSuccess
	}

	// Command --help
	if result.Help {
		if IsBuiltin(result.Command) {
			fmt.Fprint(c.stdout, BuiltinHelp(result.Command))
		} else if cmd := h.Commands()[result.Command]; cmd != nil {
			fmt.Fprint(c.stdout, CommandHelp(cmd))
		} else {
			c.writeError(fmt.Errorf("unknown command: %s", result.Command), result.Output)
			return ExitUsage
		}
		return ExitSuccess
	}

	// plugins command
	if result.Command == "plugins" {
		return c.runPlugins(ctx, result, h)
	}

	return ExitSuccess
}

// runCommand executes a plugin command with full resources.
func (c *CLI) runCommand(ctx context.Context, result *ParseResult, h *host.Host, author string) int {
	debug.Log("runCommand", "command", result.Command)

	// Read stdin if available (not a TTY and has data)
	stdin := c.readStdin()
	debug.Log("runCommand", "stdinSize", len(stdin))

	// Execute plugin command
	debug.Log("runCommand", "step", "executing command")
	resp, err := h.ExecuteCommand(ctx, result.Command, result.Args, nil, stdin, author)
	if err != nil {
		debug.Log("runCommand error", "error", err.Error())
		c.writeError(err, result.Output)
		return ExitError
	}

	debug.Log("runCommand", "step", "command completed")
	// Output based on format flag
	if result.Output == OutputJSON && len(resp.StructuredData) > 0 {
		var obj any
		if err := json.Unmarshal(resp.StructuredData, &obj); err == nil {
			enc := json.NewEncoder(c.stdout)
			enc.SetIndent("", "  ")
			enc.Encode(obj)
		} else {
			fmt.Fprintln(c.stdout, resp.TextOutput)
		}
	} else {
		fmt.Fprintln(c.stdout, resp.TextOutput)
	}
	return ExitSuccess
}

// runInit creates a new store.
func (c *CLI) runInit(ctx context.Context, result *ParseResult) int {
	store, err := llmd.Init("")
	if err != nil {
		c.writeError(err, result.Output)
		return ExitError
	}
	store.Close()
	fmt.Fprintln(c.stdout, "Initialized llmd store in .llmd/")
	return ExitSuccess
}

// runConfig handles the config command.
func (c *CLI) runConfig(ctx context.Context, result *ParseResult) int {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = &config.Config{}
	}

	switch len(result.Args) {
	case 0:
		// Show all config
		return c.showConfig(cfg, result.Output)

	case 1:
		// Show specific key
		key := result.Args[0]
		value, ok := cfg.Get(key)
		if !ok {
			return ExitSuccess
		}
		fmt.Fprintln(c.stdout, value)
		return ExitSuccess

	case 2:
		// Set key=value
		key, value := result.Args[0], result.Args[1]

		var path string
		if result.Local {
			path = config.LocalPath()
		} else {
			var err error
			path, err = config.GlobalPath()
			if err != nil {
				c.writeError(fmt.Errorf("getting global config path: %w", err), result.Output)
				return ExitError
			}
		}

		if err := config.Set(path, key, value); err != nil {
			c.writeError(err, result.Output)
			return ExitError
		}
		return ExitSuccess

	default:
		c.writeError(fmt.Errorf("usage: llmd config [key] [value]"), result.Output)
		return ExitUsage
	}
}

// showConfig displays all config values.
func (c *CLI) showConfig(cfg *config.Config, format OutputFormat) int {
	switch format {
	case OutputJSON:
		data := map[string]string{}
		if cfg.Author != "" {
			data["author"] = cfg.Author
		}
		if cfg.Output != "" {
			data["output"] = cfg.Output
		}
		json.NewEncoder(c.stdout).Encode(data)

	default:
		if cfg.Author != "" {
			fmt.Fprintf(c.stdout, "author=%s\n", cfg.Author)
		}
		if cfg.Output != "" {
			fmt.Fprintf(c.stdout, "output=%s\n", cfg.Output)
		}
	}
	return ExitSuccess
}

// needsAuthor returns true for commands that mutate data.
func needsAuthor(cmd string) bool {
	switch cmd {
	case "write", "edit", "rm", "mv", "tag", "link":
		return true
	}
	return false
}

// resolveAuthor gets author from flag, then config.
func (c *CLI) resolveAuthor(result *ParseResult) string {
	if result.Author != "" {
		return result.Author
	}
	cfg, _ := config.Load()
	if cfg != nil && cfg.Author != "" {
		return cfg.Author
	}
	return ""
}

// writeError writes an error to stderr in the appropriate format.
func (c *CLI) writeError(err error, format OutputFormat) {
	msg := err.Error()
	switch format {
	case OutputJSON:
		json.NewEncoder(c.stderr).Encode(map[string]string{"error": msg})
	default:
		fmt.Fprintf(c.stderr, "error: %s\n", msg)
	}
}

// readStdin reads from stdin if data is available.
//
// Returns nil if stdin is a TTY (interactive terminal) or if no data is
// available within a short timeout. This prevents blocking when running
// under process wrappers that leave stdin open but don't provide input.
func (c *CLI) readStdin() []byte {
	f, ok := c.stdin.(*os.File)
	if !ok {
		return nil
	}

	stat, err := f.Stat()
	if err != nil {
		return nil
	}

	// Skip if stdin is a TTY (interactive terminal)
	if stat.Mode()&os.ModeCharDevice != 0 {
		return nil
	}

	// For regular files, read directly (they have finite size)
	if stat.Mode().IsRegular() {
		data, _ := io.ReadAll(f)
		return data
	}

	// For pipes/fifos, use a timeout to avoid blocking forever
	// Start a goroutine to read, with a short deadline
	done := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(f)
		done <- data
	}()

	select {
	case data := <-done:
		return data
	case <-time.After(50 * time.Millisecond):
		// No data available within timeout - assume no input
		return nil
	}
}
