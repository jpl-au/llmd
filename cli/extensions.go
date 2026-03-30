package cli

import (
	"sort"
	"strings"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/sdk"
)

var extensionsSpec = sdk.Command{
	Name: "extensions", Desc: "List loaded extensions", Usage: "extensions",
}

// extensionsCmd lists all loaded compiled extensions. This is a storeless
// command - it runs without an open store since it only queries the registry.
func extensionsCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	names := extension.Names()
	sort.Strings(names)
	return sdk.Text(strings.Join(names, "\n")), nil
}
