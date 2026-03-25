package agents

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/key"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
)

// Run is the internal representation of an agent execution.
type Run struct {
	Key       string
	TaskKey   string
	Agent     string
	Branch    string
	Worktree  string
	Status    string
	PID       int
	ExitCode  int
	Cost      *float64
	Author    string
	StartedAt int64
	StoppedAt int64
}

// RecordOpts holds values for creating a new agent run record.
type RecordOpts struct {
	TaskKey  string
	Agent    string
	Branch   string
	Worktree string
	PID      int
	Author   string
}

// Record inserts a new agent run and emits an AgentSpawned event.
// Returns ErrRunning if the task already has a running agent.
func (a *Agents) Record(ctx context.Context, opts RecordOpts) (*Run, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}

	running, err := a.hasRunning(ctx, opts.TaskKey)
	if err != nil {
		return nil, fmt.Errorf("checking running agents: %w", err)
	}
	if running {
		return nil, fmt.Errorf("%w: %s", ErrRunning, opts.TaskKey)
	}

	k := key.Generate()
	now := time.Now().UnixMilli()

	_, err = a.db.Query(`
		INSERT INTO agent_activity (key, task_key, agent, branch, worktree, status, pid, exit_code, author, started_at)
		VALUES (?, ?, ?, ?, ?, 'running', ?, -1, ?, ?)
	`, k, opts.TaskKey, opts.Agent, opts.Branch, opts.Worktree, opts.PID, opts.Author, now).Write()
	if err != nil {
		return nil, fmt.Errorf("inserting agent run: %w", err)
	}

	r := &Run{
		Key:       k,
		TaskKey:   opts.TaskKey,
		Agent:     opts.Agent,
		Branch:    opts.Branch,
		Worktree:  opts.Worktree,
		Status:    "running",
		PID:       opts.PID,
		ExitCode:  -1,
		Author:    opts.Author,
		StartedAt: now,
	}

	if a.bus != nil {
		a.bus.Emit(ctx, pkgevents.Event{
			Type:      pkgevents.AgentSpawned,
			Key:       k,
			Author:    opts.Author,
			Timestamp: now,
			Metadata: map[string]any{
				"task_key": opts.TaskKey,
				"agent":    opts.Agent,
				"branch":   opts.Branch,
			},
		})
	}

	return r, nil
}

// CompleteOpts holds values for completing an agent run.
type CompleteOpts struct {
	ExitCode int
	Cost     *float64
}

// Complete marks a run as completed or failed based on exit code and
// emits the appropriate event. Cost is recorded when available.
func (a *Agents) Complete(ctx context.Context, taskKey string, opts CompleteOpts) error {
	r, err := a.RunByTask(ctx, taskKey)
	if err != nil {
		return err
	}
	if r.Status != "running" {
		return fmt.Errorf("agent run %s is not running (status: %s)", r.Key, r.Status)
	}

	now := time.Now().UnixMilli()
	status := "completed"
	evtType := pkgevents.AgentCompleted
	if opts.ExitCode != 0 {
		status = "failed"
		evtType = pkgevents.AgentFailed
	}

	var costVal sql.NullFloat64
	if opts.Cost != nil {
		costVal = sql.NullFloat64{Float64: *opts.Cost, Valid: true}
	}

	_, err = a.db.Query(`
		UPDATE agent_activity SET status = ?, exit_code = ?, cost = ?, pid = 0, stopped_at = ?
		WHERE key = ?
	`, status, opts.ExitCode, costVal, now, r.Key).Write()
	if err != nil {
		return fmt.Errorf("completing agent run: %w", err)
	}

	metadata := map[string]any{
		"task_key":  r.TaskKey,
		"agent":     r.Agent,
		"exit_code": opts.ExitCode,
	}
	if opts.Cost != nil {
		metadata["cost"] = *opts.Cost
	}

	if a.bus != nil {
		a.bus.Emit(ctx, pkgevents.Event{
			Type:      evtType,
			Key:       r.Key,
			Author:    r.Author,
			Timestamp: now,
			Metadata:  metadata,
		})
	}
	return nil
}

