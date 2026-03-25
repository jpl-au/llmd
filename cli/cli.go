// Package cli provides the core commands as a compiled extension.
//
// Each command is a thin wrapper around the SDK - it parses flags, calls
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

// Commands returns the full command table. Each command's spec is defined
// in its own file (e.g. exportSpec in export.go, catSpec in cat.go).
func (c *CLI) Commands() []sdk.Command {
	return []sdk.Command{
		catSpec, lsSpec, writeSpec, rmSpec, mvSpec, editSpec, sedSpec,
		grepSpec, findSpec, globSpec, historySpec, diffSpec, restoreSpec, revertSpec,
		tagSpec, linkSpec, unlinkSpec,
		importSpec, exportSpec,
		taskSpec, auditSpec, queueSpec, agentSpec,
		statusSpec, reviewSpec,
		versionSpec, configSpec, initSpec, vacuumSpec,
		mcpSpec, serveSpec, mirrorSpec, pluginsSpec, guideSpec, llmSpec,
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
	case "audit":
		return auditCmd(ctx, args)
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
	case "queue":
		return queueCmd(ctx, args)
	case "agent":
		return agentCmd(ctx, args)
	default:
		return nil, fmt.Errorf("%w: %s", sdk.ErrUnknownCmd, cmd)
	}
}
