package host

import (
	"context"
	"log/slog"

	"github.com/jpl-au/llmd/internal/llmd"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/sdk"
)

// pipelineHandler subscribes to TaskMoved events and auto-spawns
// agents when a task enters a column with a configured pipeline
// step. Spawn errors are logged, not returned - a failed auto-spawn
// must not roll back the task move.
type pipelineHandler struct {
	store *llmd.Store
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

	// Check pipeline config for the target column.
	step, err := h.store.Tasks.Step(ctx, to)
	if err != nil {
		slog.Debug("pipeline: reading step config", "column", to, "error", err)
		return nil
	}
	if step == nil {
		return nil
	}

	// Skip if the task already has a running agent.
	r, err := h.store.Agents.RunByTask(ctx, e.Key)
	if err == nil && r.Status == "running" {
		return nil
	}

	slog.Info("pipeline: auto-spawning agent", "task", e.Key, "column", to, "agent", step.Agent, "role", step.Role)

	_, err = h.agent.Spawn(e.Key, step.Agent, e.Author, sdk.SpawnOpts{
		Role:      step.Role,
		OnSuccess: step.OnSuccess,
		OnFailure: step.OnFailure,
	})
	if err != nil {
		slog.Warn("pipeline: auto-spawn failed", "task", e.Key, "column", to, "error", err)
	}

	return nil
}
