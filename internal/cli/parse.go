package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/proto/plugin"
)

// ParseResult holds the result of parsing command-line arguments.
type ParseResult struct {
	Command string
	Args    []string
	Flags   map[string]any
	Output  OutputFormat
	Help    bool
	Author string // From --author flag (overrides config)
	Local  bool   // For config --local (write to .llmd/config.yaml instead of global)
}

// Parse parses command-line arguments according to the CLI spec.
//
// The args slice should not include the program name (os.Args[1:]).
// The cmdFlags parameter provides the flag definitions for the command,
// or nil if the command is not yet known (first pass parsing).
func Parse(args []string, cmdFlags []*plugin.Flag) (*ParseResult, error) {
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

	// Build flag lookup maps
	longFlags := make(map[string]*plugin.Flag)
	shortFlags := make(map[string]*plugin.Flag)
	for _, f := range cmdFlags {
		longFlags[f.Name] = f
		if f.Short != "" {
			shortFlags[f.Short] = f
		}
	}

	// Parse remaining arguments
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
			if name == "output" {
				if !hasValue {
					if i+1 >= len(args) {
						return nil, fmt.Errorf("--output requires a value")
					}
					i++
					value = args[i]
				}
				f, err := ParseOutputFormat(value)
				if err != nil {
					return nil, err
				}
				r.Output = f
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

			// Command flag
			flag, ok := longFlags[name]
			if !ok {
				return nil, fmt.Errorf("unknown flag: --%s", name)
			}

			if err := r.setFlagValue(flag, name, value, hasValue, args, &i); err != nil {
				return nil, err
			}
			continue
		}

		// Short flag
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			short := arg[1:2]
			flag, ok := shortFlags[short]
			if !ok {
				// Could be a negative number as positional arg
				if isNumber(arg) {
					r.Args = append(r.Args, arg)
					continue
				}
				return nil, fmt.Errorf("unknown flag: -%s", short)
			}

			// Value might be attached: -fvalue
			var value string
			var hasValue bool
			if len(arg) > 2 {
				value = arg[2:]
				hasValue = true
			}

			if err := r.setFlagValue(flag, flag.Name, value, hasValue, args, &i); err != nil {
				return nil, err
			}
			continue
		}

		// Positional argument
		r.Args = append(r.Args, arg)
	}

	// Apply defaults for unset flags
	for _, f := range cmdFlags {
		if _, ok := r.Flags[f.Name]; !ok && f.Default != "" {
			v, err := parseTypedValue(f.Type, f.Default)
			if err != nil {
				return nil, fmt.Errorf("invalid default for %s: %w", f.Name, err)
			}
			r.Flags[f.Name] = v
		}
	}

	// Validate required flags
	for _, f := range cmdFlags {
		if f.Required {
			if _, ok := r.Flags[f.Name]; !ok {
				return nil, fmt.Errorf("missing required flag: --%s", f.Name)
			}
		}
	}

	return r, nil
}

// setFlagValue sets a flag value based on its type.
func (r *ParseResult) setFlagValue(flag *plugin.Flag, name, value string, hasValue bool, args []string, i *int) error {
	switch flag.Type {
	case "bool":
		if !hasValue {
			r.Flags[name] = true
		} else {
			b, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid bool value for --%s: %s", name, value)
			}
			r.Flags[name] = b
		}

	case "string":
		if !hasValue {
			if *i+1 >= len(args) {
				return fmt.Errorf("--%s requires a value", name)
			}
			*i++
			value = args[*i]
		}
		r.Flags[name] = value

	case "int":
		if !hasValue {
			if *i+1 >= len(args) {
				return fmt.Errorf("--%s requires a value", name)
			}
			*i++
			value = args[*i]
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid int value for --%s: %s", name, value)
		}
		r.Flags[name] = n

	case "stringSlice":
		if !hasValue {
			if *i+1 >= len(args) {
				return fmt.Errorf("--%s requires a value", name)
			}
			*i++
			value = args[*i]
		}
		// Handle comma-separated values
		var values []string
		if strings.Contains(value, ",") {
			values = strings.Split(value, ",")
		} else {
			values = []string{value}
		}
		// Accumulate with existing values
		if existing, ok := r.Flags[name].([]string); ok {
			r.Flags[name] = append(existing, values...)
		} else {
			r.Flags[name] = values
		}

	default:
		// Treat as string
		if !hasValue {
			if *i+1 >= len(args) {
				return fmt.Errorf("--%s requires a value", name)
			}
			*i++
			value = args[*i]
		}
		r.Flags[name] = value
	}

	return nil
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

// parseTypedValue parses a string value according to its type.
func parseTypedValue(typ, value string) (any, error) {
	switch typ {
	case "bool":
		return strconv.ParseBool(value)
	case "int":
		return strconv.Atoi(value)
	case "stringSlice":
		if strings.Contains(value, ",") {
			return strings.Split(value, ","), nil
		}
		return []string{value}, nil
	default:
		return value, nil
	}
}

// isNumber checks if a string looks like a number (including negative).
func isNumber(s string) bool {
	if s == "" {
		return false
	}
	start := 0
	if s[0] == '-' || s[0] == '+' {
		start = 1
	}
	if start >= len(s) {
		return false
	}
	for _, c := range s[start:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
