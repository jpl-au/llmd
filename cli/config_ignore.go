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

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/sdk"
)

func configIgnore(_ sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return configIgnoreList()
	}

	switch args[0] {
	case "ls", "list":
		return configIgnoreList()
	case "add":
		return configIgnoreAdd(args[1:])
	case "rm", "remove":
		return configIgnoreRm(args[1:])
	default:
		return nil, fmt.Errorf("config ignore: unknown subcommand %q", args[0])
	}
}

func configIgnoreList() (sdk.Response, error) {
	patterns, err := config.IgnorePatterns()
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

func configIgnoreAdd(args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("config ignore add: %w: pattern required", sdk.ErrMissingArg)
	}
	if err := config.AddIgnore(args[0]); err != nil {
		return nil, fmt.Errorf("config ignore add: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Added %s to .llmd/.gitignore", args[0])), nil
}

func configIgnoreRm(args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("config ignore rm: %w: pattern required", sdk.ErrMissingArg)
	}
	if err := config.RemoveIgnore(args[0]); err != nil {
		return nil, fmt.Errorf("config ignore rm: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Removed %s from .llmd/.gitignore", args[0])), nil
}
