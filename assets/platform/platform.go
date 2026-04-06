// Package platform defines the interface for agent platform behaviour
// and provides a lookup function that returns the correct implementation
// for a given agent name. Platform-specific implementations live in
// sub-packages (claude, gemini, generic).
package platform

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Platform describes platform-specific behaviour for an agent tool.
// Common concerns like worktree management, prompt assembly, and run
// tracking stay in the caller.
type Platform interface {
	// SettingsPath returns the worktree-relative path where runtime
	// settings should be written. Empty means not supported.
	SettingsPath() string

	// BudgetArgs returns CLI flags for cost control. Nil means the
	// platform has no budget mechanism.
	BudgetArgs(budget float64) []string

	// Stats extracts execution metrics from the agent's output log.
	// Returns nil when no stats are available.
	Stats(logPath string) (*RunStats, error)

	// ResumeArgs returns CLI arguments that restore the agent's prior
	// conversation context and deliver a new prompt into that session.
	// Returns nil when the platform has no resume mechanism.
	ResumeArgs(sessionID, prompt string) []string

	// ParseHook extracts a normalised HookEvent from a platform-specific
	// hook payload. Each platform sends different JSON shapes; this
	// method translates the native format into a common representation.
	ParseHook(payload []byte) (*HookEvent, error)
}

// RunStats holds execution metrics extracted from an agent's output.
// All fields are nullable - agents report what they can.
type RunStats struct {
	MonetaryCost *float64
	InputTokens  *int
	OutputTokens *int
	Model        string
	SessionID    string
}

// HookEvent is the platform-agnostic representation of an agent hook
// event. The server routes on Event to determine which SDK operations
// to perform.
type HookEvent struct {
	// Event is the normalised event type: "session.start",
	// "session.end", "task.completed", "task.failed", "tool.post".
	Event string

	// TaskID is the target task key, when the hook is task-scoped.
	TaskID string

	// Content is the message body, summary, or audit text.
	Content string

	// Meta holds platform-specific extras (session_id, tool_name, etc.).
	Meta map[string]string
}

// LastJSON scans a file and returns the last line that starts with '{'.
// Agent tools write their JSON result as the final output line.
func LastJSON(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening agent log: %w", err)
	}
	defer f.Close()

	var last string
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "{") {
			last = line
		}
	}
	if err := s.Err(); err != nil {
		return "", fmt.Errorf("reading agent log: %w", err)
	}
	return last, nil
}

// StringField returns the first non-empty string value found under
// any of the given keys. Returns empty string if none match.
func StringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
