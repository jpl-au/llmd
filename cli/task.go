// task.go dispatches task subcommands and handles add/list/show.

package cli

import (
	"fmt"
	"os"
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
	case "diff":
		return taskDiff(ctx, args)
	case "files":
		return taskFiles(ctx, args)
	default:
		return nil, fmt.Errorf("task: unknown subcommand: %s", sub)
	}
}

// taskAdd creates a new task with an optional spec body from stdin or --file.
func taskAdd(ctx sdk.Context, args []string) (sdk.Response, error) {
	var opts sdk.TaskAddOpts
	opts.Author = ctx.Author

	var positional []string
	var file string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--column":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task add: --column requires a value")
			}
			opts.Status = args[i]
		case "--priority":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task add: --priority requires a value")
			}
			p, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("task add: invalid priority: %w", err)
			}
			opts.Priority = p
		case "--assign":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task add: --assign requires a value")
			}
			opts.AssignedTo = args[i]
		case "--path":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task add: --path requires a value")
			}
			opts.Path = args[i]
		case "--file":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task add: --file requires a value")
			}
			file = args[i]
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		return nil, fmt.Errorf("task add: %w: title", sdk.ErrMissingArg)
	}
	if file != "" && opts.Path != "" {
		return nil, fmt.Errorf("task add: --file and --path are mutually exclusive")
	}

	// --file reads content from the filesystem
	body := ctx.Stdin
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("task add: %w", err)
		}
		body = data
	}

	title := strings.Join(positional, " ")
	t, err := sdk.Tasks.Add(title, body, opts)
	if err != nil {
		return nil, fmt.Errorf("task add: %w", err)
	}

	text := fmt.Sprintf("Created task %s \"%s\" in %s", t.Key, t.Title, t.Status)
	if ok, _ := sdk.Documents.Exists(t.Path); ok {
		text += fmt.Sprintf("\nSpec: %s", t.Path)
	}
	return sdk.Result{Text: text, Data: t}, nil
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

// taskShow displays a single task's metadata and spec body. Renders a
// markdown document with a metadata table (ID, status, priority,
// assignee, branch, flags, spec path) followed by the spec document
// content if it exists.
func taskShow(_ sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task show: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.Tasks.Read(args[0])
	if err != nil {
		return nil, fmt.Errorf("task show: %w", err)
	}

	// Read the document body
	body, err := sdk.Documents.Read(t.Path, 0)
	if err != nil {
		body = nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", t.Title)
	fmt.Fprintf(&b, "| Field | Value |\n")
	fmt.Fprintf(&b, "|-------|-------|\n")
	fmt.Fprintf(&b, "| ID | %s |\n", t.Key)
	fmt.Fprintf(&b, "| Status | %s |\n", t.Status)
	fmt.Fprintf(&b, "| Priority | %d |\n", t.Priority)
	if t.AssignedTo != "" {
		fmt.Fprintf(&b, "| Assigned To | %s |\n", t.AssignedTo)
	}
	if t.Branch != "" {
		fmt.Fprintf(&b, "| Branch | %s |\n", t.Branch)
	}
	if t.Flags != "" {
		fmt.Fprintf(&b, "| Flags | %s |\n", t.Flags)
	}
	fmt.Fprintf(&b, "| Spec | %s |\n", t.Path)
	fmt.Fprintf(&b, "\n---\n\n")

	if body != nil {
		b.Write(body)
	}

	return sdk.Result{Text: b.String(), Data: t}, nil
}
