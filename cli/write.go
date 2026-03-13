package cli

// write creates or updates a document. Content comes from ctx.Stdin
// (piped input on CLI, Content field via MCP). If the document already
// exists, a new version is created; if not, it's created at version 1.

import (
	"fmt"

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
	flags, positional, err := sdk.ParseArgs(writeSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	if len(positional) == 0 {
		return nil, fmt.Errorf("write: %w", sdk.ErrMissingArg)
	}

	path := positional[0]
	message := flags.String("message")

	if err := ctx.Documents.Write(path, ctx.Stdin, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Wrote %s", path)), nil
}
