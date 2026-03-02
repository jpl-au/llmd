// llmd is a versioned document store for LLMs and humans.
//
// This file is a thin dispatcher. All commands live in extensions
// (cli/ package). main.go handles global flag parsing, store
// lifecycle, author validation, and result display.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	_ "github.com/jpl-au/llmd/cli"
	"github.com/jpl-au/llmd/extension"
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
		case arg == "--db":
			if i+1 >= len(args) {
				return errorf(jsonOut, "--db requires a path")
			}
			i++
			dbPath = args[i]
		case cmd == "" && !strings.HasPrefix(arg, "-"):
			cmd = arg
			cmdArgs = args[i+1:]
			// Scan remaining args for --help/--json (so "llmd cat --help" works).
			for _, a := range cmdArgs {
				switch a {
				case "--help", "-h":
					help = true
				case "--json":
					jsonOut = true
				}
			}
			i = len(args)
		}
	}

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

	result, err := h.Exec(cmd, cmdArgs, author, stdin, dbPath)
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
