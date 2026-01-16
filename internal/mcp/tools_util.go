// tools_util.go provides helper functions for MCP tool parameter extraction.
//
// Separated to centralise the boilerplate of extracting typed parameters from
// MCP's generic argument map. These helpers provide safe defaults when
// optional parameters are missing.
//
// Design: We use permissive extraction (return default on error) rather than
// strict validation because MCP tools should be forgiving - an LLM omitting
// an optional parameter shouldn't cause cryptic errors.

package mcp

import (
	"github.com/jpl-au/llmd/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

// Parameter extraction helpers provide safe access to optional request arguments.

// getString returns a string parameter or the default if not present.
func getString(req mcp.CallToolRequest, name, def string) string {
	if v, err := req.RequireString(name); err == nil {
		return v
	}
	return def
}

// getBool returns a boolean parameter or the default if not present.
func getBool(req mcp.CallToolRequest, name string, def bool) bool { //nolint:unparam
	args, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		return def
	}
	if v, ok := args[name].(bool); ok {
		return v
	}
	return def
}

// getInt returns an integer parameter or the default. Handles JSON number type.
func getInt(req mcp.CallToolRequest, name string, def int) int { //nolint:unparam
	args, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		return def
	}
	if v, ok := args[name].(float64); ok {
		return int(v)
	}
	return def
}

// jsonResult wraps a value as an MCP text result with pretty-printed JSON.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := store.MarshalJSON(v)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
