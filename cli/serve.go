// serve.go is an alias for the mcp command. The README documents "llmd serve"
// as the MCP server entry point.

package cli

import "github.com/jpl-au/llmd/sdk"

func serve(ctx sdk.Context, args []string) (sdk.Response, error) {
	return mcpCmd(ctx, args)
}
