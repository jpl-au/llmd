// tools_config.go implements MCP tools for configuration management.
//
// Separated because config operations have unique characteristics: they
// modify persistent settings that affect all subsequent operations, and
// they must reload the service's cached config after changes.
//
// Design: Config changes trigger ReloadConfig() to ensure the running MCP
// server immediately uses new settings. Without this, config changes would
// only take effect after server restart.

package mcp

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/log"
	"github.com/mark3labs/mcp-go/mcp"
)

// configGet handles llmd_config_get tool calls.
func (h *handlers) configGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) { //nolint:revive // ctx for future use
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	author := getString(req, "author", "mcp")
	key := getString(req, "key", "")

	// Determine action based on whether key is provided
	action := "get"
	if key == "" {
		action = "list"
	}

	l := log.Event("mcp:config_get", action).Author(author)
	if key != "" {
		l.Detail("key", key)
	}
	defer func() { l.Write(err) }()

	cfg, err := config.Load()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if key == "" {
		return jsonResult(cfg.All())
	}

	v, err := cfg.Get(key)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]string{key: v})
}

// configSet handles llmd_config_set tool calls.
func (h *handlers) configSet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) { //nolint:revive // ctx for future use
	if result := h.requireInit(); result != nil {
		return result, nil
	}

	var err error
	author, err := req.RequireString("author")
	if err != nil {
		return mcp.NewToolResultError("author is required"), nil
	}

	key, err := req.RequireString("key")
	if err != nil {
		return mcp.NewToolResultError("key is required"), nil
	}

	value, err := req.RequireString("value")
	if err != nil {
		return mcp.NewToolResultError("value is required"), nil
	}

	// Note: value intentionally not logged to avoid leaking sensitive config (API keys, tokens)
	l := log.Event("mcp:config_set", "set").Author(author).Detail("key", key)
	defer func() { l.Write(err) }()

	cfg, err := config.Load()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = cfg.Set(key, value)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = cfg.Save()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Reload config into the running service so new values take effect immediately
	if reloadErr := h.svc.ReloadConfig(); reloadErr != nil {
		// Config was saved successfully, but reload failed - warn in response
		// Log the reload failure separately (err stays nil for the main operation)
		log.Event("mcp:config_set", "reload").Author(author).Detail("key", key).Write(reloadErr)
		return mcp.NewToolResultText(fmt.Sprintf("%s = %s (warning: reload failed, restart server to apply: %v)", key, value, reloadErr)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("%s = %s", key, value)), nil
}
