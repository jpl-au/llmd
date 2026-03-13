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

var importSpec = sdk.Command{
	Name: "import", Desc: `Import .md files from a directory into the store

Reads .md files from <dir> and creates documents in the store. File
paths relative to the directory become document paths. Unchanged
files are skipped unless --force is used.`, Usage: "import [options] <dir>", NeedsAuthor: true, Flags: []sdk.Flag{
		{Name: "prefix", Type: "string", Desc: "Target path prefix for imported documents"},
		{Name: "dry-run", Type: "bool", Desc: "Preview without importing"},
		{Name: "force", Type: "bool", Desc: "Import even if unchanged"},
	},
}

func importCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(importSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}

	var dir string
	if len(positional) > 0 {
		dir = positional[0]
	}
	if dir == "" {
		return nil, fmt.Errorf("import: %w", sdk.ErrMissingArg)
	}
	opts := sdk.ImportOpts{
		Prefix: flags.String("prefix"),
		DryRun: flags.Bool("dry-run"),
		Force:  flags.Bool("force"),
	}

	result, err := ctx.Documents.Import(dir, opts)
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
