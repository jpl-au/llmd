// task_add.go handles creating new tasks.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// taskAdd creates a new task with an optional spec body from stdin or --file.
var taskAddFlags = []sdk.Flag{
	{Name: "column", Type: "string"},
	{Name: "priority", Type: "int"},
	{Name: "assign", Type: "string"},
	{Name: "depends-on", Type: "string"},
	{Name: "path", Type: "string"},
	{Name: "file", Type: "string"},
}

func taskAdd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskAddFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task add: %w", err)
	}

	opts := sdk.TaskAddOpts{
		Author:     ctx.Author,
		Status:     flags.String("column"),
		Priority:   flags.Int("priority"),
		AssignedTo: flags.String("assign"),
		DependsOn:  flags.String("depends-on"),
		Path:       flags.String("path"),
	}
	file := flags.String("file")

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
