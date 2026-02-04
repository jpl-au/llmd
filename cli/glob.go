package cli

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

func glob(ctx sdk.Context, args []string) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("glob: missing pattern argument")
	}

	paths, err := sdk.API.Glob(args[0])
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	if len(paths) == 0 {
		return sdk.Rich{Text: "", Data: []string{}}, nil
	}

	return sdk.Rich{Text: strings.Join(paths, "\n"), Data: paths}, nil
}
