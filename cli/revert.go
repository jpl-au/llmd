package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func revert(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("revert: requires <path> <version> arguments")
	}

	path := args[0]
	versionStr := strings.TrimPrefix(args[1], "v")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("revert: invalid version: %s", args[1])
	}

	var message string
	for i := 2; i < len(args); i++ {
		if args[i] == "--message" && i+1 < len(args) {
			i++
			message = args[i]
		} else if strings.HasPrefix(args[i], "--message=") {
			message = strings.TrimPrefix(args[i], "--message=")
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
