package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jpl-au/llmd/internal/config"
)

// runConfig executes the config command.
//
// Usage:
//
//	llmd config                    # Show all config (merged)
//	llmd config author             # Show specific key
//	llmd config author "Jane"      # Set global config (default)
//	llmd config --local author "Jane"  # Set local config (.llmd/config.yaml)
func (c *CLI) runConfig(ctx context.Context, result *ParseResult) int {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = &config.Config{}
	}

	switch len(result.Args) {
	case 0:
		// Show all config
		return c.showConfig(cfg, result.Output)

	case 1:
		// Show specific key
		key := result.Args[0]
		value, ok := cfg.Get(key)
		if !ok {
			// Key exists but is empty - just print nothing
			return ExitSuccess
		}
		fmt.Fprintln(c.stdout, value)
		return ExitSuccess

	case 2:
		// Set key=value
		key, value := result.Args[0], result.Args[1]

		// Determine which config file to write to (default: global)
		var path string
		if result.Local {
			path = config.LocalPath()
		} else {
			var err error
			path, err = config.GlobalPath()
			if err != nil {
				c.writeError(fmt.Errorf("getting global config path: %w", err), result.Output)
				return ExitError
			}
		}

		if err := config.Set(path, key, value); err != nil {
			c.writeError(err, result.Output)
			return ExitError
		}
		return ExitSuccess

	default:
		c.writeError(fmt.Errorf("usage: llmd config [key] [value]"), result.Output)
		return ExitUsage
	}
}

// showConfig displays all config values.
func (c *CLI) showConfig(cfg *config.Config, format OutputFormat) int {
	switch format {
	case OutputJSON:
		data := map[string]string{}
		if cfg.Author != "" {
			data["author"] = cfg.Author
		}
		if cfg.Output != "" {
			data["output"] = cfg.Output
		}
		json.NewEncoder(c.stdout).Encode(data)

	default:
		// Text format
		if cfg.Author != "" {
			fmt.Fprintf(c.stdout, "author=%s\n", cfg.Author)
		}
		if cfg.Output != "" {
			fmt.Fprintf(c.stdout, "output=%s\n", cfg.Output)
		}
	}
	return ExitSuccess
}
