// config_git.go implements "llmd config git" subcommands for
// managing .llmd/.gitignore whitelist rules.
//
// Usage:
//
//	llmd config git              List gitignore rules
//	llmd config git ls           List gitignore rules
//	llmd config git allow <pat>  Allow a pattern to be committed
//	llmd config git deny <pat>   Stop allowing a pattern
package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func configGit(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return configGitList(ctx)
	}

	switch args[0] {
	case "ls", "list":
		return configGitList(ctx)
	case "allow":
		return configGitAllow(ctx, args[1:])
	case "deny":
		return configGitDeny(ctx, args[1:])
	default:
		return nil, fmt.Errorf("config git: unknown subcommand %q", args[0])
	}
}

func configGitList(ctx sdk.Context) (sdk.Response, error) {
	rules, err := ctx.Config.GitRules()
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return sdk.Text(""), nil
	}
	return sdk.Result{
		Text: strings.Join(rules, "\n"),
		Data: rules,
	}, nil
}

func configGitAllow(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("config git allow: %w: pattern required", sdk.ErrMissingArg)
	}
	if err := ctx.Config.GitAllow(args[0]); err != nil {
		return nil, fmt.Errorf("config git allow: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Allowed %s in .llmd/.gitignore", args[0])), nil
}

func configGitDeny(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("config git deny: %w: pattern required", sdk.ErrMissingArg)
	}
	if err := ctx.Config.GitDeny(args[0]); err != nil {
		return nil, fmt.Errorf("config git deny: %w", err)
	}
	return sdk.Text(fmt.Sprintf("Denied %s in .llmd/.gitignore", args[0])), nil
}
