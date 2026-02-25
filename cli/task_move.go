// task_move.go handles move, set, rm, and restore subcommands.

package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/jpl-au/llmd/sdk"
)

func taskMove(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("task move: %w: id and column", sdk.ErrMissingArg)
	}

	if err := sdk.Tasks.Move(args[0], args[1], ctx.Author); err != nil {
		if errors.Is(err, sdk.ErrNoSpec) {
			tsk, rerr := sdk.Tasks.Read(args[0])
			if rerr == nil {
				return nil, fmt.Errorf("task move: task has no spec — write a document with `llmd write %s` or link an existing one with `task link %s <path>`", tsk.Path, args[0])
			}
		}
		return nil, fmt.Errorf("task move: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Moved task %s to %s", args[0], args[1])), nil
}

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

	if err := sdk.Tasks.Set(key, ctx.Author, opts); err != nil {
		return nil, fmt.Errorf("task set: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Updated task %s", key)), nil
}

func taskRm(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task rm: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.Tasks.Delete(args[0], ctx.Author)
	if err != nil {
		return nil, fmt.Errorf("task rm: %w", err)
	}

	text := fmt.Sprintf("Removed task %s \"%s\"", t.Key, t.Title)
	if ok, _ := sdk.Documents.Exists(t.Path); ok {
		text += fmt.Sprintf("\nNote: the document at %s still exists. To remove it: llmd rm %s", t.Path, t.Path)
	}
	return sdk.Text(text), nil
}

func taskRestore(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task restore: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.Tasks.Restore(args[0], ctx.Author)
	if err != nil {
		return nil, fmt.Errorf("task restore: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Restored task %s \"%s\"", t.Key, t.Title)), nil
}
