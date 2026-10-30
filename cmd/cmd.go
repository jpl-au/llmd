// Package cmd runs the llmd command line. Execute parses global flags,
// loads configuration, sets up logging and telemetry, and delegates to
// the host for store lifecycle, author resolution and dispatch.
//
// All commands live in extensions, so a binary chooses its commands
// through blank imports. The cli package registers the built-in set. A
// build that adds its own extension has the same shape as llmd's main:
//
//	package main
//
//	import (
//		"os"
//
//		"github.com/jpl-au/llmd/cmd"
//		_ "github.com/jpl-au/llmd/cli"
//		_ "example.com/myext"
//	)
//
//	func main() { os.Exit(cmd.Execute(os.Args[1:])) }
package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"log/slog"

	"github.com/jpl-au/llmd/app"
	"github.com/jpl-au/llmd/config"
	"github.com/jpl-au/llmd/host"
	"github.com/jpl-au/llmd/sdk"
	"github.com/jpl-au/llmd/telemetry"
	"github.com/jpl-au/llmd/term"
)

// Execute runs the command named in args, which exclude the program
// name, and returns the process exit code. It returns rather than
// calling os.Exit so deferred cleanup runs and the store closes.
func Execute(args []string) int {
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
	if cfg.Display.Theme == "light" {
		markdownTheme = "light"
	}
	if cfgErr != nil {
		slog.Warn("reading config", "err", cfgErr)
	}
	// A value the config cannot use is reported once; the command runs
	// with that key's default.
	if err := cfg.Validate(); err != nil {
		for line := range strings.SplitSeq(err.Error(), "\n") {
			slog.Warn("config value ignored, using the default", "problem", line)
		}
	}

	app.Diagnostics = telemetry.Enabled
	telemetry.Init()
	defer telemetry.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// A storeless command that succeeds (init) attaches the store it
	// created to this host so the host is usable afterwards. Close it
	// on the way out so the process never exits with a store open.
	h := host.New()
	defer func() {
		if err := h.Close(); err != nil {
			slog.Warn("closing store", "err", err)
		}
	}()

	// llmd --version (or -v) is the version command.
	if g.Version {
		g.Cmd, g.Args = "version", nil
	}

	program := sdk.Program()
	if g.Cmd == "" {
		printHelp(h, program)
		return 0
	}

	if g.Help {
		cmds := h.Commands()
		if c := cmds[g.Cmd]; c != nil {
			printCmdHelp(c, program)
			return 0
		}
		return errorf(g.JSON, "%s: unknown command %s. Run %s --help to list them.", program, g.Cmd, program)
	}

	// Commands that take over the raw I/O streams (mcp, serve) manage
	// stdin themselves. Every other command gets stdin as a source it
	// reads only if it needs input (write, hook, or task add, audit and
	// queue without a content argument).
	var stdin io.Reader
	cmds := h.Commands()
	if c := cmds[g.Cmd]; c == nil || !c.Streams {
		stdin = term.Stdin()
	}

	result, err := h.Run(ctx, host.RunOpts{
		Cmd:    g.Cmd,
		Args:   g.Args,
		Author: g.Author,
		Stdin:  stdin,
		DB:     g.DB,
	})
	if err != nil {
		// A command that reports another program's exit status (agent
		// run) exits with it.
		if exit, ok := errors.AsType[*sdk.ExitError](err); ok {
			return exit.Code
		}
		// Ctrl-C (or SIGTERM) cancels the command; say so plainly
		// rather than show where the cancellation surfaced.
		if ctx.Err() != nil && errors.Is(err, context.Canceled) {
			return errorf(g.JSON, "%s: interrupted.", program)
		}
		// A command missing an argument says which, then shows its
		// usage. Under --json the error alone is the output.
		code := errorf(g.JSON, "%v", err)
		if errors.Is(err, sdk.ErrMissingArg) && !g.JSON {
			if c := cmds[g.Cmd]; c != nil {
				printCmdHelp(c, program)
			}
		}
		return code
	}

	return display(result, g.JSON)
}
