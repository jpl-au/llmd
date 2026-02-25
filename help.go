// help.go renders the root and per-command help output.

package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// commandGroup defines a labelled set of commands for help output.
type commandGroup struct {
	label string
	names []string
}

// helpGroups controls the order and grouping of the root help listing.
// Commands not listed here are omitted from the root help (they still
// work — they just aren't advertised at the top level).
var helpGroups = []commandGroup{
	{"Reading", []string{"cat", "ls", "grep", "find", "glob"}},
	{"Writing", []string{"write", "edit", "sed", "rm", "mv", "restore", "revert"}},
	{"History", []string{"history", "diff"}},
	{"Tags & Links", []string{"tag", "link", "unlink"}},
	{"Tasks", []string{"task", "status", "review"}},
	{"Bulk", []string{"import", "export", "mirror"}},
	{"Admin", []string{"init", "config", "vacuum", "version", "mcp", "serve", "plugins"}},
	{"Help", []string{"guide", "llm"}},
}

// printHelp displays grouped top-level usage. Each command shows only
// the first line of its description. "Plugin Commands:" lists yaegi
// plugins (only shown if any are loaded).
func printHelp(h *host.Host) {
	fmt.Print(`llmd - a document store for LLMs and humans

Usage:
  llmd <command> [flags] [args...]

Global Flags:
  --db <path>         Use a specific database file
  --json              Output as JSON
  --help              Show help

`)
	cmds := h.ExtCommands()
	for _, g := range helpGroups {
		fmt.Printf("%s:\n", g.label)
		for _, name := range g.names {
			c, ok := cmds[name]
			if !ok {
				continue
			}
			desc := c.Desc
			if i := strings.IndexByte(desc, '\n'); i >= 0 {
				desc = desc[:i]
			}
			fmt.Printf("  %-12s%s\n", name, desc)
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
			desc := pcmds[name].Desc
			if i := strings.IndexByte(desc, '\n'); i >= 0 {
				desc = desc[:i]
			}
			fmt.Printf("  %-12s%s\n", name, desc)
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
	fmt.Println("  --db <path>         Use a specific database file")
	fmt.Println("  --json              Output as JSON")
	fmt.Println("  --help              Show help")
}
