package cli

// revert rolls a document back to a previous version by creating a new
// version with that old content. The history is preserved — revert
// doesn't delete versions, it appends a new one.
//
// The version argument accepts an optional "v" prefix ("v3" or "3")
// so users can copy-paste from history output.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var revertSpec = sdk.Command{
	Name: "revert", Desc: `Roll back a document to a previous version

Creates a new version containing the content from an older version.
Non-destructive — existing versions are preserved in the history.
Use history to see available version numbers.`, Usage: "revert <path> <version>", MCP: true, NeedsAuthor: true, Flags: []sdk.Flag{
		{Name: "message", Type: "string", Desc: "Revert message"},
	},
}

func revert(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(revertSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("revert: %w", err)
	}
	if len(positional) < 2 {
		return nil, fmt.Errorf("revert: %w", sdk.ErrMissingArg)
	}

	path := positional[0]
	// Strip optional "v" prefix so both "v3" and "3" work.
	versionStr := strings.TrimPrefix(positional[1], "v")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("revert: %w: %s", sdk.ErrInvalidArg, positional[1])
	}
	message := flags.String("message")

	if message == "" {
		message = fmt.Sprintf("Reverted to version %d", version)
	}

	if err := ctx.Documents.Revert(path, version, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("revert: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Reverted %s to version %d", path, version)), nil
}
