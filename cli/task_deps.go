// task_deps.go handles dependency chain and readiness subcommands.

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// taskChain displays the dependency chain for a task.
func taskChain(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task chain: %w: id", sdk.ErrMissingArg)
	}

	chain, err := ctx.Tasks.Chain(args[0])
	if err != nil {
		return nil, fmt.Errorf("task chain: %w", err)
	}

	var b strings.Builder
	for i, t := range chain {
		indent := strings.Repeat("   ", i)
		prefix := ""
		if i > 0 {
			prefix = "\u2514\u2500 "
		}

		check := ""
		if t.Status == "done" {
			check = " \u2713"
		}

		fmt.Fprintf(&b, "%s%s%s  %s  [%s]%s\n", indent, prefix, t.Key, t.Title, t.Status, check)
	}

	return sdk.Result{Text: b.String(), Data: chain}, nil
}

// taskReady reports whether a task's dependencies are all satisfied.
func taskReady(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task ready: %w: id", sdk.ErrMissingArg)
	}

	ready, err := ctx.Tasks.Ready(args[0])
	if err != nil {
		return nil, fmt.Errorf("task ready: %w", err)
	}

	data := map[string]any{"key": args[0], "ready": ready}
	if ready {
		return sdk.Result{Text: fmt.Sprintf("Task %s is ready", args[0]), Data: data}, nil
	}
	return sdk.Result{Text: fmt.Sprintf("Task %s is blocked by unfinished dependencies", args[0]), Data: data}, nil
}
