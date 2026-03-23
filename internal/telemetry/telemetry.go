// Package telemetry provides diagnostic logging for command execution.
//
// Every command dispatched through CLI, MCP, or HTTP can be recorded
// to a JSONL file for post-hoc analysis. Telemetry is controlled at
// build time: builds with -tags telemetry write to telemetry.jsonl;
// default builds compile Emit to a no-op.
//
// Call [Init] once at startup and [Close] before exit. Call [Emit]
// from each dispatch site (host.Run, MCP handler, HTTP handler) to
// record an entry. The timestamp is set automatically by Emit.
package telemetry

// ErrStr returns the error string or empty if err is nil.
func ErrStr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// Entry is a single telemetry record. Command executions leave Event
// empty. Lifecycle events use Event to distinguish diagnostics from
// tool calls (e.g. "start", "stop", "connect", "start.nostore").
type Entry struct {
	Timestamp string   `json:"ts"`
	Source    string   `json:"source"`          // cli, mcp, http
	Event     string   `json:"event,omitempty"` // lifecycle event type
	Command   string   `json:"cmd,omitempty"`
	Args      []string `json:"args,omitempty"`
	Author    string   `json:"author,omitempty"`
	Success   bool     `json:"success"`
	Error     string   `json:"error,omitempty"`
}
