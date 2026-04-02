package host

import (
	"context"
	"log/slog"

	"github.com/jpl-au/llmd/internal/llmd/rules"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/sdk"
)

// pipelineHandler subscribes to TaskMoved events and auto-spawns
// agents when a task enters a column with an agent rule configured.
// Spawn errors are logged, not returned - a failed auto-spawn must
// not roll back the task move.
type pipelineHandler struct {
	dir   string // .llmd/ directory for rule file access
	agent *agentAPI
}

func (h *pipelineHandler) HandleEvent(ctx context.Context, e pkgevents.Event) error {
	if e.Type != pkgevents.TaskMoved {
		return nil
	}

	to, _ := e.Metadata["to"].(string)
	if to == "" {
		return nil
	}

	// Read rules from disk.
	rs, err := rules.Load(h.dir, "default")
	if err != nil {
		slog.Debug("pipeline: reading rules", "error", err)
		return nil
	}

	cr := rs.Column(to)
	if cr.Agent == "" {
		return nil // Manual column.
	}

	// Skip if the task already has a running agent.
	r, err := h.agent.store.Agents.RunByTask(ctx, e.Key)
	if err == nil && r.Status == "running" {
		return nil
	}

	slog.Info("pipeline: auto-spawning agent", "task", e.Key, "column", to, "agent", cr.Agent, "role", cr.Role)

	_, err = h.agent.Spawn(e.Key, cr.Agent, e.Author, sdk.SpawnOpts{
		Role:      cr.Role,
		Resume:    cr.Resume,
		OnSuccess: cr.Success,
		OnFailure: cr.Failure,
	})
	if err != nil {
		slog.Warn("pipeline: auto-spawn failed", "task", e.Key, "column", to, "error", err)
	}

	return nil
}
