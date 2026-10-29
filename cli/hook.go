// hook.go is the single entry point for agent hook events.
//
// An agent platform runs "llmd --author <name> hook" at points in its
// lifecycle and passes its native JSON payload on stdin. The command
// hands the payload to sdk.HookStore, which chooses the platform from
// the author, normalises the event and acts on it. The HTTP server
// exposes the same command as POST /hook, so one implementation
// defines what every hook does.
//
// "hook init <platform>" prints the configuration that wires a
// platform's hooks to this command.

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/assets/platform/claude"
	"github.com/jpl-au/llmd/assets/platform/gemini"
	"github.com/jpl-au/llmd/assets/platform/generic"
	"github.com/jpl-au/llmd/config"
	"github.com/jpl-au/llmd/sdk"
)

var hookSpec = sdk.Command{
	Name: "hook", Guide: "hooks", Desc: `Handle an agent hook event, or print hook configuration

Reads an agent platform's hook payload on stdin and acts on it:

  session start      prints the author's newest pending queue messages (hook.limit, default 10)
  session end/stop   sends a queue message that the session ended
  task completed     finishes the task and sends a queue message
  task failed        moves the task to blocked and sends a queue message
  tool use           records an audit entry when a task and content are given

The platform is chosen from --author: a name containing "claude" or
"gemini" selects that platform's payload format; any other name uses
the generic format, a JSON object with "event", "task_id" and
"content" fields.

Subcommands:
  init <platform>    print hook configuration (claude, gemini, generic)`,
	Usage: "hook [--task <id>] | hook init <platform>",
	Flags: []sdk.Flag{
		{Name: "task", Type: "string", Desc: "Task the event belongs to, when the payload does not say"},
	},
}

// hookCmd runs hook init, or handles an event from stdin.
func hookCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) > 0 && args[0] == "init" {
		return hookInit(ctx, args[1:])
	}
	return hookEvent(ctx, args)
}

// hookEvent handles one hook payload.
func hookEvent(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(hookSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("hook: %w", err)
	}
	if len(positional) > 0 {
		return nil, sdk.Errorf(sdk.ErrInvalidArg, "hook: unknown subcommand %s. Use init.", positional[0])
	}
	if ctx.Author == "" {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "hook: an author is required. --author names the agent and selects its payload format.")
	}
	if ctx.Hooks == nil {
		return nil, sdk.Errorf(sdk.ErrNotFound, "hook: there is no store here. Run llmd init to create one.")
	}
	payload, err := ctx.Input()
	if err != nil {
		return nil, fmt.Errorf("hook: reading stdin: %w", err)
	}
	if len(payload) == 0 {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "hook: no payload. The agent platform sends it on stdin.")
	}

	result, err := ctx.Hooks.Handle(ctx.Author, payload, sdk.HookOpts{Task: flags.String("task")})
	if err != nil {
		return nil, fmt.Errorf("hook: %w", err)
	}
	return sdk.Result{Text: result.Content, Data: result}, nil
}

var platforms = []string{"claude", "gemini", "generic"}

// hookInit prints the hook configuration for a platform. The author
// comes from the configured author when there is one, otherwise from
// the platform name.
func hookInit(_ sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "hook init: a platform is required. Use one of %s.",
			strings.Join(platforms, ", "))
	}

	name := args[0]

	author := name
	if cfg, err := config.Load(); err == nil && cfg.Author != "" {
		author = cfg.Author
	}

	var out string
	switch {
	case strings.Contains(name, "claude"):
		out = claude.HookConfig(author)
	case strings.Contains(name, "gemini"):
		out = gemini.HookConfig(author)
	default:
		out = generic.HookConfig(author)
	}

	return sdk.Text(out), nil
}
