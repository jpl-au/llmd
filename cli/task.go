// task.go dispatches task subcommands and handles list.

package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var taskSpec = sdk.Command{
	Name: "task", Desc: `Manage tasks on the board.

Subcommands (passed as first arg):
  add <title>               create task (body via content/stdin)
  list [--since 5m]          board view (all columns)
  show <id>                 task metadata + spec body
  move <id> <column>        move task to column (needs spec to leave backlog)
  set <id> [flags]          update metadata
  rm <id>                   soft-delete task
  restore <id>              restore deleted task
  column list               list columns
  column add <name>         add column
  column rm <name>          remove empty column
  column mv <name> --after  reorder column
  link <id> <path>          link task to document
  links <id>                list linked documents
  log <id> [-n limit]       audit history for a task
  start <id> [--assign agent]  start task (--assign spawns an agent)
  finish [id]               complete task (move to done, show summary)
  branch <id>               create git branch from task, checkout, start
  chain <id>                show dependency chain
  ready <id>                check if dependencies are satisfied
  diff [id]                 show git diff for task's branch
  files [id]                list files changed on task's branch
  commits [id]              list commits on task's branch`, Usage: "task <subcommand> [options]", MCP: true, MCPName: "task", Flags: []sdk.Flag{
		{Name: "column", Type: "string", Desc: "Filter by column"},
		{Name: "priority", Type: "int", Desc: "Filter or set priority"},
		{Name: "assign", Type: "string", Desc: "Filter or set assigned to"},
		{Name: "branch", Type: "string", Desc: "Git branch for this task"},
		{Name: "depends-on", Type: "string", Desc: "Task key this task depends on"},
		{Name: "path", Type: "string", Desc: "Use existing store document as spec"},
		{Name: "file", Type: "string", Desc: "Read spec from filesystem path"},
		{Name: "flag", Type: "string", Desc: "Set a flag (blocked, hold)"},
		{Name: "unflag", Type: "string", Desc: "Remove a flag"},
		{Name: "position", Type: "int", Desc: "Set position within column"},
		{Name: "after", Type: "string", Desc: "Insert/move column after this one"},
		{Name: "base", Type: "string", Desc: "Base branch for diff (default: main/master)"},
		{Name: "stat", Type: "bool", Desc: "Show diffstat instead of full diff"},
	},
}

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

	// Checked here, not via NeedsAuthor - reads don't need an author.
	switch sub {
	case "add", "move", "set", "rm", "restore", "link", "start", "finish", "branch":
		if ctx.Author == "" {
			return nil, fmt.Errorf("task %s: author required for mutations", sub)
		}
	case "column":
		if len(args) > 0 {
			switch args[0] {
			case "add", "rm", "mv", "move":
				if ctx.Author == "" {
					return nil, fmt.Errorf("task column %s: author required for mutations", args[0])
				}
			}
		}
	}

	switch sub {
	case "add":
		return taskAdd(ctx, args)
	case "list", "ls", "board":
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
		case "list", "ls":
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
	case "branch":
		return taskBranch(ctx, args)
	case "chain":
		return taskChain(ctx, args)
	case "ready":
		return taskReady(ctx, args)
	case "diff":
		return taskDiff(ctx, args)
	case "files":
		return taskFiles(ctx, args)
	case "commits":
		return taskCommits(ctx, args)
	default:
		return nil, fmt.Errorf("task: unknown subcommand: %s", sub)
	}
}

// taskList renders the task board. When filtered by column (--column or
// positional arg), shows a flat table for that column. Otherwise renders
// the full board view with all columns grouped under headings.
var taskListFlags = []sdk.Flag{
	{Name: "column", Type: "string"},
	{Name: "assign", Type: "string"},
	{Name: "priority", Type: "int"},
	{Name: "since", Type: "string", Desc: "Show tasks created after (e.g. 5m, 1h, RFC 3339)"},
}

func taskList(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskListFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task list: %w", err)
	}
	since, err := sdk.ParseSince(flags.String("since"))
	if err != nil {
		return nil, fmt.Errorf("task list: %w", err)
	}
	opts := sdk.TaskListOpts{
		Status:     flags.String("column"),
		AssignedTo: flags.String("assign"),
		Priority:   flags.Int("priority"),
		Since:      since,
	}
	if opts.Status == "" && len(positional) > 0 {
		opts.Status = positional[0]
	}

	tasks, err := ctx.Tasks.List(opts)
	if err != nil {
		return nil, fmt.Errorf("task list: %w", err)
	}

	// If filtering by status, just show a flat table
	if opts.Status != "" {
		text := formatTaskTable(ctx, tasks)
		return sdk.Result{Text: text, Data: tasks}, nil
	}

	// Board view: group by column
	cols, err := ctx.Tasks.Columns()
	if err != nil {
		return nil, fmt.Errorf("task list: %w", err)
	}

	text := formatBoard(ctx, cols, tasks)
	return sdk.Result{Text: text, Data: tasks}, nil
}
