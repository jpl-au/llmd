// config.go implements the "llmd config" command for reading and writing
// configuration values. Config is stored as nested YAML in
// .llmd/config.yaml (local) or ~/.llmd/config.yaml (global).
//
// Usage:
//
//	llmd config                        Show all config
//	llmd config <key>                  Show value (dot notation)
//	llmd config <key> <value>          Set value (dot notation)

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

var configSpec = sdk.Command{
	Name: "config", Desc: `View or set configuration (e.g. author name)

With no arguments, shows all settings. With a dot-notation key, shows
that value. With a key and value, sets it. Use --global to write to the
global config (~/.llmd/config.yaml) instead of the local store config.

Keys use dot notation for nested values:

  author            Default author for interactive terminal use
  server.addr       HTTP server listen address
  log.level         Log level (debug, info, warn, error)
  log.format        Log format (text, json)
  limits.path_length   Maximum document path length in bytes
  limits.content_size  Maximum document content size in bytes

The "config author" setting is for the human user only — LLMs and
scripts must use --author on each command instead.`, Usage: "config [key] [value] | config git [allow|deny|ls] [pattern]", Flags: []sdk.Flag{
		{Name: "global", Type: "bool", Desc: "Write to global config (~/.llmd/config.yaml)"},
	},
}

var errConfigUsage = errors.New("config: usage: llmd config [--global] [key] [value]")

// configCmd handles show-all, show-key, and set-key operations.
func configCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(configSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	global := flags.Bool("global")

	// Subcommand dispatch.
	if len(positional) > 0 && positional[0] == "git" {
		return configGit(positional[1:])
	}

	switch len(positional) {
	case 0:
		cfg, err := config.Load()
		if err != nil {
			return nil, fmt.Errorf("config: reading: %w", err)
		}
		var lines []string
		for _, k := range config.Keys() {
			if v, ok := cfg.Field(k); ok {
				lines = append(lines, fmt.Sprintf("%s=%s", k, v))
			}
		}
		if len(lines) == 0 {
			return sdk.Text(""), nil
		}
		return sdk.Text(strings.Join(lines, "\n")), nil

	case 1:
		cfg, err := config.Load()
		if err != nil {
			return nil, fmt.Errorf("config: reading: %w", err)
		}
		if v, ok := cfg.Field(positional[0]); ok {
			return sdk.Text(v), nil
		}
		return sdk.Text(""), nil

	case 2:
		key, value := positional[0], positional[1]
		if err := config.Save(key, value, global); err != nil {
			return nil, fmt.Errorf("config: saving: %w", err)
		}
		return nil, nil

	default:
		return nil, errConfigUsage
	}
}
