// hook.go scaffolds agent hook configurations.

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/assets/platform/claude"
	"github.com/jpl-au/llmd/assets/platform/gemini"
	"github.com/jpl-au/llmd/assets/platform/generic"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

var hookSpec = sdk.Command{
	Name: "hook", Desc: `Generate agent hook configurations.

Prints a hook configuration for the named platform that integrates
the agent with llmd's queue, task, and audit systems.

Subcommands:
  init <platform>    generate hook config (claude, gemini, generic)

The output is printed to stdout. For Claude Code, merge the hooks
section into .claude/settings.json. For Gemini, add to your hook
config. For generic agents, save as a wrapper script.`, Usage: "hook init <platform>",
}

// hookCmd dispatches hook subcommands.
func hookCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("hook: %w: subcommand required (init)", sdk.ErrMissingArg)
	}

	switch args[0] {
	case "init":
		return hookInit(ctx, args[1:])
	default:
		return nil, fmt.Errorf("hook: unknown subcommand: %s", args[0])
	}
}

var platforms = []string{"claude", "gemini", "generic"}

// hookInit generates a hook configuration for the named platform.
func hookInit(_ sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("hook init: %w: platform required (%s)",
			sdk.ErrMissingArg, strings.Join(platforms, ", "))
	}

	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		cfg = config.Config{}
	}
	addr := cfg.Server.Addr
	if addr == "" {
		addr = "localhost:5563"
	}

	// Default author from the platform name.
	author := name
	if cfg.Author != "" {
		author = cfg.Author
	}

	var out string
	switch {
	case strings.Contains(name, "claude"):
		out = claude.HookConfig(addr, author)
	case strings.Contains(name, "gemini"):
		out = gemini.HookConfig(addr, author)
	default:
		out = generic.HookConfig(addr, author)
	}

	return sdk.Text(out), nil
}
