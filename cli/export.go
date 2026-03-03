// export.go exports documents from the store to the filesystem.
//
// Usage:
//
//	llmd export <prefix> <dir>             Export documents to directory
//	llmd export --overwrite <prefix> <dir> Overwrite existing files

package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func exportCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	var opts sdk.ExportOpts
	var positional []string

	for _, arg := range args {
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		default:
			positional = append(positional, arg)
		}
	}

	if len(positional) < 2 {
		return nil, fmt.Errorf("export: %w", sdk.ErrMissingArg)
	}

	prefix, dir := positional[0], positional[1]
	result, err := ctx.Documents.Export(prefix, dir, opts)
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
