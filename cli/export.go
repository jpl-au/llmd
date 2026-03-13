// export.go exports documents from the store to the filesystem.
//
// Usage:
//
//	llmd export <path> [dir]             Export documents to directory (default: .)
//	llmd export --overwrite <path> [dir] Overwrite existing files

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var exportSpec = sdk.Command{
	Name: "export", Desc: `Export documents from the store as files

Writes matching documents to the filesystem as .md files. If <path>
matches a single document, that document is exported. If it ends
with /, all documents under that path are exported.

The target directory defaults to the current directory if omitted.`, Usage: "export [options] <path> [dir]", Flags: []sdk.Flag{
		{Name: "overwrite", Type: "bool", Desc: "Overwrite existing files"},
	},
}

func exportCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(exportSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}

	if len(positional) < 1 {
		return nil, fmt.Errorf("export: %w: document path\n\nUsage: llmd export <path> [dir]\n\nExamples:\n  llmd export notes/todo\n  llmd export notes/ ./backup", sdk.ErrMissingArg)
	}

	path := positional[0]
	dir := "."
	if len(positional) >= 2 {
		dir = positional[1]
	}
	opts := sdk.ExportOpts{Overwrite: flags.Bool("overwrite")}
	result, err := ctx.Documents.Export(path, dir, opts)
	if err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}

	var lines []string
	for _, p := range result.Exported {
		lines = append(lines, fmt.Sprintf("exported %s", p))
	}
	if len(result.Skipped) > 0 {
		lines = append(lines, fmt.Sprintf("skipped %d existing", len(result.Skipped)))
	}

	return sdk.Result{Text: strings.Join(lines, "\n"), Data: result}, nil
}
