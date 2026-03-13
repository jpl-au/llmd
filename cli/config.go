// config.go implements the "llmd config" command for reading and writing
// configuration values. Config is stored in simple key=value files:
// global (~/.llmd/config) and local (.llmd/config).
//
// Usage:
//
//	llmd config                   Show all config
//	llmd config <key>             Show specific key
//	llmd config <key> <value>     Set config value

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var configSpec = sdk.Command{
	Name: "config", Desc: `View or set configuration (e.g. author name)

With no arguments, shows all settings. With a key, shows that value.
With a key and value, sets it. Use --global to write to the global
config (~/.llmd/config) instead of the local store config.`, Usage: "config [key] [value] | config ignore [add|rm|ls] [pattern]", Flags: []sdk.Flag{
		{Name: "global", Type: "bool", Desc: "Write to global config (~/.llmd/config)"},
	},
}

var errConfigUsage = errors.New("config: usage: llmd config [--global] [key] [value]")

// configCmd handles show-all, show-key, and set-key operations.
func configCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	cfg, err := ctx.Config.Read()
	if err != nil {
		return nil, fmt.Errorf("config: reading: %w", err)
	}

	flags, positional, err := sdk.ParseArgs(configSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	global := flags.Bool("global")

	// Subcommand dispatch.
	if len(positional) > 0 && positional[0] == "ignore" {
		return configIgnore(ctx, positional[1:])
	}

	switch len(positional) {
	case 0:
		var lines []string
		for k, v := range cfg {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
		}
		return sdk.Text(strings.Join(lines, "\n")), nil

	case 1:
		if v, ok := cfg[positional[0]]; ok {
			return sdk.Text(v), nil
		}
		return sdk.Text(""), nil

	case 2:
		key, value := positional[0], positional[1]
		if key != "author" {
			return nil, fmt.Errorf("config: unknown key: %s", key)
		}
		if err := ctx.Config.Write(key, value, sdk.WriteOpts{Global: global}); err != nil {
			return nil, fmt.Errorf("config: saving: %w", err)
		}
		return nil, nil

	default:
		return nil, errConfigUsage
	}
}
