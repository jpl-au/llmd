// Package cli provides the core commands as a compiled extension.
//
// Each command is a thin wrapper around the SDK — it parses flags, calls
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

// CLI implements [sdk.Plugin] and [extension.Extension] to provide the
// core command set. It registers itself at init time via
// [extension.Register] and provides all built-in commands: document
// CRUD, search, tags, links, tasks, bulk operations, and administrative
// commands. The CLI is the only compiled extension that ships with llmd.
type CLI struct{}

func (c *CLI) Name() string       { return "cli" }
func (c *CLI) Plugin() sdk.Plugin { return c }

// NoStoreCommands returns commands that can run without an open store.
func (c *CLI) NoStoreCommands() []string {
	return []string{"version", "config", "init", "plugins", "guide", "llm"}
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
			{Name: "tree", Type: "bool", Desc: "Render as directory tree"},
		}},
		{Name: "write", Desc: "Write a document", Usage: "write <path>", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
			{Name: "message", Type: "string", Desc: "Version message"},
		}},
		{Name: "rm", Desc: "Delete a document", Usage: "rm <path>", MCP: true, NeedsAuthor: true},
		{Name: "mv", Desc: "Move or rename a document", Usage: "mv <from> <to>", MCP: true, NeedsAuthor: true},
		{Name: "edit", Desc: "Edit a document via search/replace", Usage: "edit <path> <old> <new>", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
			{Name: "message", Type: "string", Desc: "Version message"},
		}},
		{Name: "sed", Desc: "sed-style substitution", Usage: "sed [-i] 's/old/new/' <path>", MCP: true, NeedsAuthor: true},
		{Name: "grep", Desc: "Search documents", Usage: "grep [options] <pattern> [path]", MCP: true, MCPName: "llmd_grep", Flags: []sdk.Flag{
			{Name: "n", Type: "bool", Desc: "Show line numbers"},
			{Name: "l", Type: "bool", Desc: "Show only filenames"},
			{Name: "c", Type: "bool", Desc: "Show match count only"},
			{Name: "C", Type: "int", Desc: "Lines of context"},
		}},
		{Name: "find", Desc: "Full-text search (paths only)", Usage: "find <query> [path]", MCP: true, MCPName: "llmd_find"},
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

		// Tags and links
		{Name: "tag", Desc: "Manage document tags", Usage: "tag [options] [path] [name]", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
			{Name: "delete", Short: "d", Type: "bool", Desc: "Remove a tag"},
			{Name: "find", Short: "f", Type: "bool", Desc: "Find documents with tag"},
		}},
		{Name: "link", Desc: "Create links between documents", Usage: "link [options] <from> [to]", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
			{Name: "label", Type: "string", Desc: "Link label"},
			{Name: "in", Type: "bool", Desc: "Show incoming links"},
		}},
		{Name: "unlink", Desc: "Remove document links", Usage: "unlink <from> <to>", MCP: true, NeedsAuthor: true},

		// Bulk operations
		{Name: "import", Desc: "Bulk import from filesystem", Usage: "import [options] <dir>", NeedsAuthor: true, Flags: []sdk.Flag{
			{Name: "prefix", Type: "string", Desc: "Target path prefix"},
			{Name: "dry-run", Type: "bool", Desc: "Preview without importing"},
			{Name: "force", Type: "bool", Desc: "Import even if unchanged"},
		}},
		{Name: "export", Desc: "Export documents to filesystem", Usage: "export [options] <prefix> <dir>", Flags: []sdk.Flag{
			{Name: "overwrite", Type: "bool", Desc: "Overwrite existing files"},
		}},

		// Tasks
		{Name: "task", Desc: `Manage tasks on the board.

Subcommands (passed as first arg):
  add <title>               create task (body via content/stdin)
  list                      board view (all columns)
  show <id>                 task metadata + spec body
  move <id> <column>        move task to column
  set <id> [flags]          update metadata
  rm <id>                   soft-delete task
  restore <id>              restore deleted task
  column list               list columns
  column add <name>         add column
  column rm <name>          remove empty column
  column mv <name> --after  reorder column
  link <id> <path>          link task to document
  links <id>                list linked documents
  log <id> [-n limit]       audit history for a task
  start <id>                start task (record branch, move to in-progress)
  finish [id]               complete task (move to done, show summary)
  branch <id>               create git branch from task, checkout, start
  diff [id]                 show git diff for task's branch
  files [id]                list files changed on task's branch
  commits [id]              list commits on task's branch`, Usage: "task <subcommand> [options]", MCP: true, MCPName: "task", NeedsAuthor: true, Flags: []sdk.Flag{
			{Name: "column", Type: "string", Desc: "Filter by column"},
			{Name: "priority", Type: "int", Desc: "Filter or set priority"},
			{Name: "assign", Type: "string", Desc: "Filter or set assigned to"},
			{Name: "branch", Type: "string", Desc: "Git branch for this task"},
			{Name: "path", Type: "string", Desc: "Use existing store document as spec"},
			{Name: "file", Type: "string", Desc: "Read spec from filesystem path"},
			{Name: "flag", Type: "string", Desc: "Set a flag (blocked, hold)"},
			{Name: "unflag", Type: "string", Desc: "Remove a flag"},
			{Name: "position", Type: "int", Desc: "Set position within column"},
			{Name: "after", Type: "string", Desc: "Insert/move column after this one"},
			{Name: "base", Type: "string", Desc: "Base branch for diff (default: main/master)"},
			{Name: "stat", Type: "bool", Desc: "Show diffstat instead of full diff"},
		}},

		// Views
		{Name: "status", Desc: "Store overview dashboard", Usage: "status [-n limit]", Flags: []sdk.Flag{
			{Name: "n", Type: "int", Desc: "Items per section (default 5)"},
		}},
		{Name: "review", Desc: "Review pending tasks with context", Usage: "review [--column name] [-n limit]", Flags: []sdk.Flag{
			{Name: "column", Type: "string", Desc: "Filter by column"},
			{Name: "n", Type: "int", Desc: "Maximum tasks to show"},
		}},

		// Admin and help
		{Name: "version", Desc: "Show version information", Usage: "version"},
		{Name: "config", Desc: "Manage configuration", Usage: "config [key] [value]"},
		{Name: "init", Desc: "Initialise a new store", Usage: "init"},
		{Name: "vacuum", Desc: "Clean up deleted documents", Usage: "vacuum"},
		{Name: "mcp", Desc: "Start MCP stdio server", Usage: "mcp"},
		{Name: "serve", Desc: "Start HTTP API server (coming soon)", Usage: "serve"},
		{Name: "mirror", Desc: "Mirror documents to filesystem", Usage: "mirror [prefix]"},
		{Name: "plugins", Desc: "List loaded plugins", Usage: "plugins"},
		{Name: "guide", Desc: "Built-in documentation", Usage: "guide [--raw] [topic]", MCP: true, Flags: []sdk.Flag{
			{Name: "raw", Type: "bool", Desc: "Output raw markdown without rendering"},
		}},
		{Name: "llm", Desc: "Quick command reference for LLMs", Usage: "llm", MCP: true},
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
	case "sed":
		return sed(ctx, args)
	case "grep":
		return grep(ctx, args)
	case "find":
		return find(ctx, args)
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
	case "tag":
		return tag(ctx, args)
	case "link":
		return linkCmd(ctx, args)
	case "unlink":
		return unlink(ctx, args)
	case "task":
		return taskCmd(ctx, args)
	case "status":
		return status(ctx, args)
	case "review":
		return review(ctx, args)
	case "import":
		return importCmd(ctx, args)
	case "export":
		return exportCmd(ctx, args)
	case "version":
		return versionCmd(ctx, args)
	case "config":
		return configCmd(ctx, args)
	case "init":
		return initCmd(ctx, args)
	case "vacuum":
		return vacuumCmd(ctx, args)
	case "mcp":
		return mcpCmd(ctx, args)
	case "serve":
		return serve(ctx, args)
	case "plugins":
		return pluginsCmd(ctx, args)
	case "mirror":
		return mirror(ctx, args)
	case "guide":
		return guideCmd(ctx, args)
	case "llm":
		return llm(ctx, args)
	default:
		return nil, fmt.Errorf("%w: %s", sdk.ErrUnknownCmd, cmd)
	}
}
