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
	"sort"
	"strings"
	"time"

	_ "github.com/jpl-au/llmd/cli"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/llmd"
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
	var cmd string
	var cmdArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--help" || arg == "-h":
			help = true
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
		printHelp(host.New(nil))
		return 0
	}

	// Check if command needs a store by consulting Storeless extensions.
	needsStore := true
	for _, ext := range extension.All() {
		if sl, ok := ext.(extension.Storeless); ok {
			for _, name := range sl.NoStoreCommands() {
				if name == cmd {
					needsStore = false
					break
				}
			}
		}
	}

	var store *llmd.Store
	if needsStore {
		var err error
		store, err = llmd.Open("")
		if err != nil {
			return errorf(jsonOut, "%v", err)
		}
		defer store.Close()
	}

	h := host.New(store)

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

	author := config.Load()["author"]
	if author == "" && c.NeedsAuthor {
		return errorf(jsonOut, "author not configured\n\nSet your author name:\n  llmd config author \"Your Name\"")
	}

	stdin := readStdin()

	result, err := h.Exec(cmd, cmdArgs, author, stdin)
	if err != nil {
		return errorf(jsonOut, "%v", err)
	}

	switch r := result.(type) {
	case sdk.Text:
		if string(r) != "" {
			fmt.Println(string(r))
		}
	case sdk.Result:
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(r.Data)
		} else if r.Text != "" {
			fmt.Println(r.Text)
		}
	case sdk.Data:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r.V)
	}

	return 0
}

// errorf writes an error to stderr (as JSON if --json, plain text otherwise)
// and returns exit code 1.
func errorf(jsonOut bool, format string, args ...any) int {
	msg := fmt.Sprintf(format, args...)
	if jsonOut {
		json.NewEncoder(os.Stderr).Encode(map[string]string{"error": msg})
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

// printHelp displays top-level usage. "Commands:" lists extension commands,
// "Plugin Commands:" lists yaegi plugins (only shown if any are loaded).
func printHelp(h *host.Host) {
	fmt.Print(`llmd - a document store for LLMs and humans

Usage:
  llmd <command> [flags] [args...]

Global Flags:
  --json              Output as JSON
  --help              Show help

`)
	cmds := h.ExtCommands()
	if len(cmds) > 0 {
		fmt.Println("Commands:")
		names := make([]string, 0, len(cmds))
		for name := range cmds {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("  %-12s%s\n", name, cmds[name].Desc)
		}
		fmt.Println()
	}

	pcmds := h.PluginCommands()
	if len(pcmds) > 0 {
		fmt.Println("Plugin Commands:")
		names := make([]string, 0, len(pcmds))
		for name := range pcmds {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("  %-12s%s\n", name, pcmds[name].Desc)
		}
		fmt.Println()
	}

	fmt.Println(`Use "llmd <command> --help" for more information about a command.`)
}

// printCmdHelp displays detailed help for a single command, including
// its flags and global flags.
func printCmdHelp(c *sdk.Command) {
	fmt.Printf("%s - %s\n\n", c.Name, c.Desc)
	if c.Usage != "" {
		fmt.Printf("Usage:\n  llmd %s\n\n", c.Usage)
	}
	if len(c.Flags) > 0 {
		fmt.Println("Flags:")
		for _, f := range c.Flags {
			var flagStr string
			if f.Short != "" {
				flagStr = fmt.Sprintf("-%s, --%s", f.Short, f.Name)
			} else {
				flagStr = fmt.Sprintf("    --%s", f.Name)
			}
			switch f.Type {
			case "string":
				flagStr += " <string>"
			case "int":
				flagStr += " <int>"
			}
			fmt.Printf("  %-24s%s\n", flagStr, f.Desc)
		}
		fmt.Println()
	}
	fmt.Println("Global Flags:")
	fmt.Println("  --json              Output as JSON")
	fmt.Println("  --help              Show help")
}
