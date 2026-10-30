// agent_run.go implements the agent run subcommand. This is an internal
// command invoked by Spawn to manage the full agent lifecycle in Go,
// replacing the previous bash wrapper script.

package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/jpl-au/llmd/ai/harness"
	"github.com/jpl-au/llmd/engine/agents"
	"github.com/jpl-au/llmd/sdk"
)

// maxLogTail is the maximum number of bytes captured from the end of
// the agent log when recording a failure audit.
const maxLogTail = 1000

var agentRunFlags = []sdk.Flag{
	{Name: "worktree", Type: "string", Desc: "Path to the agent worktree"},
}

func agentRun(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(agentRunFlags, args)
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}
	if len(positional) == 0 {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "agent run: a task key is required.")
	}

	taskKey := positional[0]
	worktree := flags.String("worktree")
	if worktree == "" {
		return nil, sdk.Errorf(sdk.ErrMissingArg, "agent run: --worktree is required.")
	}

	// Read the resolved run config from the worktree.
	cfg, err := agents.ReadRunConfig(worktree)
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}

	// Run the agent under its harness. A harness that cannot run the
	// agent fails the run like an agent that exits with 1, so the run
	// is still recorded and the task still moves.
	result := &sdk.RunResult{ExitCode: 1}
	if h, ok := harness.Lookup(cfg.Harness); !ok {
		slog.Error("this build has no such harness", "program", sdk.Program(), "harness", cfg.Harness)
	} else if r, err := h.Run(ctx.Context, cfg.Spec); err != nil {
		slog.Error("running agent", "harness", cfg.Harness, "error", err)
	} else {
		result = r
	}
	exitCode := result.ExitCode
	agent := cfg.Spec.Agent

	// Record run completion before moving the task so pipeline
	// handlers see this run as finished when the move triggers
	// the next step.
	if err := ctx.Agents.Complete(taskKey, *result); err != nil {
		slog.Error("recording agent completion", "task", taskKey, "error", err)
	}

	// On failure, capture the agent's last output as an audit note
	// so the human reviewing the blocked task can see what went wrong.
	if exitCode != 0 {
		captureFailure(ctx, taskKey, agent, cfg.Spec.Log)
	}

	// Move the task based on exit code.
	if exitCode == 0 {
		moveTask(ctx, taskKey, agent, cfg.OnSuccess)
	} else {
		moveTask(ctx, taskKey, agent, cfg.OnFailure)
	}

	// The runner's exit code is the agent's, for whoever started it.
	// Returning it, rather than calling os.Exit here, lets the store
	// close and checkpoint on the way out, and keeps an MCP or HTTP
	// caller's server running.
	if exitCode != 0 {
		return nil, &sdk.ExitError{Cmd: "agent run", Code: exitCode}
	}
	return sdk.Text(""), nil
}

// captureFailure reads the tail of the agent log and writes it as an
// audit note on the task.
func captureFailure(ctx sdk.Context, taskKey, agent, logPath string) {
	data, err := tailFile(logPath, maxLogTail)
	if err != nil {
		slog.Debug("reading agent log for failure audit", "error", err)
		return
	}
	if len(data) == 0 {
		return
	}

	if _, err := ctx.Audits.Add(sdk.AuditOpts{
		Target:  taskKey,
		Content: string(data),
		Author:  agent,
	}); err != nil {
		slog.Debug("adding failure audit", "task", taskKey, "error", err)
	}
}

// moveTask moves a task to the given column. Empty column means skip.
func moveTask(ctx sdk.Context, taskKey, agent, column string) {
	if column == "" {
		return
	}
	if err := ctx.Tasks.Move(taskKey, column, agent); err != nil {
		slog.Debug("moving task after agent run", "task", taskKey, "column", column, "error", err)
	}
}

// tailFile reads the last n bytes of a file.
func tailFile(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size()
	if size == 0 {
		return nil, nil
	}

	offset := int64(0)
	if size > int64(n) {
		offset = size - int64(n)
	}

	buf := make([]byte, size-offset)
	if _, err := f.ReadAt(buf, offset); err != nil {
		return nil, err
	}
	return buf, nil
}
