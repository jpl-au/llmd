// agent_run.go implements the agent run subcommand. This is an internal
// command invoked by Spawn to manage the full agent lifecycle in Go,
// replacing the previous bash wrapper script.

package cli

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jpl-au/llmd/internal/llmd/agents"
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
		return nil, fmt.Errorf("agent run: %w: task key", sdk.ErrMissingArg)
	}

	taskKey := positional[0]
	worktree := flags.String("worktree")
	if worktree == "" {
		return nil, fmt.Errorf("agent run: %w: --worktree required", sdk.ErrMissingArg)
	}

	// Read the resolved run config from the worktree.
	cfg, err := agents.ReadRunConfig(worktree)
	if err != nil {
		return nil, fmt.Errorf("agent run: %w", err)
	}

	// Start the agent subprocess.
	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Dir = worktree
	cmd.Env = os.Environ()

	logPath := filepath.Join(worktree, ".llmd-agent.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("agent run: creating log file: %w", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("agent run: starting agent: %w", err)
	}

	// Forward termination signals to the child so llmd agent stop
	// cleanly shuts down the agent rather than orphaning it.
	defer forward(cmd)()

	// Wait for the agent to finish.
	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			slog.Error("waiting for agent process", "error", err)
			exitCode = 1
		}
	}

	// Record run completion before moving the task so pipeline
	// handlers see this run as finished when the move triggers
	// the next step.
	if err := ctx.Agents.Complete(taskKey, exitCode); err != nil {
		slog.Error("recording agent completion", "task", taskKey, "error", err)
	}

	// On failure, capture the agent's last output as an audit note
	// so the human reviewing the blocked task can see what went wrong.
	if exitCode != 0 {
		captureFailure(ctx, taskKey, cfg.Agent, logPath)
	}

	// Move the task based on exit code.
	if exitCode == 0 {
		moveTask(ctx, taskKey, cfg.Agent, cfg.OnSuccess)
	} else {
		moveTask(ctx, taskKey, cfg.Agent, cfg.OnFailure)
	}

	os.Exit(exitCode)
	return nil, nil // unreachable
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
