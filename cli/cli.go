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
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/sdk"
)

func init() {
	extension.Register(&CLI{})
}

// CLI implements [sdk.Extension] to provide the core command set. It
// registers itself at init time via [extension.Register] and provides
// all built-in commands: document CRUD, search, tags, links, tasks,
// bulk operations, and administrative commands.
type CLI struct{}

func (c *CLI) Name() string { return "cli" }

// NoStoreCommands returns commands that can run without an open store.
func (c *CLI) NoStoreCommands() []string {
	return []string{"version", "config", "init", "extensions", "guide", "hook"}
}

// Commands returns the full command table. Each command's spec is defined
// in its own file (e.g. exportSpec in export.go, catSpec in cat.go).
func (c *CLI) Commands() []sdk.Command {
	return []sdk.Command{
		catSpec, lsSpec, writeSpec, rmSpec, mvSpec, editSpec, sedSpec,
		grepSpec, findSpec, globSpec, historySpec, diffSpec, restoreSpec, revertSpec,
		tagSpec, linkSpec, unlinkSpec,
		importSpec, exportSpec,
		taskSpec, auditSpec, queueSpec, agentSpec, ruleSpec,
		graphSpec,
		statusSpec, reviewSpec,
		versionSpec, configSpec, initSpec, vacuumSpec,
		mcpSpec, serveSpec, mirrorSpec, extensionsSpec, guideSpec, hookSpec,
	}
}

// handler is the function behind one command.
type handler func(ctx sdk.Context, args []string) (sdk.Response, error)

// handlers maps each command name to its function. Exec looks the
// command up here, and a test checks that this table and Commands()
// name exactly the same commands, so a command without a handler, or a
// handler without a spec, fails the build rather than a user's call.
var handlers = map[string]handler{
	"cat":        cat,
	"ls":         ls,
	"write":      write,
	"rm":         rm,
	"mv":         mv,
	"edit":       edit,
	"sed":        sed,
	"grep":       grep,
	"find":       find,
	"glob":       glob,
	"history":    historyCmd,
	"diff":       diffCmd,
	"restore":    restore,
	"revert":     revert,
	"tag":        tag,
	"link":       linkCmd,
	"unlink":     unlink,
	"task":       taskCmd,
	"audit":      auditCmd,
	"graph":      graph,
	"status":     status,
	"review":     review,
	"import":     importCmd,
	"export":     exportCmd,
	"version":    versionCmd,
	"config":     configCmd,
	"init":       initCmd,
	"vacuum":     vacuumCmd,
	"mcp":        mcpCmd,
	"serve":      serve,
	"extensions": extensionsCmd,
	"mirror":     mirror,
	"guide":      guideCmd,
	"hook":       hookCmd,
	"queue":      queueCmd,
	"agent":      agentCmd,
	"rule":       ruleCmd,
}

// Exec dispatches a command name to its handler.
func (c *CLI) Exec(ctx sdk.Context, cmd string, args []string) (sdk.Response, error) {
	h, ok := handlers[cmd]
	if !ok {
		return nil, sdk.Errorf(sdk.ErrUnknownCmd, "llmd: unknown command %s. Run llmd --help to list them.", cmd)
	}
	return h(ctx, args)
}
