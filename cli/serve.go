// serve.go starts the HTTP API server.

package cli

import (
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/server"
	"github.com/jpl-au/llmd/sdk"
)

var serveSpec = sdk.Command{
	Name: "serve",
	Desc: `Start HTTP API server

Listens on localhost:5563 by default. To change the address, set
server.addr in your config:

  llmd config server.addr "localhost:9090"

Every registered command becomes an HTTP route - reads are GET,
mutations are POST. The /events endpoint streams real-time store
events via Server-Sent Events (SSE).

Logs all requests to stderr. Shuts down gracefully on SIGINT or
SIGTERM.

See 'llmd guide serve' for the full route reference.`,
	Usage:   "serve",
	Streams: true,
}

func serve(ctx sdk.Context, args []string) (sdk.Response, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	s := server.New(cfg, sdk.SubscribeEvents)
	return nil, s.ListenAndServe(ctx)
}
