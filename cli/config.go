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

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

var errConfigUsage = errors.New("config: usage: llmd config [--global] [key] [value]")

// configCmd handles show-all, show-key, and set-key operations.
func configCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	cfg := config.Load()

	var global bool
	var positional []string
	for _, arg := range args {
		if arg == "--global" {
			global = true
		} else {
			positional = append(positional, arg)
		}
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
		if err := config.Save(key, value, global); err != nil {
			return nil, fmt.Errorf("config: saving: %w", err)
		}
		return nil, nil

	default:
		return nil, errConfigUsage
	}
}
