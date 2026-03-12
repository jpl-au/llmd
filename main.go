// llmd is a versioned document store for LLMs and humans.
//
// This file is a thin dispatcher. All commands live in extensions
// (cli/ package). main.go handles global flag parsing, store
// lifecycle, author validation, and result display.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	_ "github.com/jpl-au/llmd/cli"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run is the real entry point, separated from main() so it can return
// an exit code instead of calling os.Exit directly.
func run(args []string) int {
	var jsonOut bool
	var help bool
	var verbose bool
	var dbPath string
	var authorFlag string
	var cmd string
	var cmdArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--help" || arg == "-h":
			help = true
		case arg == "--verbose":
			verbose = true
		case arg == "--db":
			if i+1 >= len(args) {
				return errorf(jsonOut, "--db requires a path")
			}
			i++
			dbPath = args[i]
		case arg == "--author":
			if i+1 >= len(args) {
				return errorf(jsonOut, "--author requires a name")
			}
			i++
			authorFlag = args[i]
		case strings.HasPrefix(arg, "--author="):
			authorFlag = strings.TrimPrefix(arg, "--author=")
		case cmd == "" && !strings.HasPrefix(arg, "-"):
			cmd = arg
			cmdArgs = args[i+1:]
			// Scan remaining args for global flags (so "llmd cat --help" works).
			for j := 0; j < len(cmdArgs); j++ {
				a := cmdArgs[j]
				switch {
				case a == "--help" || a == "-h":
					help = true
				case a == "--json":
					jsonOut = true
				case a == "--verbose":
					verbose = true
				case a == "--author" && j+1 < len(cmdArgs):
					j++
					authorFlag = cmdArgs[j]
				case strings.HasPrefix(a, "--author="):
					authorFlag = strings.TrimPrefix(a, "--author=")
				}
			}
			i = len(args)
		}
	}

	cfg, cfgErr := config.Load()
	initLog(cfg, jsonOut, verbose)
	if cfgErr != nil {
		slog.Warn("reading config", "err", cfgErr)
	}

	// Create a root context that cancels on SIGINT (Ctrl+C).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// No command — show help. We create a host without a store just
	// for command discovery (help/plugins listing).
	if cmd == "" {
		printHelp(host.New())
		return 0
	}

	// Check if command needs a store by consulting Storeless extensions.
	needsStore := true
	for _, ext := range extension.All() {
		if sl, ok := ext.(extension.Storeless); ok {
			if slices.Contains(sl.NoStoreCommands(), cmd) {
				needsStore = false
			}
		}
	}

	var h *host.Host
	if needsStore {
		var err error
		h, err = host.Open(dbPath)
		if err != nil {
			if dbPath == "" {
				return errorf(jsonOut, "%v (run 'llmd init' first)", err)
			}
			return errorf(jsonOut, "%v", err)
		}
		defer h.Close()
	} else {
		h = host.New()
	}

	// Validate command exists.
	cmds := h.Commands()
	c := cmds[cmd]
	if c == nil {
		return errorf(jsonOut, "unknown command: %s", cmd)
	}

	// --help for a specific command.
	if help {
		printCmdHelp(c)
		return 0
	}

	// Resolve author. --author flag takes precedence over config.
	// Non-interactive callers (LLMs, scripts) must always use --author
	// so that mutations are correctly attributed.
	author := authorFlag
	if author == "" {
		authorCfg, err := sdk.Config.Read()
		if err != nil {
			slog.Warn("reading config for author", "err", err)
		}
		author = authorCfg["author"]
	}

	if c.NeedsAuthor {
		if author == "" {
			return errorf(jsonOut, "author not configured\n\nSet your author name:\n  llmd config author \"Your Name\"\n\nOr pass --author on the command line:\n  llmd --author \"Name\" %s ...", cmd)
		}
		if authorFlag == "" && !stdoutIsTTY() {
			return errorf(jsonOut, "--author is required for non-interactive use\n\nLLMs and scripts must identify themselves:\n  llmd --author \"Claude\" %s ...\n\nThe config author (%q) is reserved for interactive terminal use.", cmd, author)
		}
	}

	stdin := readStdin()

	result, err := h.Exec(ctx, cmd, cmdArgs, author, stdin, dbPath)
	if err != nil {
		return errorf(jsonOut, "%v", err)
	}

	switch r := result.(type) {
	case sdk.Text:
		if string(r) != "" {
			lipgloss.Println(string(r))
		}
	case sdk.Result:
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(r.Data); err != nil {
				return errorf(false, "encoding JSON: %v", err)
			}
		} else if r.Text != "" {
			lipgloss.Println(r.Text)
		}
	case sdk.Data:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r.V); err != nil {
			return errorf(false, "encoding JSON: %v", err)
		}
	}

	return 0
}

// errorf writes an error to stderr (as JSON if --json, plain text otherwise)
// and returns exit code 1.
func errorf(jsonOut bool, format string, args ...any) int {
	msg := fmt.Sprintf(format, args...)
	if jsonOut {
		_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"error": msg})
	} else {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	}
	return 1
}

// initLog configures the process-wide slog logger. By default the level
// is Warn (quiet CLI). --verbose overrides to Debug. Config keys
// log_level and log_format provide persistent control. --json implies
// JSON-formatted logs so structured output stays machine-readable.
func initLog(cfg map[string]string, jsonOut, verbose bool) {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelDebug
	} else if v, ok := cfg["log_level"]; ok {
		switch v {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}

	format := cfg["log_format"]
	if jsonOut {
		format = "json"
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// stdoutIsTTY reports whether stdout is connected to a terminal.
// Used to distinguish interactive (human) use from non-interactive
// (LLM/script) use when enforcing --author.
func stdoutIsTTY() bool {
	f, err := os.Stdout.Stat()
	if err != nil {
		slog.Debug("cannot stat stdout, assuming non-interactive", "err", err)
		return false
	}
	return f.Mode()&os.ModeCharDevice != 0
}

// readStdin reads piped input if present, or returns nil for interactive
// terminals. For pipes, a 50ms timeout avoids blocking on empty pipes
// (e.g. certain process managers).
func readStdin() []byte {
	f := os.Stdin
	stat, err := f.Stat()
	if err != nil {
		return nil
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return nil
	}
	if stat.Mode().IsRegular() {
		data, _ := io.ReadAll(f)
		return data
	}
	done := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(f)
		done <- data
	}()
	select {
	case data := <-done:
		return data
	case <-time.After(50 * time.Millisecond):
		return nil
	}
}
