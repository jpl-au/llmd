// task_column.go handles column subcommands: list, add, rm, mv,
// set, unset, show. Also provides pipeline display.

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// taskColumns lists all board columns in display order, one per line.
func taskColumns(ctx sdk.Context, _ []string) (sdk.Response, error) {
	cols, err := ctx.Tasks.Columns()
	if err != nil {
		return nil, fmt.Errorf("task columns: %w", err)
	}
	return sdk.Result{Text: strings.Join(cols, "\n"), Data: cols}, nil
}

// taskAddColumn adds a new column to the board. Takes a column name as
// a positional argument and an optional --after flag to control placement.
var taskColFlags = []sdk.Flag{
	{Name: "after", Type: "string"},
}

func taskAddColumn(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskColFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task column add: %w", err)
	}
	if len(positional) == 0 {
		return nil, fmt.Errorf("task column add: %w: name", sdk.ErrMissingArg)
	}

	name := positional[0]
	after := flags.String("after")

	if err := ctx.Tasks.AddColumn(name, after, ctx.Author); err != nil {
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

	if err := ctx.Tasks.RemoveColumn(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("task column rm: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Removed column %s", args[0])), nil
}

// taskMvColumn reorders a column to appear after another. Requires both
// a column name and --after flag specifying the target position.
func taskMvColumn(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskColFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task column mv: %w", err)
	}
	if len(positional) == 0 {
		return nil, fmt.Errorf("task column mv: %w: name", sdk.ErrMissingArg)
	}

	name := positional[0]
	after := flags.String("after")
	if after == "" {
		return nil, fmt.Errorf("task column mv: --after is required")
	}

	if err := ctx.Tasks.MoveColumn(name, after, ctx.Author); err != nil {
		return nil, fmt.Errorf("task column mv: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Moved column %s after %s", name, after)), nil
}

var taskStepFlags = []sdk.Flag{
	{Name: "agent", Type: "string", Desc: "Agent to auto-spawn"},
	{Name: "role", Type: "string", Desc: "Agent role (developer, tester, auditor)"},
	{Name: "on-success", Type: "string", Desc: "Column to move to on success"},
	{Name: "on-failure", Type: "string", Desc: "Column to move to on failure"},
}

func taskColumnSet(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(taskStepFlags, args)
	if err != nil {
		return nil, fmt.Errorf("task column set: %w", err)
	}
	if len(positional) == 0 {
		return nil, fmt.Errorf("task column set: %w: column name", sdk.ErrMissingArg)
	}

	name := positional[0]
	agent := flags.String("agent")
	role := flags.String("role")
	if agent == "" {
		return nil, fmt.Errorf("task column set: --agent is required")
	}
	if role == "" {
		return nil, fmt.Errorf("task column set: --role is required")
	}

	cfg := sdk.StepConfig{
		Agent:     agent,
		Role:      role,
		OnSuccess: flags.String("on-success"),
		OnFailure: flags.String("on-failure"),
	}

	if err := ctx.Tasks.SetStep(name, cfg, ctx.Author); err != nil {
		return nil, fmt.Errorf("task column set: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Configured %s: agent=%s role=%s", name, agent, role)), nil
}

func taskColumnUnset(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task column unset: %w: column name", sdk.ErrMissingArg)
	}

	if err := ctx.Tasks.UnsetStep(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("task column unset: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Removed pipeline config from %s", args[0])), nil
}

func taskColumnShow(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("task column show: %w: column name", sdk.ErrMissingArg)
	}

	step, err := ctx.Tasks.Step(args[0])
	if err != nil {
		return nil, fmt.Errorf("task column show: %w", err)
	}
	if step == nil {
		return sdk.Text(fmt.Sprintf("%s: no pipeline config", args[0])), nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Column:     %s", args[0]))
	lines = append(lines, fmt.Sprintf("Agent:      %s", step.Agent))
	lines = append(lines, fmt.Sprintf("Role:       %s", step.Role))
	if step.OnSuccess != "" {
		lines = append(lines, fmt.Sprintf("On success: %s", step.OnSuccess))
	}
	if step.OnFailure != "" {
		lines = append(lines, fmt.Sprintf("On failure: %s", step.OnFailure))
	}

	return sdk.Result{Text: strings.Join(lines, "\n"), Data: step}, nil
}

func taskPipeline(ctx sdk.Context, _ []string) (sdk.Response, error) {
	cols, err := ctx.Tasks.Columns()
	if err != nil {
		return nil, fmt.Errorf("task pipeline: %w", err)
	}

	var lines []string
	for _, col := range cols {
		step, err := ctx.Tasks.Step(col)
		if err != nil {
			continue
		}
		if step == nil {
			lines = append(lines, fmt.Sprintf("%-15s  -", col))
			continue
		}
		parts := []string{fmt.Sprintf("%-15s  %s (%s)", col, step.Agent, step.Role)}
		if step.OnSuccess != "" {
			parts = append(parts, fmt.Sprintf("-> %s", step.OnSuccess))
		}
		if step.OnFailure != "" {
			parts = append(parts, fmt.Sprintf("!! %s", step.OnFailure))
		}
		lines = append(lines, strings.Join(parts, "  "))
	}

	return sdk.Text(strings.Join(lines, "\n")), nil
}
