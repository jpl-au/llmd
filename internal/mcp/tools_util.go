// tools_util.go provides helper functions for MCP tool parameter extraction.
//
// Separated to centralise the boilerplate of extracting typed parameters from
// MCP's generic argument map. These helpers provide safe defaults when
// optional parameters are missing.
//
// Design: We use permissive extraction (return default on error) rather than
// strict validation because MCP tools should be forgiving - an LLM omitting
// an optional parameter shouldn't cause cryptic errors. This is important
// because LLMs frequently omit optional parameters or provide them in
// unexpected formats; returning sensible defaults keeps the tool usable
// rather than failing with type errors that the LLM may struggle to interpret.

package mcp

import (
	"github.com/jpl-au/llmd/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

// getString extracts a string parameter from the MCP request, returning the
// provided default if the parameter is missing or cannot be parsed as a string.
//
// This uses RequireString internally but swallows the error, which aligns with
// our permissive extraction philosophy: optional parameters should never cause
// tool failures. The caller specifies what default makes sense for their use
// case (empty string, "mcp", etc).
func getString(req mcp.CallToolRequest, name, def string) string {
	if v, err := req.RequireString(name); err == nil {
		return v
	}
	return def
}

// getBool extracts a boolean parameter from the MCP request arguments.
//
// Unlike getString, we access the raw argument map directly because the mcp-go
// library's RequireBool doesn't exist. JSON booleans decode as Go bool values,
// so a simple type assertion suffices. Returns the default if the parameter is
// missing or not a boolean, which handles cases where an LLM might accidentally
// pass "true" (string) instead of true (boolean).
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

// getInt extracts an integer parameter from the MCP request arguments.
//
// JSON numbers are decoded as float64 in Go's encoding/json, so we must type
// assert to float64 first and then convert to int. This is a quirk of JSON
// that catches many developers: there's no integer type in JSON, only "number".
// Returns the default if the parameter is missing or not a number, ensuring
// that tool calls with invalid version numbers (for example) fail gracefully
// rather than with type errors.
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

// getStrings extracts a string array parameter from the MCP request arguments.
//
// JSON arrays decode as []any in Go, requiring iteration to safely extract each
// element as a string. Non-string elements are silently skipped rather than
// causing errors, which provides resilience against malformed LLM input. Returns
// nil (not empty slice) when the parameter is absent, allowing callers to
// distinguish between "not provided" and "provided but empty" if needed.
//
// This is used by tools like llmd_read that accept multiple paths, enabling
// batch operations that reduce round-trips between the LLM and the MCP server.
func getStrings(req mcp.CallToolRequest, name string) []string {
	args, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		return nil
	}
	arr, ok := args[name].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// jsonResult serialises any value as pretty-printed JSON and wraps it in an
// MCP text result for return to the LLM client.
//
// We use store.MarshalJSON (which pretty-prints with indentation) rather than
// compact JSON because LLMs parse structured output more reliably when it's
// formatted for readability. The slight increase in token count is worthwhile
// for the improved parsing accuracy and debuggability when inspecting logs.
//
// Errors during marshalling are converted to MCP error results rather than
// propagating as Go errors, keeping the tool response pattern consistent:
// all failures are communicated via MCP's error result mechanism, giving the
// LLM actionable feedback it can potentially retry or report to the user.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := store.MarshalJSON(v)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
