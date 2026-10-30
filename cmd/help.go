// help.go renders the root and per-command help output.

package cmd

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/host"
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
// the first line of its description. program is the binary's name,
// which every command line in the help starts with.
func printHelp(h *host.Host, program string) {
	fmt.Printf(`%[1]s - a versioned document store with task boards and agent orchestration

AI agents: run "%[1]s guide" to get started.

Usage:
  %[1]s <command> [flags] [args...]

Global Flags:
  --author <name>     Who is making the change (required for agents and scripts)
  --db <path>         Use a specific database file
  --json              Output as JSON
  --verbose           Debug logging on stderr
  --help              Show help
  --version           Show version information

`, program)
	cmds := h.Commands()
	for _, g := range append(helpGroups, extensionGroups(h.Extensions())...) {
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

	fmt.Printf("Use \"%s <command> --help\" for more information about a command.\n", program)
}

// extensionGroups returns a group for each extension whose commands are
// in none of helpGroups, under the extension's name, so the commands a
// build adds to llmd are listed beside llmd's own.
func extensionGroups(exts []sdk.Extension) []commandGroup {
	listed := make(map[string]bool)
	for _, g := range helpGroups {
		for _, name := range g.names {
			listed[name] = true
		}
	}
	var groups []commandGroup
	for _, e := range exts {
		var names []string
		for _, c := range e.Commands() {
			if !listed[c.Name] {
				names = append(names, c.Name)
			}
		}
		if len(names) > 0 {
			groups = append(groups, commandGroup{e.Name(), names})
		}
	}
	return groups
}

// printCmdHelp displays help for a single command: the full form for
// a person at a terminal, the agent form when stdout is a pipe, as it
// is for a model running llmd in a shell.
func printCmdHelp(c *sdk.Command, program string) {
	fmt.Print(cmdHelp(c, program, isTTY()))
}

// cmdHelp renders a command's help. For a terminal it is the whole
// description, the usage line, the flags with their types, and the
// global flags. For a pipe it is the command's AgentHelp, the same
// text the MCP tool list carries, followed by the global flags an
// agent on the command line needs.
func cmdHelp(c *sdk.Command, program string, tty bool) string {
	var b strings.Builder
	if !tty {
		b.WriteString(c.AgentHelp(program))
		b.WriteString("\nGlobal: --author <name> (required to change anything), --json (structured output), --db <path>, --verbose\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%s - %s\n\n", c.Name, c.Desc)
	if c.Usage != "" {
		fmt.Fprintf(&b, "Usage:\n  %s %s\n\n", program, c.Usage)
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
