// task_next.go claims the next ready task for the caller.

package cli

import (
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var taskNextFlags = []sdk.Flag{
	{Name: "from", Type: "string", Desc: "Column to take from (default up-next)"},
	{Name: "to", Type: "string", Desc: "Column to move to (default in-progress)"},
	{Name: "assign", Type: "string", Desc: "Assignee (default: the author)"},
}

// taskNext takes the first ready task of a column, moving and
// assigning it in one write, and shows it in full so the caller can
// start work without a second call. Nothing ready is an error, so a
// script or agent loop can stop on it.
func taskNext(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, _, err := sdk.ParseArgs(taskNextFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task next: %w", err)
	}
	opts := sdk.NextOpts{
		From:       flags.String("from"),
		To:         flags.String("to"),
		AssignedTo: flags.String("assign"),
	}
	from := opts.From
	if from == "" {
		from = "up-next"
	}

	t, err := ctx.Tasks.Next(ctx.Author, opts)
	if err != nil {
		if errors.Is(err, sdk.ErrNoneReady) {
			return nil, sdk.Errorf(sdk.ErrNoneReady, "task next: no task is ready in %s. Run llmd task list to see the board.", from)
		}
		return nil, fmt.Errorf("task next: %w", err)
	}

	text := fmt.Sprintf("Claimed %s: moved from %s to %s, assigned to %s.\n\n%s",
		t.Key, from, t.Status, t.AssignedTo, taskDetail(ctx, t))
	return sdk.Markdown{Text: text, Data: t}, nil
}
