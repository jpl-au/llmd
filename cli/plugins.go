package cli

import (
	"sort"
	"strings"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/sdk"
)

// pluginsCmd lists all loaded plugins: compiled extensions first,
// then yaegi dynamic plugins. This is a storeless command — it runs
// without an open store since it only queries the registry.
func pluginsCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	var lines []string

	names := extension.Names()
	sort.Strings(names)
	for _, n := range names {
		lines = append(lines, n)
	}

	if sdk.PluginNames != nil {
		pnames := sdk.PluginNames()
		sort.Strings(pnames)
		for _, n := range pnames {
			lines = append(lines, n)
		}
	}

	return sdk.Text(strings.Join(lines, "\n")), nil
}
