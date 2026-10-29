// help.go renders the root and per-command help output.

package main

import (
	"fmt"
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
// work - they just aren't advertised at the top level).
var helpGroups = []commandGroup{
	{"Reading", []string{"cat", "ls", "grep", "find", "glob"}},
	{"Writing", []string{"write", "edit", "sed", "rm", "mv", "restore", "revert"}},
	{"History", []string{"history", "diff", "graph"}},
	{"Tags & Links", []string{"tag", "link", "unlink"}},
	{"Tasks", []string{"task", "agent", "rule", "hook", "status", "review"}},
	{"Audits & Queue", []string{"audit", "queue"}},
	{"Bulk", []string{"import", "export", "mirror"}},
	{"Admin", []string{"init", "config", "vacuum", "version", "mcp", "serve", "extensions"}},
	{"Help", []string{"guide"}},
}

// printHelp displays grouped top-level usage. Each command shows only
// the first line of its description.
func printHelp(h *host.Host) {
	fmt.Print(`llmd - a versioned document store with task boards and agent orchestration

AI agents: run "llmd guide" to get started.

Usage:
  llmd <command> [flags] [args...]

Global Flags:
  --author <name>     Who is making the change (required for agents and scripts)
  --db <path>         Use a specific database file
  --json              Output as JSON
  --verbose           Debug logging on stderr
  --help              Show help
  --version           Show version information

`)
	cmds := h.Commands()
	for _, g := range helpGroups {
		fmt.Printf("%s:\n", g.label)
		for _, name := range g.names {
			c, ok := cmds[name]
			if !ok {
				continue
			}
			desc := c.Summary()
			if i := strings.IndexByte(desc, '\n'); i >= 0 {
				desc = desc[:i]
			}
			fmt.Printf("  %-12s%s\n", name, desc)
		}
		fmt.Println()
	}

	fmt.Println(`Use "llmd <command> --help" for more information about a command.`)
}

// printCmdHelp displays help for a single command: the full form for
// a person at a terminal, the agent form when stdout is a pipe, as it
// is for a model running llmd in a shell.
func printCmdHelp(c *sdk.Command) {
	fmt.Print(cmdHelp(c, isTTY()))
}

// cmdHelp renders a command's help. For a terminal it is the whole
// description, the usage line, the flags with their types, and the
// global flags. For a pipe it is the command's AgentHelp, the same
// text the MCP tool list carries, followed by the global flags an
// agent on the command line needs.
func cmdHelp(c *sdk.Command, tty bool) string {
	var b strings.Builder
	if !tty {
		b.WriteString(c.AgentHelp())
		b.WriteString("\nGlobal: --author <name> (required to change anything), --json (structured output), --db <path>, --verbose\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%s - %s\n\n", c.Name, c.Desc)
	if c.Usage != "" {
		fmt.Fprintf(&b, "Usage:\n  llmd %s\n\n", c.Usage)
	}
	if len(c.Flags) > 0 {
		b.WriteString("Flags:\n")
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
			fmt.Fprintf(&b, "  %-24s%s\n", flagStr, f.Desc)
		}
		b.WriteString("\n")
	}
	b.WriteString("Global Flags:\n")
	b.WriteString("  --author <name>     Author for mutations (required for LLMs/scripts)\n")
	b.WriteString("  --db <path>         Use a specific database file\n")
	b.WriteString("  --json              Output as JSON\n")
	b.WriteString("  --verbose           Debug logging on stderr\n")
	b.WriteString("  --help              Show help\n")
	return b.String()
}
