// importcmd.go imports files from the filesystem into the store.
//
// Usage:
//
//	llmd import <dir>                    Import .md files from directory
//	llmd import --prefix docs/ <dir>     Import under a path prefix
//	llmd import --dry-run <dir>          Show what would be imported
//	llmd import --force <dir>            Re-import even if unchanged

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func importCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	var opts sdk.ImportOpts
	var dir string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--prefix" && i+1 < len(args):
			i++
			opts.Prefix = args[i]
		case strings.HasPrefix(args[i], "--prefix="):
			opts.Prefix = strings.TrimPrefix(args[i], "--prefix=")
		case args[i] == "--dry-run":
			opts.DryRun = true
		case args[i] == "--force":
			opts.Force = true
		default:
			dir = args[i]
		}
	}

	if dir == "" {
		return nil, fmt.Errorf("import: %w", sdk.ErrMissingArg)
	}

	result, err := sdk.Documents.Import(dir, opts)
	if err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}

	var lines []string
	for _, p := range result.Created {
		lines = append(lines, fmt.Sprintf("created %s", p))
	}
	for _, p := range result.Updated {
		lines = append(lines, fmt.Sprintf("updated %s", p))
	}
	if len(result.Skipped) > 0 {
		lines = append(lines, fmt.Sprintf("skipped %d unchanged", len(result.Skipped)))
	}

	return sdk.Result{Text: strings.Join(lines, "\n"), Data: result}, nil
}
