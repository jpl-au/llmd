// Package cli provides core commands as an extension.
package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/sdk"
)

func init() {
	extension.Register(&CLI{})
}

// CLI provides core document commands.
type CLI struct{}

// Name returns the extension name.
func (c *CLI) Name() string { return "cli" }

// Plugin returns this extension as an sdk.Plugin.
func (c *CLI) Plugin() sdk.Plugin { return c }

// Commands returns the CLI commands.
func (c *CLI) Commands() []sdk.Command {
	return []sdk.Command{
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
		{Name: "write", Desc: "Write a document", Usage: "write <path>", MCP: true, Flags: []sdk.Flag{
			{Name: "message", Type: "string", Desc: "Version message"},
		}},
		{Name: "rm", Desc: "Delete a document", Usage: "rm <path>", MCP: true},
		{Name: "mv", Desc: "Move or rename a document", Usage: "mv <from> <to>", MCP: true},
		{Name: "edit", Desc: "Edit a document via search/replace", Usage: "edit <path> <old> <new>", MCP: true, Flags: []sdk.Flag{
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
		{Name: "restore", Desc: "Restore a deleted document", Usage: "restore <path>", MCP: true},
		{Name: "revert", Desc: "Revert to a previous version", Usage: "revert <path> <version>", MCP: true, Flags: []sdk.Flag{
			{Name: "message", Type: "string", Desc: "Revert message"},
		}},
	}
}

// Exec executes a command.
func (c *CLI) Exec(ctx sdk.Context, cmd string, args []string) (sdk.Result, error) {
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
	default:
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
}
