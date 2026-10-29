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

	s := server.New(ctx.Runtime,
		server.WithAddr(cfg.Server.Addr),
		server.WithAuthor(cfg.Author),
		server.WithToken(cfg.Server.Token),
		server.WithWebhooks(cfg.Webhook),
		server.WithWebhookQueue(cfg.Server.WebhookWorkers, cfg.Server.WebhookQueue),
		// Room for the request around the largest document allowed.
		server.WithMaxBody(int64(cfg.CountSetting("limits.content_size", 1))+64<<10),
		server.WithTimeouts(
			cfg.DurationSetting("server.read_header_timeout"),
			cfg.DurationSetting("server.read_timeout"),
			cfg.DurationSetting("server.idle_timeout"),
		),
		server.WithShutdownTimeout(cfg.DurationSetting("server.shutdown_timeout")),
		server.WithWatchInterval(cfg.DurationSetting("server.watch_interval")),
	)
	return nil, s.ListenAndServe(ctx)
}
