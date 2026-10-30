// agent_harness.go implements agent harness: show or name the harness
// an agent runs under.

package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

// harnessView is the structured form of agent harness.
type harnessView struct {
	Agent   string `json:"agent"`
	Harness string `json:"harness"`
}

func agentHarness(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "agent harness: an agent name is required.")
	}
	name := args[0]

	// Naming a harness changes the store, so it is recorded with an
	// author. Showing one changes nothing.
	if len(args) > 1 {
		if ctx.Author == "" {
			return nil, sdk.AuthorRequired("agent harness")
		}
		if err := ctx.Agents.SetHarness(name, args[1], ctx.Author); err != nil {
			return nil, fmt.Errorf("agent harness: %w", err)
		}
	}

	h, err := ctx.Agents.Harness(name)
	if err != nil {
		return nil, fmt.Errorf("agent harness: %w", err)
	}
	result := sdk.Result{
		Text: fmt.Sprintf("%s runs under the harness %s.", name, h),
		Data: harnessView{Agent: name, Harness: h},
	}
	// agent.harness in the configuration file wins on this computer,
	// so a harness named for the store may not be the one that runs.
	if len(args) > 1 && h != args[1] {
		result.Note = fmt.Sprintf("agent.harness in the configuration file runs %s on this computer.", h)
	}
	return result, nil
}
