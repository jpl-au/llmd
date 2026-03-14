// llmd is a versioned document store for LLMs and humans.
//
// This file is a thin dispatcher. All commands live in extensions
// (cli/ package). main.go handles orchestration: flag parsing, store
// lifecycle, author validation, and dispatch.
package main

import (
	"context"
	"os"
	"os/signal"
	"slices"

	"log/slog"

	_ "github.com/jpl-au/llmd/cli"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/term"
	"github.com/jpl-au/llmd/sdk"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is the real entry point, separated from main() so it can return
// an exit code instead of calling os.Exit directly.
func run(args []string) int {
	g, err := parseGlobal(args)
	if err != nil {
		return errorf(false, "%v", err)
	}

	cfg, cfgErr := config.Load()
	initLog(cfg, g.JSON, g.Verbose)
	if cfgErr != nil {
		slog.Warn("reading config", "err", cfgErr)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if g.Cmd == "" {
		printHelp(host.New())
		return 0
	}

	// Check if command needs a store by consulting Storeless extensions.
	needsStore := true
	for _, ext := range extension.All() {
		if sl, ok := ext.(extension.Storeless); ok {
			if slices.Contains(sl.NoStoreCommands(), g.Cmd) {
				needsStore = false
			}
		}
	}

	var h *host.Host
	if needsStore {
		var err error
		h, err = host.Open(g.DB)
		if err != nil {
			if g.DB == "" {
				return errorf(g.JSON, "%v (run 'llmd init' first)", err)
			}
			return errorf(g.JSON, "%v", err)
		}
		defer h.Close()
	} else {
		h = host.New()
	}

	cmds := h.Commands()
	c := cmds[g.Cmd]
	if c == nil {
		return errorf(g.JSON, "unknown command: %s", g.Cmd)
	}

	if g.Help {
		printCmdHelp(c)
		return 0
	}

	// Resolve author. --author flag takes precedence over config.
	// Config author is only used for interactive terminals —
	// non-interactive callers (LLMs, scripts) must always use
	// --author so mutations are correctly attributed.
	author := g.Author
	if author == "" && term.Interactive() {
		authorCfg, err := sdk.Config.Read()
		if err != nil {
			slog.Warn("reading config for author", "err", err)
		}
		author = authorCfg["author"]
	}

	if c.NeedsAuthor && author == "" {
		if term.Interactive() {
			return errorf(g.JSON, "author not configured\n\nSet your author name:\n  llmd config author \"Your Name\"\n\nOr pass --author on the command line:\n  llmd --author \"Name\" %s ...", g.Cmd)
		}
		return errorf(g.JSON, "--author is required for non-interactive use\n\nLLMs and scripts must identify themselves:\n  llmd --author \"Claude\" %s ...", g.Cmd)
	}

	stdin := term.ReadStdin()

	result, err := h.Exec(ctx, g.Cmd, g.Args, author, stdin, g.DB)
	if err != nil {
		return errorf(g.JSON, "%v", err)
	}

	return display(result, g.JSON)
}
