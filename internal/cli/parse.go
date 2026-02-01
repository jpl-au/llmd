package cli

import (
	"fmt"
	"strings"
)

// ParseResult holds the result of parsing command-line arguments.
type ParseResult struct {
	Command string
	Args    []string
	Flags   map[string]any
	Output  OutputFormat
	Help    bool
	Raw     bool   // Skip glamour markdown rendering
	Author  string // From --author flag (overrides config)
	Local   bool   // For config --local (write to .llmd/config.yaml instead of global)
}

// Parse parses command-line arguments according to the CLI spec.
//
// The args slice should not include the program name (os.Args[1:]).
// Only global flags (--help, --json, --md, --author, --local) are extracted.
// All other flags and positional arguments are passed through raw
// in Args for the plugin to parse.
func Parse(args []string) (*ParseResult, error) {
	r := &ParseResult{
		Args:   make([]string, 0),
		Flags:  make(map[string]any),
		Output: OutputText,
	}

	if len(args) == 0 {
		return r, nil
	}

	// First arg is --help → root help
	if args[0] == "--help" {
		r.Help = true
		return r, nil
	}

	// First arg is the command
	r.Command = args[0]
	args = args[1:]

	// Parse remaining arguments - only extract global flags
	stopFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// After --, everything is positional
		if stopFlags {
			r.Args = append(r.Args, arg)
			continue
		}

		// Stop flag parsing marker
		if arg == "--" {
			stopFlags = true
			continue
		}

		// Long flag
		if strings.HasPrefix(arg, "--") {
			name, value, hasValue := parseFlag(arg[2:])

			// Global flags
			if name == "help" {
				r.Help = true
				continue
			}
			if name == "json" {
				r.Output = OutputJSON
				continue
			}
			if name == "md" || name == "markdown" {
				r.Output = OutputMarkdown
				continue
			}
			if name == "author" {
				if !hasValue {
					if i+1 >= len(args) {
						return nil, fmt.Errorf("--author requires a value")
					}
					i++
					value = args[i]
				}
				r.Author = value
				continue
			}
			if name == "local" {
				r.Local = true
				continue
			}
			if name == "raw" {
				r.Raw = true
				continue
			}

			// Unknown flag - pass through to plugin
			r.Args = append(r.Args, arg)
			continue
		}

		// Short flag - pass through to plugin (not a global flag)
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			r.Args = append(r.Args, arg)
			continue
		}

		// Positional argument
		r.Args = append(r.Args, arg)
	}

	return r, nil
}

// parseFlag splits a flag into name and optional value.
// For "name=value" returns ("name", "value", true).
// For "name" returns ("name", "", false).
func parseFlag(s string) (name, value string, hasValue bool) {
	if idx := strings.Index(s, "="); idx != -1 {
		return s[:idx], s[idx+1:], true
	}
	return s, "", false
}
