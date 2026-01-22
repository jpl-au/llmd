package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/proto/plugin"
)

// builtinCommands lists commands handled by the CLI itself.
var builtinCommands = []struct {
	name string
	desc string
}{
	{"version", "Show version information"},
	{"config", "Manage configuration"},
	{"plugins", "List loaded plugins"},
}

// RootHelp generates the root help text.
func RootHelp(commands map[string]*host.RegisteredCommand) string {
	var b strings.Builder

	b.WriteString("llmd - a document store for LLMs and humans\n\n")
	b.WriteString("Usage:\n")
	b.WriteString("  llmd <command> [flags] [args...]\n\n")

	b.WriteString("Global Flags:\n")
	b.WriteString("  --output <format>   Output format: text, json, md (default: text)\n")
	b.WriteString("  --help              Show help\n\n")

	b.WriteString("Built-in Commands:\n")
	for _, cmd := range builtinCommands {
		fmt.Fprintf(&b, "  %-12s%s\n", cmd.name, cmd.desc)
	}
	b.WriteString("\n")

	// Plugin commands sorted by name
	if len(commands) > 0 {
		b.WriteString("Plugin Commands:\n")
		names := make([]string, 0, len(commands))
		for name := range commands {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			cmd := commands[name]
			fmt.Fprintf(&b, "  %-12s%s\n", name, cmd.Description)
		}
		b.WriteString("\n")
	}

	b.WriteString("Use \"llmd <command> --help\" for more information about a command.\n")

	return b.String()
}

// CommandHelp generates help text for a plugin command.
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
			writeFlagHelp(&b, f)
		}
		b.WriteString("\n")
	}

	b.WriteString("Global Flags:\n")
	b.WriteString("  --output <format>   Output format: text, json, md (default: text)\n")
	b.WriteString("  --help              Show help\n")

	return b.String()
}

// BuiltinHelp generates help text for a built-in command.
func BuiltinHelp(name string) string {
	switch name {
	case "version":
		return "version - Show version information\n"

	case "plugins":
		var b strings.Builder
		b.WriteString("plugins - List loaded plugins\n\n")
		b.WriteString("Usage:\n")
		b.WriteString("  llmd plugins [flags]\n\n")
		b.WriteString("Global Flags:\n")
		b.WriteString("  --output <format>   Output format: text, json, md (default: text)\n")
		b.WriteString("  --help              Show help\n")
		return b.String()

	case "config":
		return "config - Manage configuration (to be specified)\n"

	default:
		return fmt.Sprintf("%s - No help available\n", name)
	}
}

// writeFlagHelp writes a single flag's help line.
func writeFlagHelp(b *strings.Builder, f *plugin.Flag) {
	var flagStr string
	if f.Short != "" {
		flagStr = fmt.Sprintf("-%s, --%s", f.Short, f.Name)
	} else {
		flagStr = fmt.Sprintf("    --%s", f.Name)
	}

	// Add type hint
	switch f.Type {
	case "string":
		flagStr += " <string>"
	case "int":
		flagStr += " <int>"
	case "stringSlice":
		flagStr += " <value>"
	}

	// Format with description
	fmt.Fprintf(b, "  %-24s%s", flagStr, f.Description)
	if f.Required {
		b.WriteString(" (required)")
	}
	b.WriteString("\n")
}

// IsBuiltin returns true if the command is a built-in command.
func IsBuiltin(name string) bool {
	for _, cmd := range builtinCommands {
		if cmd.name == name {
			return true
		}
	}
	return false
}
