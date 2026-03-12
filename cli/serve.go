// serve.go will start an HTTP API server. Not yet implemented.

package cli

import (
	"fmt"

	"github.com/jpl-au/llmd/sdk"
)

var serveSpec = sdk.Command{
	Name: "serve", Desc: "Start HTTP API server (coming soon)", Usage: "serve",
}

func serve(_ sdk.Context, _ []string) (sdk.Response, error) {
	return nil, fmt.Errorf("serve: HTTP API server is not yet implemented — use 'llmd mcp' for the MCP stdio server")
}
