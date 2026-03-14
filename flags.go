package main

import (
	"fmt"
	"strings"
)

// globalFlags holds the result of parsing global flags from the
// command line. Global flags are stripped before the remaining args
// reach per-command flag parsing.
type globalFlags struct {
	JSON    bool
	Help    bool
	Verbose bool
	DB      string
	Author  string
	Cmd     string
	Args    []string
}

// parseGlobal extracts global flags from the raw argument list.
// Flags appearing after the command name are also consumed so they
// don't leak into per-command ParseArgs.
func parseGlobal(args []string) (globalFlags, error) {
	var g globalFlags

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			g.JSON = true
		case arg == "--help" || arg == "-h":
			g.Help = true
		case arg == "--verbose":
			g.Verbose = true
		case arg == "--db":
			if i+1 >= len(args) {
				return g, fmt.Errorf("--db requires a path")
			}
			i++
			g.DB = args[i]
		case arg == "--author":
			if i+1 >= len(args) {
				return g, fmt.Errorf("--author requires a name")
			}
			i++
			g.Author = args[i]
		case strings.HasPrefix(arg, "--author="):
			g.Author = strings.TrimPrefix(arg, "--author=")
		case g.Cmd == "" && !strings.HasPrefix(arg, "-"):
			g.Cmd = arg
			g.Args = stripGlobal(args[i+1:], &g)
			return g, nil
		}
	}

	return g, nil
}

// stripGlobal removes global flags from per-command args so they
// don't reach per-command ParseArgs (which would reject them as
// unknown).
func stripGlobal(raw []string, g *globalFlags) []string {
	out := make([]string, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		a := raw[i]
		switch {
		case a == "--help" || a == "-h":
			g.Help = true
		case a == "--json":
			g.JSON = true
		case a == "--verbose":
			g.Verbose = true
		case a == "--author" && i+1 < len(raw):
			i++
			g.Author = raw[i]
		case strings.HasPrefix(a, "--author="):
			g.Author = strings.TrimPrefix(a, "--author=")
		default:
			out = append(out, a)
		}
	}
	return out
}
