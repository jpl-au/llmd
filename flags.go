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
	Version bool
	DB      string
	Author  string
	Cmd     string
	Args    []string
}

// parseGlobal extracts global flags from the raw argument list. They
// may come before the command or after it, in either --name value or
// --name=value form, and mean the same in both places. After the
// command, "--" ends flag parsing: everything from it on is the
// command's own. Before the command, an unknown flag is an error
// rather than being dropped.
//
// --version and -v are global only before the command (llmd
// --version): after it they belong to the command, as in cat
// --version 3.
func parseGlobal(args []string) (globalFlags, error) {
	var g globalFlags

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			g.Cmd = arg
			rest, err := stripGlobal(args[i+1:], &g)
			g.Args = rest
			return g, err
		}
		if arg == "--version" || arg == "-v" {
			g.Version = true
			continue
		}
		n, err := g.take(args, i)
		if err != nil {
			return g, err
		}
		if n == 0 {
			return g, fmt.Errorf("llmd: unknown flag %s. The global flags are --author, --json, --db, --verbose and --help.", arg)
		}
		i += n - 1
	}

	return g, nil
}

// stripGlobal removes global flags from per-command args so they
// don't reach per-command ParseArgs (which would reject them as
// unknown). It stops at "--", passing it and what follows through.
func stripGlobal(raw []string, g *globalFlags) ([]string, error) {
	out := make([]string, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] == "--" {
			return append(out, raw[i:]...), nil
		}
		n, err := g.take(raw, i)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			out = append(out, raw[i])
			continue
		}
		i += n - 1
	}
	return out, nil
}

// take reads the global flag at args[i], if it is one, and returns how
// many arguments it used: 0 when args[i] is not a global flag, 2 when
// its value is the next argument.
func (g *globalFlags) take(args []string, i int) (int, error) {
	arg := args[i]
	switch arg {
	case "--json":
		g.JSON = true
		return 1, nil
	case "--help", "-h":
		g.Help = true
		return 1, nil
	case "--verbose":
		g.Verbose = true
		return 1, nil
	}

	name, value, inline := strings.Cut(arg, "=")
	var dst *string
	var what string
	switch name {
	case "--db":
		dst, what = &g.DB, "a path"
	case "--author":
		dst, what = &g.Author, "a name"
	default:
		return 0, nil
	}
	if inline {
		if value == "" {
			return 0, fmt.Errorf("llmd: %s needs %s.", name, what)
		}
		*dst = value
		return 1, nil
	}
	if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
		return 0, fmt.Errorf("llmd: %s needs %s.", name, what)
	}
	*dst = args[i+1]
	return 2, nil
}
