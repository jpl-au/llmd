// tools_init.go implements the MCP tool for initialising a new store.
//
// This tool works without an existing store, allowing LLMs to bootstrap
// a new llmd repository. Other tools require initialisation first.

package mcp

import (
	"context"
	"log/slog"

	"github.com/jpl-au/llmd/internal/document"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/mark3labs/mcp-go/mcp"
)

// initStore handles llmd_init tool calls.
func (h *handlers) initStore(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.svc != nil {
		return mcp.NewToolResultError("store already initialised"), nil
	}

	local := getBool(req, "local", false)

	err := document.Init(false, h.db, local, "")

	log.Event("mcp:init", "init").Author("mcp").Detail("local", local).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Open the newly created store
	svc, err := document.New(h.db)
	if err != nil {
		return mcp.NewToolResultError("init succeeded but failed to open store: " + err.Error()), nil
	}
	h.svc = svc

	slog.Info("store initialised", "local", local)

	if local {
		return mcp.NewToolResultText("store initialised (local - gitignored)"), nil
	}
	return mcp.NewToolResultText("store initialised"), nil
}
