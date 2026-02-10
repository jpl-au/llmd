// Package cli provides the core commands as a compiled extension.
//
// Each command is a thin wrapper around sdk.API — it parses flags, calls
// the store, and returns both human-readable text and structured data.
// The host decides which to display (text for terminals, data for --json).
//
// Commands parse their own flags rather than relying on the host because
// each command has different positional argument conventions (like Unix
// utilities: "cat <path>...", "grep <pattern> [path]", etc.) that don't
// fit a single parsing model.
//
// All commands are registered at init time via extension.Register,
// following the database/sql.Register convention.
package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/sdk"
)

func init() {
	extension.Register(&CLI{})
}

// CLI implements sdk.Plugin to provide core document commands.
type CLI struct{}

func (c *CLI) Name() string       { return "cli" }
func (c *CLI) Plugin() sdk.Plugin { return c }

// NoStoreCommands returns commands that can run without an open store.
func (c *CLI) NoStoreCommands() []string {
	return []string{"version", "config", "init", "plugins"}
}

// Commands returns the full command table. MCP and MCPName fields control
// which commands are exposed to AI agents via the MCP server.
func (c *CLI) Commands() []sdk.Command {
	return []sdk.Command{
		// Document commands
		{Name: "cat", Desc: "Read a document", Usage: "cat [options] <path>...", MCP: true, Flags: []sdk.Flag{
			{Name: "version", Type: "int", Desc: "Read specific version"},
			{Name: "n", Type: "bool", Desc: "Number output lines"},
		}},
		{Name: "ls", Desc: "List documents", Usage: "ls [prefix]", MCP: true, Flags: []sdk.Flag{
			{Name: "l", Type: "bool", Desc: "Long format with details"},
			{Name: "a", Type: "bool", Desc: "Include deleted documents"},
			{Name: "r", Type: "bool", Desc: "Reverse sort order"},
			{Name: "t", Type: "bool", Desc: "Sort by time (newest first)"},
		}},
		{Name: "write", Desc: "Write a document", Usage: "write <path>", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
			{Name: "message", Type: "string", Desc: "Version message"},
		}},
		{Name: "rm", Desc: "Delete a document", Usage: "rm <path>", MCP: true, NeedsAuthor: true},
		{Name: "mv", Desc: "Move or rename a document", Usage: "mv <from> <to>", MCP: true, NeedsAuthor: true},
		{Name: "edit", Desc: "Edit a document via search/replace", Usage: "edit <path> <old> <new>", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
			{Name: "message", Type: "string", Desc: "Version message"},
		}},
		{Name: "grep", Desc: "Search documents", Usage: "grep [options] <pattern> [path]", MCP: true, MCPName: "llmd_grep", Flags: []sdk.Flag{
			{Name: "n", Type: "bool", Desc: "Show line numbers"},
			{Name: "l", Type: "bool", Desc: "Show only filenames"},
			{Name: "c", Type: "bool", Desc: "Show match count only"},
			{Name: "C", Type: "int", Desc: "Lines of context"},
		}},
		{Name: "glob", Desc: "Find documents by path pattern", Usage: "glob <pattern>", MCP: true, MCPName: "llmd_glob"},
		{Name: "history", Desc: "Show version history", Usage: "history [-n limit] <path>", MCP: true, Flags: []sdk.Flag{
			{Name: "n", Type: "int", Desc: "Maximum versions to show"},
		}},
		{Name: "diff", Desc: "Compare documents", Usage: "diff <source> [target]", MCP: true, Flags: []sdk.Flag{
			{Name: "C", Type: "int", Desc: "Lines of context"},
			{Name: "stat", Type: "bool", Desc: "Show stats only"},
		}},
		{Name: "restore", Desc: "Restore a deleted document", Usage: "restore <path>", MCP: true, NeedsAuthor: true},
		{Name: "revert", Desc: "Revert to a previous version", Usage: "revert <path> <version>", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
			{Name: "message", Type: "string", Desc: "Revert message"},
		}},

		// Former built-in commands
		{Name: "version", Desc: "Show version information", Usage: "version"},
		{Name: "config", Desc: "Manage configuration", Usage: "config [key] [value]", Flags: []sdk.Flag{
			{Name: "key", Type: "string", Desc: "Config key to get or set"},
		}},
		{Name: "init", Desc: "Initialize a new store", Usage: "init"},
		{Name: "vacuum", Desc: "Clean up deleted documents", Usage: "vacuum"},
		{Name: "mcp", Desc: "Start MCP stdio server", Usage: "mcp"},
		{Name: "plugins", Desc: "List loaded plugins", Usage: "plugins"},
	}
}

// Exec dispatches a command name to its handler.
func (c *CLI) Exec(ctx sdk.Context, cmd string, args []string) (sdk.Response, error) {
	switch cmd {
	case "cat":
		return cat(ctx, args)
	case "ls":
		return ls(ctx, args)
	case "write":
		return write(ctx, args)
	case "rm":
		return rm(ctx, args)
	case "mv":
		return mv(ctx, args)
	case "edit":
		return edit(ctx, args)
	case "grep":
		return grep(ctx, args)
	case "glob":
		return glob(ctx, args)
	case "history":
		return historyCmd(ctx, args)
	case "diff":
		return diffCmd(ctx, args)
	case "restore":
		return restore(ctx, args)
	case "revert":
		return revert(ctx, args)
	case "version":
		return version(ctx, args)
	case "config":
		return configCmd(ctx, args)
	case "init":
		return initCmd(ctx, args)
	case "vacuum":
		return vacuumCmd(ctx, args)
	case "mcp":
		return mcpCmd(ctx, args)
	case "plugins":
		return pluginsCmd(ctx, args)
	default:
		return nil, fmt.Errorf("%w: %s", sdk.ErrUnknownCmd, cmd)
	}
}
