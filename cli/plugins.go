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

// pluginsCmd lists all loaded plugins: compiled extensions first,
// then yaegi dynamic plugins. This is a storeless command - it runs
// without an open store since it only queries the registry.
func pluginsCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	var lines []string

	names := extension.Names()
	sort.Strings(names)
	lines = append(lines, names...)

	if sdk.PluginNames != nil {
		pnames := sdk.PluginNames()
		sort.Strings(pnames)
		lines = append(lines, pnames...)
	}

	return sdk.Text(strings.Join(lines, "\n")), nil
}
