// config.go implements the "llmd config" command for configuration management.
//
// Separated from extension.go to isolate config-specific logic including
// the local vs global config precedence rules.
//
// Design: Config follows a cascade model similar to git: local config
// (.llmd/config.yaml) takes precedence over global (~/.llmd/config.yaml).
// The --local flag forces use of local config even if it doesn't exist yet,
// enabling config setup during init workflows.

package core

import (
	"fmt"

	"github.com/jpl-au/llmd/cmd"
	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config [key] [value]",
		Short: "View or set config values",
		Long: `View or set config values.

  llmd config                 # show config
  llmd config sync.files      # show sync.files value
  llmd config sync.files true # set sync.files

Configuration locations:
  Global: ~/.llmd/config.yaml
  Local:  .llmd/config.yaml (created by init)

Uses local config if it exists, otherwise global.
Writes go to the same place reads come from.
Use --local to use local config instead.`,
		Args: cobra.MaximumNArgs(2),
		RunE: runConfig,
	}
	c.Flags().Bool(extension.FlagLocal, false, "Use local config (.llmd/config.yaml)")
	return c
}

func runConfig(c *cobra.Command, args []string) error {
	forceLocal, _ := c.Flags().GetBool(extension.FlagLocal)

	// Load config: local if exists, otherwise global
	// --local flag forces local even if it doesn't exist yet
	var cfg *config.Config
	var err error
	if forceLocal {
		cfg, err = config.LoadScope(config.ScopeLocal)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		return cmd.PrintJSONError(fmt.Errorf("config load: %w", err))
	}

	scopeName := "global"
	if cfg.Scope() == config.ScopeLocal {
		scopeName = "local"
	}

	switch len(args) {
	case 0:
		// Show all values
		for k, v := range cfg.All() {
			fmt.Fprintf(cmd.Out(), "%s: %s\n", k, v)
		}
		log.Event("core:config", "list").Author(cmd.Author()).Write(nil)

	case 1:
		// Get single value
		v, err := cfg.Get(args[0])
		log.Event("core:config", "get").Author(cmd.Author()).Detail("key", args[0]).Write(err)
		if err != nil {
			return cmd.PrintJSONError(fmt.Errorf("config get %q: %w", args[0], err))
		}
		fmt.Fprintln(cmd.Out(), v)

	case 2:
		// Set value - write to same place we read from
		if err := cfg.Set(args[0], args[1]); err != nil {
			log.Event("core:config", "set").Author(cmd.Author()).Detail("key", args[0]).Write(err)
			return cmd.PrintJSONError(fmt.Errorf("config set %q: %w", args[0], err))
		}

		saveErr := cfg.Save()
		// Note: value intentionally not logged to avoid leaking sensitive config (API keys, tokens)
		log.Event("core:config", "set").Author(cmd.Author()).Detail("key", args[0]).Detail("scope", scopeName).Write(saveErr)
		if saveErr != nil {
			return cmd.PrintJSONError(fmt.Errorf("config save: %w", saveErr))
		}
		fmt.Fprintf(cmd.Out(), "%s = %s (%s)\n", args[0], args[1], scopeName)
	}
	return nil
}
