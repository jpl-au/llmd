package cli

import "github.com/jpl-au/llmd/sdk"

// version returns the build version string. In release builds this is
// injected via -ldflags; during development it returns "llmd dev".
func version(ctx sdk.Context, args []string) (sdk.Response, error) {
	return sdk.Text("llmd dev"), nil
}
