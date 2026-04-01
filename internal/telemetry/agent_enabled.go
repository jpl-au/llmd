//go:build telemetry

package telemetry

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const agentTelemetryFile = "agent-telemetry.md"

// EmitAgent appends a region-delimited markdown block containing the
// raw LLM response to .llmd/agent-telemetry.md. Each block includes
// the run key, task key, agent name, and timestamp so entries can be
// correlated back to agent_events rows. The file is opened and closed
// per call rather than held open for the process lifetime, because
// agent completions are infrequent and the file may be read by other
// processes between writes.
func EmitAgent(e AgentEntry) {
	if e.RawJSON == "" {
		return
	}

	path := filepath.Join(".llmd", agentTelemetryFile)
	if _, err := os.Stat(".llmd"); os.IsNotExist(err) {
		path = agentTelemetryFile
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Debug("agent telemetry disabled: cannot open log", "path", path, "err", err)
		return
	}
	defer f.Close()

	ts := time.Now().UTC().Format(time.RFC3339)

	block := fmt.Sprintf(
		"============= START %s =============\n"+
			"Run: %s\n"+
			"Task: %s\n"+
			"Agent: %s\n\n"+
			"```json\n"+
			"%s\n"+
			"```\n"+
			"============= END %s =============\n\n",
		ts, e.RunKey, e.TaskKey, e.Agent, e.RawJSON, ts,
	)

	if _, err := f.WriteString(block); err != nil {
		slog.Debug("agent telemetry write failed", "err", err)
	}
}
