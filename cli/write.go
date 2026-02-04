package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func write(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("write: missing path argument")
	}

	path := args[0]
	var message string
	for i := 1; i < len(args); i++ {
		if args[i] == "--message" && i+1 < len(args) {
			i++
			message = args[i]
		} else if strings.HasPrefix(args[i], "--message=") {
			message = strings.TrimPrefix(args[i], "--message=")
		}
	}

	if err := sdk.API.Write(path, ctx.Stdin, ctx.Author, message); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Wrote %s", path)), nil
}
