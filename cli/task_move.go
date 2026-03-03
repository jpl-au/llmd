// task_move.go handles move, set, rm, and restore subcommands.

package cli

import (
	"errors"
	"fmt"
	"strconv"

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
				return nil, fmt.Errorf("task move: spec required — tasks cannot leave the backlog until their document has content beyond the title heading.\n\nWrite the spec:\n  llmd write %s\n\nOr link an existing document:\n  llmd task link %s <path>", tsk.Path, args[0])
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
func taskSet(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task set: %w: id", sdk.ErrMissingArg)
	}

	key := args[0]
	args = args[1:]

	var opts sdk.TaskSetOpts
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--title":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task set: --title requires a value")
			}
			opts.Title = &args[i]
		case "--priority":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task set: --priority requires a value")
			}
			p, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("task set: invalid priority: %w", err)
			}
			opts.Priority = &p
		case "--assign":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task set: --assign requires a value")
			}
			opts.AssignedTo = &args[i]
		case "--position":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task set: --position requires a value")
			}
			p, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("task set: invalid position: %w", err)
			}
			opts.Position = &p
		case "--flag":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task set: --flag requires a value")
			}
			opts.Flag = args[i]
		case "--unflag":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task set: --unflag requires a value")
			}
			opts.Unflag = args[i]
		case "--branch":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task set: --branch requires a value")
			}
			opts.Branch = &args[i]
		}
	}

	if err := ctx.Tasks.Set(key, ctx.Author, opts); err != nil {
		return nil, fmt.Errorf("task set: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Updated task %s", key)), nil
}

// taskRm soft-deletes a task. The backing document is not removed — the
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
