package cli

import (
	"sort"
	"strings"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/sdk"
)

var pluginsSpec = sdk.Command{
	Name: "plugins", Desc: "List loaded plugins", Usage: "plugins",
}

// pluginsCmd lists all loaded compiled extensions. This is a storeless
// command - it runs without an open store since it only queries the registry.
func pluginsCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	names := extension.Names()
	sort.Strings(names)
	return sdk.Text(strings.Join(names, "\n")), nil
}
