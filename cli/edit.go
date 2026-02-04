package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func edit(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("edit: requires <path> <old> <new> arguments")
	}

	path, old, new := args[0], args[1], args[2]

	var message string
	for i := 3; i < len(args); i++ {
		if args[i] == "--message" && i+1 < len(args) {
			i++
			message = args[i]
		} else if strings.HasPrefix(args[i], "--message=") {
			message = strings.TrimPrefix(args[i], "--message=")
		}
	}

	if err := sdk.API.Edit(path, old, new, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("edit: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Edited %s", path)), nil
}
