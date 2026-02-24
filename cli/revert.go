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

func revert(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("revert: %w", sdk.ErrMissingArg)
	}

	path := args[0]
	// Strip optional "v" prefix so both "v3" and "3" work.
	versionStr := strings.TrimPrefix(args[1], "v")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("revert: %w: %s", sdk.ErrInvalidArg, args[1])
	}

	var message string
	for i := 2; i < len(args); i++ {
		if args[i] == "--message" && i+1 < len(args) {
			i++
			message = args[i]
		} else if after, ok := strings.CutPrefix(args[i], "--message="); ok {
			message = after
		}
	}

	if message == "" {
		message = fmt.Sprintf("Reverted to version %d", version)
	}

	if err := sdk.API.Revert(path, version, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("revert: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Reverted %s to version %d", path, version)), nil
}
