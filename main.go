// llmd is a document store for LLMs and humans.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jpl-au/llmd/internal/cli"
	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/llmd"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx := context.Background()

	store, err := llmd.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening store: %v\n", err)
		return cli.ExitError
	}
	defer store.Close()

	h, err := host.New(ctx, store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: creating host: %v\n", err)
		return cli.ExitError
	}
	defer h.Close(ctx)

	if err := h.LoadPlugins(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: loading plugins: %v\n", err)
		return cli.ExitError
	}

	c := cli.New(h)
	return c.Run(ctx, os.Args[1:])
}
