// task_column.go handles column subcommands: list, add, rm, mv.

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// taskColumns lists all board columns in display order, one per line.
func taskColumns(_ sdk.Context, _ []string) (sdk.Response, error) {
	cols, err := sdk.Tasks.Columns()
	if err != nil {
		return nil, fmt.Errorf("task columns: %w", err)
	}
	return sdk.Result{Text: strings.Join(cols, "\n"), Data: cols}, nil
}

// taskAddColumn adds a new column to the board. Takes a column name as
// a positional argument and an optional --after flag to control placement.
func taskAddColumn(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task column add: %w: name", sdk.ErrMissingArg)
	}

	name := args[0]
	var after string
	for i := 1; i < len(args); i++ {
		if args[i] == "--after" && i+1 < len(args) {
			i++
			after = args[i]
		}
	}

	if err := sdk.Tasks.AddColumn(name, after, ctx.Author); err != nil {
		return nil, fmt.Errorf("task column add: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Added column %s", name)), nil
}

// taskRmColumn removes an empty column from the board. Fails if any
// tasks still occupy the column.
func taskRmColumn(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task column rm: %w: name", sdk.ErrMissingArg)
	}

	if err := sdk.Tasks.RemoveColumn(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("task column rm: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Removed column %s", args[0])), nil
}

// taskMvColumn reorders a column to appear after another. Requires both
// a column name and --after flag specifying the target position.
func taskMvColumn(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task column mv: %w: name", sdk.ErrMissingArg)
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
		return nil, fmt.Errorf("task column mv: --after is required")
	}

	if err := sdk.Tasks.MoveColumn(name, after, ctx.Author); err != nil {
		return nil, fmt.Errorf("task column mv: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Moved column %s after %s", name, after)), nil
}
