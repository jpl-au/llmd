// serve.go starts the HTTP API server.

package cli

import (
	"github.com/jpl-au/llmd/internal/server"
	"github.com/jpl-au/llmd/sdk"
)

var serveSpec = sdk.Command{
	Name: "serve",
	Desc: `Start HTTP API server

Listens on localhost:5563 by default. To change the address, set
serve_addr in your config:

  llmd config serve_addr "localhost:9090"

Every registered command becomes an HTTP route — reads are GET,
mutations are POST. Logs all requests to stderr. Shuts down
gracefully on SIGINT or SIGTERM.

See 'llmd guide serve' for the full route reference.`,
	Usage: "serve",
}

func serve(ctx sdk.Context, args []string) (sdk.Response, error) {
	cfg, err := sdk.Config.Read()
	if err != nil {
		return nil, err
	}

	addr := cfg["serve_addr"]
	if addr == "" {
		addr = "localhost:5563"
	}

	s := server.New(addr, ctx.Author)
	return nil, s.ListenAndServe(ctx)
}
