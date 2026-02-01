// llmd is a document store for LLMs and humans.
package main

import (
	"context"
	"os"

	"github.com/jpl-au/llmd/internal/cli"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx := context.Background()
	return cli.New().Run(ctx, os.Args[1:])
}
