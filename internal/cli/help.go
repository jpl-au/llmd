package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/proto/plugin"
)

// Built-in command help.
const (
	helpVersion = "version - Show version information\n"

	helpPlugins = `plugins - List loaded plugins

Usage:
  llmd plugins [flags]

Global Flags:
  --json              Output as JSON
  --help              Show help
`

	helpConfig = `config - Manage configuration

Usage:
  llmd config                       Show all config
  llmd config <key>                 Show specific key
  llmd config <key> <value>         Set global config
  llmd config --local <key> <value> Set local config
`

	helpInit = `init - Initialize a new store

Usage:
  llmd init

Creates a new llmd store in .llmd/llmd.db
Fails if store already exists.
`

	helpVacuum = `vacuum - Clean up deleted documents

Usage:
  llmd vacuum [flags]

Permanently removes soft-deleted documents, orphaned tags, and links.

Global Flags:
  --json              Output as JSON
  --help              Show help
`
)

const rootHeader = `llmd - a document store for LLMs and humans

Usage:
  llmd <command> [flags] [args...]

Global Flags:
  --json              Output as JSON
  --help              Show help

Built-in Commands:
  config      Manage configuration
  init        Initialize a new store
  plugins     List loaded plugins
  vacuum      Clean up deleted documents
  version     Show version information

`

const rootFooter = `
Use "llmd <command> --help" for more information about a command.
`

// RootHelp returns help text with plugin commands.
func RootHelp(commands map[string]*host.RegisteredCommand) string {
	if len(commands) == 0 {
		return rootHeader + rootFooter[1:]
	}

	var b strings.Builder
	b.WriteString(rootHeader)
	b.WriteString("Plugin Commands:\n")

	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintf(&b, "  %-12s%s\n", name, commands[name].Description)
	}
	b.WriteString(rootFooter)

	return b.String()
}

// CommandHelp returns help text for a plugin command.
func CommandHelp(cmd *host.RegisteredCommand) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s - %s\n\n", cmd.Name, cmd.Description)

	if cmd.Usage != "" {
		b.WriteString("Usage:\n")
		fmt.Fprintf(&b, "  %s\n\n", cmd.Usage)
	}

	if len(cmd.Flags) > 0 {
		b.WriteString("Flags:\n")
		for _, f := range cmd.Flags {
			writeFlag(&b, f)
		}
		b.WriteString("\n")
	}

	b.WriteString("Global Flags:\n")
	b.WriteString("  --json              Output as JSON\n")
	b.WriteString("  --help              Show help\n")

	return b.String()
}

// BuiltinHelp returns help text for a built-in command.
func BuiltinHelp(name string) string {
	switch name {
	case "config":
		return helpConfig
	case "init":
		return helpInit
	case "plugins":
		return helpPlugins
	case "vacuum":
		return helpVacuum
	case "version":
		return helpVersion
	default:
		return fmt.Sprintf("%s - No help available\n", name)
	}
}

// IsBuiltin returns true if the command is built-in.
func IsBuiltin(name string) bool {
	switch name {
	case "config", "init", "plugins", "vacuum", "version":
		return true
	}
	return false
}

func writeFlag(b *strings.Builder, f *plugin.Flag) {
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
	case "stringSlice":
		flagStr += " <value>"
	}

	fmt.Fprintf(b, "  %-24s%s", flagStr, f.Description)
	if f.Required {
		b.WriteString(" (required)")
	}
	b.WriteString("\n")
}
