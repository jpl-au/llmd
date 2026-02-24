// task.go provides task management commands.
//
// Usage:
//
//	llmd task add <title>                Create a task (--column to set column)
//	llmd task list                       List all tasks (board view)
//	llmd task show <id>                  Show task + spec
//	llmd task move <id> <column>         Move task to column
//	llmd task set <id> [flags]           Update task metadata
//	llmd task rm <id>                    Soft-delete task
//	llmd task restore <id>               Restore deleted task
//	llmd task columns                    List columns
//	llmd task add-column <name>          Add a column
//	llmd task rm-column <name>           Remove empty column
//	llmd task mv-column <name> --after   Reorder column
//	llmd task link <id> <path>           Link task to document
//	llmd task links <id>                 List linked documents
//	llmd task log <id>                   Show audit history

package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/jpl-au/llmd/sdk"
)

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
	default:
		return nil, fmt.Errorf("task: unknown subcommand: %s", sub)
	}
}

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
	t, err := sdk.API.TaskAdd(title, body, opts)
	if err != nil {
		return nil, fmt.Errorf("task add: %w", err)
	}

	text := fmt.Sprintf("Created task %s \"%s\" in %s", t.Key, t.Title, t.Status)
	if ok, _ := sdk.API.Exists(t.Path); ok {
		text += fmt.Sprintf("\nSpec: %s", t.Path)
	}
	return sdk.Result{Text: text, Data: t}, nil
}

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

	tasks, err := sdk.API.TaskList(opts)
	if err != nil {
		return nil, fmt.Errorf("task list: %w", err)
	}

	// If filtering by status, just show a flat table
	if opts.Status != "" {
		text := formatTaskTable(tasks)
		return sdk.Result{Text: text, Data: tasks}, nil
	}

	// Board view: group by column
	cols, err := sdk.API.TaskColumns()
	if err != nil {
		return nil, fmt.Errorf("task list: %w", err)
	}

	text := formatBoard(cols, tasks)
	return sdk.Result{Text: text, Data: tasks}, nil
}

func taskShow(_ sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task show: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.API.TaskRead(args[0])
	if err != nil {
		return nil, fmt.Errorf("task show: %w", err)
	}

	// Read the document body
	body, err := sdk.API.Read(t.Path, 0)
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

func taskMove(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("task move: %w: id and column", sdk.ErrMissingArg)
	}

	if err := sdk.API.TaskMove(args[0], args[1], ctx.Author); err != nil {
		if errors.Is(err, sdk.ErrNoSpec) {
			tsk, rerr := sdk.API.TaskRead(args[0])
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
		}
	}

	if err := sdk.API.TaskSet(key, ctx.Author, opts); err != nil {
		return nil, fmt.Errorf("task set: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Updated task %s", key)), nil
}

func taskRm(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task rm: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.API.TaskDelete(args[0], ctx.Author)
	if err != nil {
		return nil, fmt.Errorf("task rm: %w", err)
	}

	text := fmt.Sprintf("Removed task %s \"%s\"\nNote: the document at %s still exists. To remove it: llmd rm %s",
		t.Key, t.Title, t.Path, t.Path)
	return sdk.Text(text), nil
}

func taskRestore(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task restore: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.API.TaskRestore(args[0], ctx.Author)
	if err != nil {
		return nil, fmt.Errorf("task restore: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Restored task %s \"%s\"", t.Key, t.Title)), nil
}

func taskColumns(_ sdk.Context, _ []string) (sdk.Response, error) {
	cols, err := sdk.API.TaskColumns()
	if err != nil {
		return nil, fmt.Errorf("task columns: %w", err)
	}
	return sdk.Result{Text: strings.Join(cols, "\n"), Data: cols}, nil
}

func taskAddColumn(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task add-column: %w: name", sdk.ErrMissingArg)
	}

	name := args[0]
	var after string
	for i := 1; i < len(args); i++ {
		if args[i] == "--after" && i+1 < len(args) {
			i++
			after = args[i]
		}
	}

	if err := sdk.API.TaskAddColumn(name, after, ctx.Author); err != nil {
		return nil, fmt.Errorf("task add-column: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Added column %s", name)), nil
}

func taskRmColumn(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task rm-column: %w: name", sdk.ErrMissingArg)
	}

	if err := sdk.API.TaskRemoveColumn(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("task rm-column: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Removed column %s", args[0])), nil
}

func taskMvColumn(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task mv-column: %w: name", sdk.ErrMissingArg)
	}

	name := args[0]
	var after string
	for i := 1; i < len(args); i++ {
		if args[i] == "--after" && i+1 < len(args) {
			i++
			after = args[i]
		}
	}

	if after == "" {
		return nil, fmt.Errorf("task mv-column: --after is required")
	}

	if err := sdk.API.TaskMoveColumn(name, after, ctx.Author); err != nil {
		return nil, fmt.Errorf("task mv-column: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Moved column %s after %s", name, after)), nil
}

func taskLink(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("task link: %w: id and path", sdk.ErrMissingArg)
	}

	// Look up the task to get its document path
	t, err := sdk.API.TaskRead(args[0])
	if err != nil {
		return nil, fmt.Errorf("task link: %w", err)
	}

	if err := sdk.API.LinkAdd(t.Path, args[1], "", ctx.Author); err != nil {
		return nil, fmt.Errorf("task link: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Linked task %s to %s", args[0], args[1])), nil
}

