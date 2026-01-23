//go:build wasip1

package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// Revert defines the revert command for reverting to a previous version.
var Revert = sdk.Command{
	Name:        "revert",
	Description: "Revert a document to a previous version",
	Usage:       "revert <path> <version>",
	MCPEnabled:  true,
	Flags: []sdk.Flag{
		{Name: "message", Short: "m", Type: "string", Description: "Revert message"},
	},
}

// ExecRevert executes the revert command.
func ExecRevert(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("revert: requires <path> <version> arguments")
	}

	path := args[0]
	versionStr := strings.TrimPrefix(args[1], "v")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("revert: invalid version: %s", args[1])
	}

	message, _ := flags["message"].(string)
	if message == "" {
		message = fmt.Sprintf("Reverted to version %d", version)
	}

	if err := sdk.Host.Revert(path, version, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("revert: %w", err)
	}

	return sdk.TextResult(fmt.Sprintf("Reverted %s to version %d", path, version)), nil
}
