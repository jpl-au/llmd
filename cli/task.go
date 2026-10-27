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
  list [--since 5m] [--all]  board view (done shows its newest 10 unless --all)
  show <id>                 task metadata + spec body
  move <id> <column>        move task to column (needs spec to leave backlog)
  next [--from col] [--to col]  claim the first ready task: move + assign in one write
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
  commits [id]              list commits on task's branch`, Usage: "task <subcommand> [options]", MCP: true, MCPName: "task", MCPDesc: `Task board. Columns: backlog, up-next, in-progress, review, approval, done, blocked.
Subcommands: list [--column c] [--all] (board; done shows its newest 10), show <id>, add <title> (spec via content; --column, --priority, --assign, --depends-on), next [--from c] [--to c] [--assign who] (claim the first ready task: move + assign in one write, returns its spec), move <id> <column>, set <id> (--title, --priority, --assign, --flag, --unflag, --depends-on, --branch), start <id>, finish [id], branch <id>, rm <id>, restore <id>, link <id> <path>, links <id>, log <id>, chain <id>, ready <id>, diff|files|commits [id], column list|add|rm|mv.
Flags blocked and hold pause a task; human marks it for the user: leave those alone.`, Flags: []sdk.Flag{
		{Name: "column", Type: "string", Desc: "Filter by column"},
		{Name: "priority", Type: "int", Desc: "Filter or set priority"},
		{Name: "assign", Type: "string", Desc: "Filter or set assigned to"},
		{Name: "branch", Type: "string", Desc: "Git branch for this task"},
		{Name: "depends-on", Type: "string", Desc: "Task key this task depends on"},
		{Name: "path", Type: "string", Desc: "Use existing store document as spec"},
		{Name: "file", Type: "string", Desc: "Read spec from filesystem path"},
		{Name: "flag", Type: "string", Desc: "Set a flag (blocked, hold, human)"},
		{Name: "unflag", Type: "string", Desc: "Remove a flag"},
		{Name: "position", Type: "int", Desc: "Set position within column"},
		{Name: "after", Type: "string", Desc: "Insert/move column after this one"},
		{Name: "base", Type: "string", Desc: "Base branch for diff (default: main/master)"},
		{Name: "stat", Type: "bool", Desc: "Show diffstat instead of full diff"},
		{Name: "from", Type: "string", Desc: "Column next takes from (default up-next)"},
		{Name: "to", Type: "string", Desc: "Column next moves to (default in-progress)"},
		{Name: "all", Type: "bool", Desc: "List every done task"},
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
	case "add", "move", "next", "set", "rm", "restore", "link", "start", "finish", "branch":
		if ctx.Author == "" {
			return nil, fmt.Errorf("task %s: %w: author required for mutations", sub, sdk.ErrMissingArg)
		}
	case "column":
		if len(args) > 0 {
			switch args[0] {
			case "add", "rm", "mv", "move":
				if ctx.Author == "" {
					return nil, fmt.Errorf("task column %s: %w: author required for mutations", args[0], sdk.ErrMissingArg)
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
	case "next":
		return taskNext(ctx, args)
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
			return nil, fmt.Errorf("task column: %w: unknown subcommand %q", sdk.ErrInvalidArg, colSub)
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
		return nil, fmt.Errorf("task: %w: unknown subcommand %q", sdk.ErrInvalidArg, sub)
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
	{Name: "all", Type: "bool", Desc: "List every done task"},
}

// doneShown is how many done tasks the board view lists without --all.
// Done is the one column that only grows, so an old board would
// otherwise spend most of its output, and an agent's context, on
// finished work.
const doneShown = 10

// trimDone keeps the newest keep tasks of the done column and every
// task of every other column. Tasks arrive in board order, where the
// newest done task is last. Returns the tasks kept and how many done
// tasks were left out.
func trimDone(tasks []*sdk.Task, keep int) ([]*sdk.Task, int) {
	var done int
	for _, t := range tasks {
		if t.Status == "done" {
			done++
		}
	}
	hidden := done - keep
	if hidden <= 0 {
		return tasks, 0
	}
	out := make([]*sdk.Task, 0, len(tasks)-hidden)
	skip := hidden
	for _, t := range tasks {
		if t.Status == "done" && skip > 0 {
			skip--
			continue
		}
		out = append(out, t)
	}
	return out, hidden
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

	var hidden int
	if !flags.Bool("all") {
		tasks, hidden = trimDone(tasks, doneShown)
	}
	text := formatBoard(ctx, cols, tasks, hidden)
	return sdk.Result{Text: text, Data: tasks}, nil
}
