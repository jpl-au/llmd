// flags.go provides standardised argument parsing driven by Flag definitions.
//
// ParseArgs uses the same []Flag metadata that powers --help and MCP tool
// schemas, so a single definition drives parsing, documentation, and tool
// discovery. Plugins (including Yaegi) get correct flag parsing by calling
// ParseArgs with their Command.Flags — no external dependencies required.

package sdk

import (
	"fmt"
	"strconv"
	"strings"
)

// Flag describes a command flag. The same definition drives --help display,
// MCP tool schemas, and argument parsing via [ParseArgs].
type Flag struct {
	Name  string // Long flag name (e.g. "version" for --version)
	Short string // Optional short form (e.g. "n" for -n)
	Type  string // "bool", "string", or "int"
	Desc  string
}

// FlagValues holds parsed flag values keyed by Flag.Name.
type FlagValues struct {
	set     map[string]bool
	bools   map[string]bool
	strings map[string]string
	ints    map[string]int
}

// Has reports whether the named flag was explicitly provided.
func (fv FlagValues) Has(name string) bool { return fv.set[name] }

// Bool returns the value of a boolean flag, or false if unset.
func (fv FlagValues) Bool(name string) bool { return fv.bools[name] }

// String returns the value of a string flag, or "" if unset.
func (fv FlagValues) String(name string) string { return fv.strings[name] }

// Int returns the value of an integer flag, or 0 if unset.
func (fv FlagValues) Int(name string) int { return fv.ints[name] }

// ParseArgs parses args against the given flag definitions. It returns
// parsed values, remaining positional arguments, and any parse error.
//
// Supports:
//   - Long flags: --name, --name=value, --name value
//   - Short flags: -n, -C3, -C 3
//   - Combined short bools: -lat (equivalent to -l -a -t)
//   - -- to stop flag parsing (everything after is positional)
func ParseArgs(flags []Flag, args []string) (FlagValues, []string, error) {
	byLong := make(map[string]*Flag, len(flags))
	byShort := make(map[byte]*Flag, len(flags))
	for i := range flags {
		f := &flags[i]
		byLong[f.Name] = f
		if f.Short != "" {
			byShort[f.Short[0]] = f
		} else if len(f.Name) == 1 {
			byShort[f.Name[0]] = f
		}
	}

	fv := FlagValues{
		set:     make(map[string]bool),
		bools:   make(map[string]bool),
		strings: make(map[string]string),
		ints:    make(map[string]int),
	}
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// -- terminates flag parsing.
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}

		// Long flags (--name, --name=value).
		if strings.HasPrefix(arg, "--") {
			name, val, hasEq := strings.Cut(arg[2:], "=")
			f, ok := byLong[name]
			if !ok {
				return fv, nil, fmt.Errorf("unknown flag: --%s", name)
			}
			if err := setFlag(&fv, f, val, hasEq, func() (string, error) {
				i++
				if i >= len(args) {
					return "", fmt.Errorf("--%s requires a value", name)
				}
				return args[i], nil
			}); err != nil {
				return fv, nil, err
			}
			continue
		}

		// Short flags (-n, -C3, -lat).
		if len(arg) > 1 && arg[0] == '-' {
			chars := arg[1:]
			for j := 0; j < len(chars); j++ {
				c := chars[j]
				f, ok := byShort[c]
				if !ok {
					return fv, nil, fmt.Errorf("unknown flag: -%c", c)
				}
				if f.Type == "bool" {
					fv.set[f.Name] = true
					fv.bools[f.Name] = true
					continue
				}
				// Non-bool short flag: consume remainder or next arg.
				rest := chars[j+1:]
				if err := setFlag(&fv, f, rest, rest != "", func() (string, error) {
					i++
					if i >= len(args) {
						return "", fmt.Errorf("-%c requires a value", c)
					}
					return args[i], nil
				}); err != nil {
					return fv, nil, err
				}
				break // rest of chars consumed
			}
			continue
		}

		positional = append(positional, arg)
	}

	return fv, positional, nil
}

// setFlag stores a parsed value into fv. If hasVal is true, val is used
// directly. Otherwise nextArg is called to fetch the next argument.
func setFlag(fv *FlagValues, f *Flag, val string, hasVal bool, nextArg func() (string, error)) error {
	fv.set[f.Name] = true
	switch f.Type {
	case "bool":
		fv.bools[f.Name] = true
	case "string":
		if !hasVal {
			v, err := nextArg()
			if err != nil {
				return err
			}
			val = v
		}
		fv.strings[f.Name] = val
	case "int":
		if !hasVal {
			v, err := nextArg()
			if err != nil {
				return err
			}
			val = v
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("--%s: invalid integer: %s", f.Name, val)
		}
		fv.ints[f.Name] = n
	}
	return nil
}
