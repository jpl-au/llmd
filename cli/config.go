// config.go implements the "llmd config" command for reading and writing
// configuration values. Config is stored in simple key=value files:
// global (~/.config/llmd/config) and local (.llmd/config).
//
// Usage:
//
//	llmd config                   Show all config
//	llmd config <key>             Show specific key
//	llmd config <key> <value>     Set config value

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

// configCmd handles show-all, show-key, and set-key operations.
func configCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	cfg := config.Load()

	switch len(args) {
	case 0:
		var lines []string
		for k, v := range cfg {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
		}
		return sdk.Text(strings.Join(lines, "\n")), nil

	case 1:
		if v, ok := cfg[args[0]]; ok {
			return sdk.Text(v), nil
		}
		return sdk.Text(""), nil

	case 2:
		key, value := args[0], args[1]
		if key != "author" {
			return nil, fmt.Errorf("config: unknown key: %s", key)
		}
		if err := config.Save(key, value); err != nil {
			return nil, fmt.Errorf("config: saving: %v", err)
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("config: usage: llmd config [key] [value]")
	}
}
