// task_move.go handles move, set, rm, and restore subcommands.

package cli

import (
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

// taskMove changes a task's column. Requires two positional arguments:
// the task key and the target column name. When the move fails due to
// ErrNoSpec (task has no spec body), the error message includes
// actionable instructions for writing or linking a spec.
func taskMove(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("task move: %w: id and column", sdk.ErrMissingArg)
	}

	if err := ctx.Tasks.Move(args[0], args[1], ctx.Author); err != nil {
		if errors.Is(err, sdk.ErrNoSpec) {
			tsk, rerr := ctx.Tasks.Read(args[0])
			if rerr == nil {
				return nil, fmt.Errorf("task move: spec required - tasks cannot leave the backlog until their document has content beyond the title heading.\n\nWrite the spec:\n  llmd write %s\n\nOr link an existing document:\n  llmd task link %s <path>", tsk.Path, args[0])
			}
		}
		return nil, fmt.Errorf("task move: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Moved task %s to %s", args[0], args[1])), nil
}

// taskSet updates task metadata fields. Parses --title, --priority,
// --assign, --position, --flag, --unflag, and --branch from the
// argument list and builds a TaskSetOpts with non-nil pointers only
// for the fields that were explicitly provided.
var taskSetFlags = []sdk.Flag{
	{Name: "title", Type: "string"},
	{Name: "priority", Type: "int"},
	{Name: "assign", Type: "string"},
	{Name: "position", Type: "int"},
	{Name: "flag", Type: "string"},
	{Name: "unflag", Type: "string"},
	{Name: "branch", Type: "string"},
	{Name: "depends-on", Type: "string"},
}

func taskSet(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task set: %w: id", sdk.ErrMissingArg)
	}

	key := args[0]
	flags, _, err := sdk.ParseArgs(taskSetFlags, args[1:])
	if err != nil {
		return nil, fmt.Errorf("task set: %w", err)
	}

	var opts sdk.TaskSetOpts
	if flags.Has("title") {
		v := flags.String("title")
		opts.Title = &v
	}
	if flags.Has("priority") {
		v := flags.Int("priority")
		opts.Priority = &v
	}
	if flags.Has("assign") {
		v := flags.String("assign")
		opts.AssignedTo = &v
	}
	if flags.Has("position") {
		v := flags.Int("position")
		opts.Position = &v
	}
	if flags.Has("branch") {
		v := flags.String("branch")
		opts.Branch = &v
	}
	if flags.Has("depends-on") {
		v := flags.String("depends-on")
		opts.DependsOn = &v
	}
	opts.Flag = flags.String("flag")
	opts.Unflag = flags.String("unflag")

	if err := ctx.Tasks.Set(key, ctx.Author, opts); err != nil {
		return nil, fmt.Errorf("task set: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Updated task %s", key)), nil
}

// taskRm soft-deletes a task. The backing document is not removed - the
// output reminds the user to delete it separately if desired.
func taskRm(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task rm: %w: id", sdk.ErrMissingArg)
	}

	t, err := ctx.Tasks.Delete(args[0], ctx.Author)
	if err != nil {
		return nil, fmt.Errorf("task rm: %w", err)
	}

	text := fmt.Sprintf("Removed task %s \"%s\"", t.Key, t.Title)
	if ok, _ := ctx.Documents.Exists(t.Path); ok {
		text += fmt.Sprintf("\nNote: the document at %s still exists. To remove it: llmd rm %s", t.Path, t.Path)
	}
	return sdk.Text(text), nil
}

// taskRestore undeletes a soft-deleted task, returning it to the column
// it was in when deleted.
func taskRestore(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task restore: %w: id", sdk.ErrMissingArg)
	}

	t, err := ctx.Tasks.Restore(args[0], ctx.Author)
	if err != nil {
		return nil, fmt.Errorf("task restore: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Restored task %s \"%s\"", t.Key, t.Title)), nil
}