func taskLinks(_ sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task links: %w: id", sdk.ErrMissingArg)
	}

	t, err := sdk.API.TaskRead(args[0])
	if err != nil {
		return nil, fmt.Errorf("task links: %w", err)
	}

	links, err := sdk.API.LinkList(t.Path, "out")
	if err != nil {
		return nil, fmt.Errorf("task links: %w", err)
	}

	var lines []string
	for _, l := range links {
		if l.Label != "" {
			lines = append(lines, fmt.Sprintf("%s (%s)", l.To, l.Label))
		} else {
			lines = append(lines, l.To)
		}
	}
	return sdk.Result{Text: strings.Join(lines, "\n"), Data: links}, nil
}

func taskLog(_ sdk.Context, args []string) (sdk.Response, error) {
	var key string
	limit := 0

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("task log: -n requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return nil, fmt.Errorf("task log: invalid limit: %w", err)
			}
			limit = n
		default:
			if key == "" && !strings.HasPrefix(args[i], "-") {
				key = args[i]
			}
		}
	}

	events, err := sdk.API.TaskLog(key, limit)
	if err != nil {
		return nil, fmt.Errorf("task log: %w", err)
	}

	if len(events) == 0 {
		return sdk.Result{Text: emptyCol.Render("no history"), Data: events}, nil
	}

	// Show TASK column when listing all history
	showSubject := key == ""

	headers := []string{"TIME", "ACTOR", "ACTION", "OLD", "NEW"}
	if showSubject {
		headers = []string{"TIME", "TASK", "ACTOR", "ACTION", "OLD", "NEW"}
	}

	t := table.New().
		Headers(headers...).
		Border(lipgloss.RoundedBorder()).
		BorderRow(false).
		BorderStyle(tblBorder).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return tblHeader
			}
			if col == 0 {
				return tblDim
			}
			return tblCell
		})

	for _, e := range events {
		ts := time.UnixMilli(e.Timestamp).Format("2006-01-02 15:04")
		if showSubject {
			t.Row(ts, e.Subject, e.Actor, e.Action, e.OldValue, e.NewValue)
		} else {
			t.Row(ts, e.Actor, e.Action, e.OldValue, e.NewValue)
		}
	}

	return sdk.Result{Text: t.String(), Data: events}, nil
}

// Styles for the board view. Generic table styles (tblHeader, tblCell,
// tblDim, tblBorder) live in styles.go.
var (
	colHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")). // bright blue
			PaddingLeft(1)

	colCount = lipgloss.NewStyle().
			Faint(true)

	emptyCol = lipgloss.NewStyle().
			Faint(true).
			Italic(true).
			PaddingLeft(3)

	flagBlocked = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")). // red
			Padding(0, 1)

	flagHold = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")). // yellow
			Padding(0, 1)
)

// formatBoard renders the board view grouped by column.
func formatBoard(cols []string, tasks []*sdk.Task) string {
	specs := specExists(tasks)

	byStatus := make(map[string][]*sdk.Task)
	for _, t := range tasks {
		byStatus[t.Status] = append(byStatus[t.Status], t)
	}

	var b strings.Builder
	for i, col := range cols {
		if i > 0 {
			b.WriteByte('\n')
		}

		tt := byStatus[col]
		heading := colHeader.Render(strings.ToUpper(col)) +
			" " + colCount.Render(fmt.Sprintf("(%d)", len(tt)))
		b.WriteString(heading)
		b.WriteByte('\n')

		if len(tt) == 0 {
			b.WriteString(emptyCol.Render("no tasks"))
			b.WriteByte('\n')
		} else {
			b.WriteString(taskTable(tt, specs))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// formatTaskTable renders a flat table of tasks.
func formatTaskTable(tasks []*sdk.Task) string {
	if len(tasks) == 0 {
		return emptyCol.Render("no tasks")
	}
	return taskTable(tasks, specExists(tasks))
}

// specExists checks which tasks have a document at their path.
func specExists(tasks []*sdk.Task) map[string]bool {
	m := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		if t.Path == "" {
			continue
		}
		exists, err := sdk.API.Exists(t.Path)
		if err == nil && exists {
			m[t.Key] = true
		}
	}
	return m
}

// taskTable renders a terminal table using lipgloss.
func taskTable(tasks []*sdk.Task, specs map[string]bool) string {
	t := table.New().
		Headers("ID", "PRI", "TITLE", "ASSIGNED TO", "SPEC", "FLAGS").
		Border(lipgloss.RoundedBorder()).
		BorderRow(false).
		BorderStyle(tblBorder).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return tblHeader
			}
			// Colour flags by value
			if col == 5 && row >= 0 && row < len(tasks) {
				f := tasks[row].Flags
				if strings.Contains(f, "blocked") {
					return flagBlocked
				}
				if strings.Contains(f, "hold") {
					return flagHold
				}
			}
			// Dim empty cells (assigned to, spec)
			if col == 3 || col == 4 {
				return tblDim
			}
			return tblCell
		})

	for _, tk := range tasks {
		spec := ""
		if specs[tk.Key] {
			spec = tk.Path
		}
		t.Row(
			tk.Key,
			strconv.Itoa(tk.Priority),
			tk.Title,
			tk.AssignedTo,
			spec,
			tk.Flags,
		)
	}

	return t.String()
}
