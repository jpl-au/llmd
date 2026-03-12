// version.go implements the version command, which displays build
// information including version tag, build time, Go version, platform,
// and git commit hash. Returns both text (for terminal display) and
// structured data (for --json).

package cli

import (
	"github.com/jpl-au/llmd/app"
	"github.com/jpl-au/llmd/sdk"
)

var versionSpec = sdk.Command{
	Name: "version", Desc: "Show version information", Usage: "version",
}

// versionCmd returns build information as both human-readable text and
// structured data. The version info is set at build time via ldflags;
// during development it shows "dev" for the version tag.
func versionCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	info := app.Version()
	return sdk.Result{Text: info.String(), Data: info}, nil
}
