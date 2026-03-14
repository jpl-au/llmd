// serve.go starts the HTTP API server.

package cli

import (
	"github.com/jpl-au/llmd/internal/server"
	"github.com/jpl-au/llmd/sdk"
)

var serveSpec = sdk.Command{
	Name:  "serve",
	Desc:  "Start HTTP API server",
	Usage: "serve",
}

func serve(ctx sdk.Context, args []string) (sdk.Response, error) {
	cfg, err := sdk.Config.Read()
	if err != nil {
		return nil, err
	}

	addr := cfg["serve_addr"]
	if addr == "" {
		addr = "localhost:8080"
	}

	s := server.New(addr, ctx.Author)
	return nil, s.ListenAndServe()
}
