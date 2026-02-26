// task.go dispatches task subcommands and handles list.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// taskCmd dispatches to task subcommands. It peels off the first
// positional argument as the subcommand name and delegates to the
// appropriate handler. Column subcommands are nested one level deeper
// ("task column add", "task column rm", etc.).
func taskCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task: %w", sdk.ErrMissingArg)
	}

	sub := args[0]
	args = args[1:]

	switch sub {
	case "add":
		return taskAdd(ctx, args)
	case "list":
		return taskList(ctx, args)
	case "show":
		return taskShow(ctx, args)
	case "move":
		return taskMove(ctx, args)
	case "set":
		return taskSet(ctx, args)
	case "rm":
		return taskRm(ctx, args)
	case "restore":
		return taskRestore(ctx, args)
	case "column":
		if len(args) == 0 {
			return taskColumns(ctx, nil)
		}
		colSub := args[0]
		args = args[1:]
		switch colSub {
		case "list":
			return taskColumns(ctx, args)
		case "add":
			return taskAddColumn(ctx, args)
		case "rm":
			return taskRmColumn(ctx, args)
		case "mv", "move":
			return taskMvColumn(ctx, args)
		default:
			return nil, fmt.Errorf("task column: unknown subcommand %q", colSub)
		}
	case "link":
		return taskLink(ctx, args)
	case "links":
		return taskLinks(ctx, args)
	case "log":
		return taskLog(ctx, args)
	case "start":
		return taskStart(ctx, args)
	case "finish":
		return taskFinish(ctx, args)
	case "diff":
		return taskDiff(ctx, args)
	case "files":
		return taskFiles(ctx, args)
	default:
		return nil, fmt.Errorf("task: unknown subcommand: %s", sub)
	}
}

// taskList renders the task board. When filtered by column (--column or
// positional arg), shows a flat table for that column. Otherwise renders
// the full board view with all columns grouped under headings.
func taskList(_ sdk.Context, args []string) (sdk.Response, error) {
	var opts sdk.TaskListOpts

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--column":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task list: --column requires a value")
			}
			opts.Status = args[i]
		case "--assign":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task list: --assign requires a value")
			}
			opts.AssignedTo = args[i]
		case "--priority":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task list: --priority requires a value")
			}
			p, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("task list: invalid priority: %w", err)
			}
			opts.Priority = p
		default:
			// Positional arg = column name
			if opts.Status == "" && !strings.HasPrefix(args[i], "-") {
				opts.Status = args[i]
			}
		}
	}

	tasks, err := sdk.Tasks.List(opts)
	if err != nil {
		return nil, fmt.Errorf("task list: %w", err)
	}

	// If filtering by status, just show a flat table
	if opts.Status != "" {
		text := formatTaskTable(tasks)
		return sdk.Result{Text: text, Data: tasks}, nil
	}

	// Board view: group by column
	cols, err := sdk.Tasks.Columns()
	if err != nil {
		return nil, fmt.Errorf("task list: %w", err)
	}

	text := formatBoard(cols, tasks)
	return sdk.Result{Text: text, Data: tasks}, nil
}
