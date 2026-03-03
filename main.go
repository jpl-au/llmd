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
		case cmd == "" && !strings.HasPrefix(arg, "-"):
			cmd = arg
			cmdArgs = args[i+1:]
			// Scan remaining args for --help/--json/--verbose (so "llmd cat --help" works).
			for _, a := range cmdArgs {
				switch a {
				case "--help", "-h":
					help = true
				case "--json":
					jsonOut = true
				case "--verbose":
					verbose = true
				}
			}
			i = len(args)
		}
	}

	initLog(config.Load(), jsonOut, verbose)

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

	author := sdk.Config.Read()["author"]
	if author == "" && c.NeedsAuthor {
		return errorf(jsonOut, "author not configured\n\nSet your author name:\n  llmd config author \"Your Name\"")
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
