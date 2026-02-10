package cli

import (
	"github.com/jpl-au/llmd/internal/version"
	"github.com/jpl-au/llmd/sdk"
)

func versionCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	info := version.Get()
	return sdk.Result{Text: info.String(), Data: info}, nil
}
