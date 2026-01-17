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
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	cfg, err := config.Load()
	if err != nil {
		log.Event("mcp:config_get", "get").Author("mcp").Write(err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	key := getString(req, "key", "")
	if key == "" {
		log.Event("mcp:config_get", "list").Author("mcp").Write(nil)
		return jsonResult(cfg.All())
	}

	v, err := cfg.Get(key)

	log.Event("mcp:config_get", "get").Author("mcp").Detail("key", key).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]string{key: v})
}

// configSet handles llmd_config_set tool calls.
func (h *handlers) configSet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) { //nolint:revive // ctx for future use
	if err := h.requireInit(); err != nil {
		return err, nil
	}

	key, err := req.RequireString("key")
	if err != nil {
		return mcp.NewToolResultError("key is required"), nil //nolint:nilerr
	}

	value, err := req.RequireString("value")
	if err != nil {
		return mcp.NewToolResultError("value is required"), nil //nolint:nilerr
	}

	cfg, err := config.Load()
	if err != nil {
		log.Event("mcp:config_set", "set").Author("mcp").Detail("key", key).Detail("value", value).Write(err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := cfg.Set(key, value); err != nil {
		log.Event("mcp:config_set", "set").Author("mcp").Detail("key", key).Detail("value", value).Write(err)
		return mcp.NewToolResultError(err.Error()), nil
	}

	err = cfg.Save()

	log.Event("mcp:config_set", "set").Author("mcp").Detail("key", key).Detail("value", value).Write(err)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Reload config into the running service so new values take effect immediately
	if err := h.svc.ReloadConfig(); err != nil {
		log.Event("mcp:config_set", "reload").Author("mcp").Write(err)
		// Config was saved successfully, but reload failed - warn in response
		return mcp.NewToolResultText(fmt.Sprintf("%s = %s (warning: reload failed, restart server to apply: %v)", key, value, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("%s = %s", key, value)), nil
}
