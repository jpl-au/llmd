// config_ignore.go implements "llmd config ignore" subcommands for
// managing .llmd/.gitignore patterns.
//
// Usage:
//
//	llmd config ignore              List ignore patterns
//	llmd config ignore ls           List ignore patterns
//	llmd config ignore add <pat>    Add an ignore pattern
//	llmd config ignore rm <pat>     Remove an ignore pattern
package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func configIgnore(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return configIgnoreList(ctx)
	}

	switch args[0] {
	case "ls", "list":
		return configIgnoreList(ctx)
	case "add":
		return configIgnoreAdd(ctx, args[1:])
	case "rm", "remove":
		return configIgnoreRm(ctx, args[1:])
	default:
		return nil, fmt.Errorf("config ignore: unknown subcommand %q", args[0])
	}
}

func configIgnoreList(ctx sdk.Context) (sdk.Response, error) {
	patterns, err := ctx.Config.IgnorePatterns()
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		return sdk.Text(""), nil
	}
	return sdk.Result{
		Text: strings.Join(patterns, "\n"),
		Data: patterns,
	}, nil
}

func configIgnoreAdd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("config ignore add: %w: pattern required", sdk.ErrMissingArg)
	}
	if err := ctx.Config.AddIgnore(args[0]); err != nil {
		return nil, fmt.Errorf("config ignore add: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Added %s to .llmd/.gitignore", args[0])), nil
}

func configIgnoreRm(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("config ignore rm: %w: pattern required", sdk.ErrMissingArg)
	}
	if err := ctx.Config.RemoveIgnore(args[0]); err != nil {
		return nil, fmt.Errorf("config ignore rm: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Removed %s from .llmd/.gitignore", args[0])), nil
}
