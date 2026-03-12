package cli

// write creates or updates a document. Content comes from ctx.Stdin
// (piped input on CLI, Content field via MCP). If the document already
// exists, a new version is created; if not, it's created at version 1.

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var writeSpec = sdk.Command{
	Name: "write", Desc: `Create or update a document from standard input

Reads content from stdin and writes it as a new document version.
If the document does not exist, it is created at version 1.`, Usage: "write <path>", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
		{Name: "message", Type: "string", Desc: "Version message"},
	},
}

func write(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("write: %w", sdk.ErrMissingArg)
	}

	path := args[0]
	var message string
	for i := 1; i < len(args); i++ {
		if args[i] == "--message" && i+1 < len(args) {
			i++
			message = args[i]
		} else if after, ok := strings.CutPrefix(args[i], "--message="); ok {
			message = after
		}
	}

	if err := ctx.Documents.Write(path, ctx.Stdin, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Wrote %s", path)), nil
}
