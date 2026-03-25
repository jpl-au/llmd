// llmd is a versioned document store for LLMs and humans.
//
// This file is a thin dispatcher. All commands live in extensions
// (cli/ package). main.go parses global flags and delegates to the
// host for store lifecycle, author resolution, and dispatch.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"github.com/jpl-au/llmd/app"
	_ "github.com/jpl-au/llmd/cli"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/telemetry"
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

	// A long-running server should default to Info-level logging so
	// startup and request events are visible.
	if g.Cmd == "serve" && cfg.Log.Level == "" && !g.Verbose {
		cfg.Log.Level = "info"
	}

	initLog(cfg, g.JSON, g.Verbose)
	if cfgErr != nil {
		slog.Warn("reading config", "err", cfgErr)
	}

	app.Diagnostics = telemetry.Enabled
	telemetry.Init()
	defer telemetry.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	h := host.New()

	if g.Cmd == "" {
		printHelp(h)
		return 0
	}

	if g.Help {
		cmds := h.Commands()
		if c := cmds[g.Cmd]; c != nil {
			printCmdHelp(c)
			return 0
		}
		return errorf(g.JSON, "unknown command: %s", g.Cmd)
	}

	// Commands that take over the raw I/O streams (mcp, serve) manage
	// stdin themselves. The host only pre-reads stdin for one-shot
	// commands that accept piped input (write, edit).
	var stdin []byte
	cmds := h.Commands()
	if c := cmds[g.Cmd]; c == nil || !c.Streams {
		stdin = term.ReadStdin()
	}

	result, err := h.Run(ctx, host.RunOpts{
		Cmd:    g.Cmd,
		Args:   g.Args,
		Author: g.Author,
		Stdin:  stdin,
		DB:     g.DB,
	})
	if err != nil {
		// If a command fails because it needs arguments, show help
		// instead of a bare error. The user clearly needs guidance.
		if errors.Is(err, sdk.ErrMissingArg) {
			if c := cmds[g.Cmd]; c != nil {
				printCmdHelp(c)
				return 1
			}
		}
		return errorf(g.JSON, "%v", err)
	}

	return display(result, g.JSON)
}
