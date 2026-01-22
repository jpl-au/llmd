// llmd is a document store for LLMs and humans.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v") {
		version.Print()
		return nil
	}

	store, err := llmd.Open("")
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer store.Close()

	h, err := host.New(ctx, store)
	if err != nil {
		return fmt.Errorf("creating host: %w", err)
	}
	defer h.Close(ctx)

	if err := h.LoadPlugins(ctx); err != nil {
		return fmt.Errorf("loading plugins: %w", err)
	}

	// TODO: CLI routing
	return nil
}