// MarkStopped marks a running agent as stopped and emits an event.
func (a *Agents) MarkStopped(ctx context.Context, taskKey, author string) (*Run, error) {
	r, err := a.RunByTask(ctx, taskKey)
	if err != nil {
		return nil, err
	}
	if r.Status != "running" {
		return nil, fmt.Errorf("agent run %s is not running (status: %s)", r.Key, r.Status)
	}

	now := time.Now().UnixMilli()
	_, err = a.db.Query(`
		UPDATE agent_activity SET status = 'stopped', pid = 0, stopped_at = ?
		WHERE key = ?
	`, now, r.Key).Write()
	if err != nil {
		return nil, fmt.Errorf("stopping agent run: %w", err)
	}

	r.Status = "stopped"
	r.PID = 0
	r.StoppedAt = now

	if a.bus != nil {
		a.bus.Emit(ctx, pkgevents.Event{
			Type:      pkgevents.AgentStopped,
			Key:       r.Key,
			Author:    author,
			Timestamp: now,
			Metadata: map[string]any{
				"task_key": r.TaskKey,
				"agent":    r.Agent,
			},
		})
	}
	return r, nil
}

// RunByTask returns the most recent run for a task.
func (a *Agents) RunByTask(ctx context.Context, taskKey string) (*Run, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	row, err := a.db.Query(`
		SELECT key, task_key, agent, branch, worktree, status, pid, exit_code, cost, author, started_at, stopped_at
		FROM agent_activity
		WHERE task_key = ?
		ORDER BY started_at DESC
		LIMIT 1
	`, taskKey).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, err
	}
	return scanRun(row)
}

// ListOpts filters agent run queries.
type ListOpts struct {
	Status  string
	TaskKey string
	Agent   string
}

// List returns agent runs matching the filter.
func (a *Agents) List(ctx context.Context, opts ListOpts) ([]*Run, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}

	query := `
		SELECT key, task_key, agent, branch, worktree, status, pid, exit_code, cost, author, started_at, stopped_at
		FROM agent_activity WHERE 1=1
	`
	var args []any

	if opts.Status != "" {
		query += ` AND status = ?`
		args = append(args, opts.Status)
	}
	if opts.TaskKey != "" {
		query += ` AND task_key = ?`
		args = append(args, opts.TaskKey)
	}
	if opts.Agent != "" {
		query += ` AND agent = ?`
		args = append(args, opts.Agent)
	}

	query += ` ORDER BY started_at DESC`

	rows, err := a.db.Query(query, args...).WithContext(ctx).Read()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		var r Run
		var branch, worktree sql.NullString
		var cost sql.NullFloat64
		var stoppedAt sql.NullInt64
		if err := rows.Scan(
			&r.Key, &r.TaskKey, &r.Agent, &branch, &worktree,
			&r.Status, &r.PID, &r.ExitCode, &cost, &r.Author,
			&r.StartedAt, &stoppedAt,
		); err != nil {
			return nil, err
		}
		if branch.Valid {
			r.Branch = branch.String
		}
		if worktree.Valid {
			r.Worktree = worktree.String
		}
		if cost.Valid {
			r.Cost = &cost.Float64
		}
		if stoppedAt.Valid {
			r.StoppedAt = stoppedAt.Int64
		}
		runs = append(runs, &r)
	}
	return runs, rows.Err()
}

// scanRun reads a single run from a database row.
func scanRun(row *sql.Row) (*Run, error) {
	var r Run
	var branch, worktree sql.NullString
	var cost sql.NullFloat64
	var stoppedAt sql.NullInt64

	err := row.Scan(
		&r.Key, &r.TaskKey, &r.Agent, &branch, &worktree,
		&r.Status, &r.PID, &r.ExitCode, &cost, &r.Author,
		&r.StartedAt, &stoppedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}

	if branch.Valid {
		r.Branch = branch.String
	}
	if worktree.Valid {
		r.Worktree = worktree.String
	}
	if cost.Valid {
		r.Cost = &cost.Float64
	}
	if stoppedAt.Valid {
		r.StoppedAt = stoppedAt.Int64
	}
	return &r, nil
}
