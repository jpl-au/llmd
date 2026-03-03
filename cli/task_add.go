// task_add.go handles creating new tasks.

package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

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
	t, err := ctx.Tasks.Add(title, body, opts)
	if err != nil {
		return nil, fmt.Errorf("task add: %w", err)
	}

	text := fmt.Sprintf("Created task %s \"%s\" in %s", t.Key, t.Title, t.Status)
	if ok, _ := ctx.Documents.Exists(t.Path); ok {
		text += fmt.Sprintf("\nSpec: %s", t.Path)
	}
	return sdk.Result{Text: text, Data: t}, nil
}
