package sdk

// HookStore handles the lifecycle events an agent platform sends
// through its hook system. There is one implementation, in the host,
// so the CLI (llmd hook), the HTTP server (POST /hook), MCP and
// extensions all do exactly the same thing for the same event.
type HookStore interface {
	// Handle parses payload in the format of the author's platform
	// (names containing "claude" or "gemini" select that platform;
	// anything else is the generic format) and acts on the event:
	//
	//   session.start   Content lists the author's pending messages
	//   session.end     a queue message says the session ended
	//   session.stop    the same as session.end
	//   task.completed  the task is finished and a message sent
	//   task.failed     the task is moved to blocked and a message sent
	//   tool.post       an audit entry is added when a task and
	//                   content are present
	//
	// Each action is recorded in the result when it succeeds; a failed
	// action is logged and skipped so one problem does not stop the
	// rest. An unknown event is accepted and does nothing.
	Handle(author string, payload []byte, opts HookOpts) (*HookResult, error)
}

// HookOpts adjusts how a hook payload is handled.
type HookOpts struct {
	// Task is the task key the event belongs to, used when the
	// payload does not name one. Spawned agents receive it through
	// their generated hook configuration.
	Task string
}

// HookResult reports what Handle did.
type HookResult struct {
	Event   string   `json:"event"`             // normalised event name
	Task    string   `json:"task,omitempty"`    // task the event applied to
	Actions []string `json:"actions,omitempty"` // actions taken, in order
	Content string   `json:"content,omitempty"` // text for the agent, if any
}
