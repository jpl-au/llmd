package agents

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/key"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
)

// Run is the materialised view of an agent execution. It does not
// map directly to a single table row; instead it is assembled by
// joining the immutable identity from agent_runs with the latest
// terminal event (if any) from agent_events. Status and PID are
// derived at query time rather than stored, because the underlying
// tables are insert-only and never mutated after creation.
type Run struct {
	Key          string
	TaskKey      string
	Agent        string
	Branch       string
	Worktree     string
	Status       string
	PID          int
	ExitCode     int
	MonetaryCost *float64
	InputTokens  *int
	OutputTokens *int
	Model        string
	SessionID    string
	Author       string
	StartedAt    int64
	StoppedAt    int64
}

// RecordOpts captures the spawn-time identity of an agent run. These
// values are written once to agent_runs and never change. PID is
// recorded here because it is only meaningful at spawn time; once the
// process exits, the PID is no longer valid.
type RecordOpts struct {
	TaskKey  string
	Agent    string
	Branch   string
	Worktree string
	PID      int
	Author   string
}

// Record inserts a new agent run and a "spawned" event, then emits
// an AgentSpawned event. Returns ErrRunning if the task already has
// a running agent.
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
		INSERT INTO agent_runs (key, task_key, agent, branch, worktree, pid, author, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, k, opts.TaskKey, opts.Agent, opts.Branch, opts.Worktree, opts.PID, opts.Author, now).Write()
	if err != nil {
		return nil, fmt.Errorf("inserting agent run: %w", err)
	}

	_, err = a.db.Query(`
		INSERT INTO agent_events (run_key, event, created_at)
		VALUES (?, 'spawned', ?)
	`, k, now).Write()
	if err != nil {
		return nil, fmt.Errorf("inserting spawned event: %w", err)
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

// CompleteOpts carries the stats extracted from the agent's output
// after it exits. These are written to a terminal event row in
// agent_events. SessionID is captured here so that future spawns
// can resume the agent's conversation context if the task opts in
// via the "resume" flag.
type CompleteOpts struct {
	ExitCode     int
	MonetaryCost *float64
	InputTokens  *int
	OutputTokens *int
	Model        string
	SessionID    string
}

// Complete records the outcome of an agent run by inserting a
// terminal event ("completed" or "failed") into agent_events. This
// is an insert, not an update, because the agent tables follow an
// append-only pattern. The run's status is derived from this event
// at query time by the LEFT JOIN in runQuery.
func (a *Agents) Complete(ctx context.Context, taskKey string, opts CompleteOpts) error {
	r, err := a.RunByTask(ctx, taskKey)
	if err != nil {
		return err
	}
	if r.Status != "running" {
		return fmt.Errorf("agent run %s is not running (status: %s)", r.Key, r.Status)
	}

	now := time.Now().UnixMilli()
	event := "completed"
	evtType := pkgevents.AgentCompleted
	if opts.ExitCode != 0 {
		event = "failed"
		evtType = pkgevents.AgentFailed
	}

	var costVal sql.NullFloat64
	if opts.MonetaryCost != nil {
		costVal = sql.NullFloat64{Float64: *opts.MonetaryCost, Valid: true}
	}
	var inTok, outTok sql.NullInt64
	if opts.InputTokens != nil {
		inTok = sql.NullInt64{Int64: int64(*opts.InputTokens), Valid: true}
	}
	if opts.OutputTokens != nil {
		outTok = sql.NullInt64{Int64: int64(*opts.OutputTokens), Valid: true}
	}
	var model sql.NullString
	if opts.Model != "" {
		model = sql.NullString{String: opts.Model, Valid: true}
	}
	var sessionID sql.NullString
	if opts.SessionID != "" {
		sessionID = sql.NullString{String: opts.SessionID, Valid: true}
	}

	_, err = a.db.Query(`
		INSERT INTO agent_events (run_key, event, exit_code, monetary_cost, input_tokens, output_tokens, model, session_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, r.Key, event, opts.ExitCode, costVal, inTok, outTok, model, sessionID, now).Write()
	if err != nil {
		return fmt.Errorf("inserting completion event: %w", err)
	}

	metadata := map[string]any{
		"task_key":  r.TaskKey,
		"agent":     r.Agent,
		"exit_code": opts.ExitCode,
	}
	if opts.MonetaryCost != nil {
		metadata["monetary_cost"] = *opts.MonetaryCost
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

// MarkStopped records that a running agent was manually terminated
// by inserting a "stopped" event. Unlike Complete, no stats are
// captured because the agent did not finish normally and its output
// may be incomplete or absent.
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
		INSERT INTO agent_events (run_key, event, created_at)
		VALUES (?, 'stopped', ?)
	`, r.Key, now).Write()
	if err != nil {
		return nil, fmt.Errorf("inserting stopped event: %w", err)
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

// runQuery is the base SELECT shared by RunByTask and List. It uses
// a LEFT JOIN so that running agents (which have no terminal event
// yet) still appear in results with NULL event columns. The join
// filters to only terminal events (completed, failed, stopped) and
// ignores the initial "spawned" event, because status is derived
// from which terminal event exists, not from the spawned row.
const runQuery = `
	SELECT
		r.key, r.task_key, r.agent, r.branch, r.worktree,
		r.pid, r.author, r.started_at,
		e.event, e.exit_code, e.monetary_cost, e.input_tokens,
		e.output_tokens, e.model, e.session_id, e.created_at
	FROM agent_runs r
	LEFT JOIN agent_events e ON e.run_key = r.key
		AND e.event IN ('completed', 'failed', 'stopped')
`

// RunByTask returns the most recent run for a task, ordered by row
// ID rather than timestamp. Row ID ordering is necessary because
// Windows has coarse millisecond clock resolution, and two runs
// created in quick succession can share the same started_at value.
func (a *Agents) RunByTask(ctx context.Context, taskKey string) (*Run, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	row, err := a.db.Query(runQuery+`
		WHERE r.task_key = ?
		ORDER BY r.id DESC
		LIMIT 1
	`, taskKey).WithContext(ctx).ReadRow()
	if err != nil {
		return nil, err
	}
	return scanRun(row)
}

// ListOpts filters agent run queries. Status is filtered in Go
// after materialisation rather than in SQL, because status is
// derived from the joined terminal event, not stored as a column.
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

	query := runQuery + ` WHERE 1=1`
	var args []any

	if opts.TaskKey != "" {
		query += ` AND r.task_key = ?`
		args = append(args, opts.TaskKey)
	}
	if opts.Agent != "" {
		query += ` AND r.agent = ?`
		args = append(args, opts.Agent)
	}

	// Status cannot be filtered in SQL because it is not a stored
	// column; it is derived from the joined terminal event in
	// applyNullable. Filter in Go after scanning instead.
	query += ` ORDER BY r.id DESC`

	rows, err := a.db.Query(query, args...).WithContext(ctx).Read()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		r, err := scanRunRow(rows)
		if err != nil {
			return nil, err
		}
		if opts.Status != "" && r.Status != opts.Status {
			continue
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// scanRun reads a single materialised Run from a sql.Row. This is
// the single-row variant used by RunByTask. The column order must
// match runQuery exactly: run identity columns first, then event
// columns from the LEFT JOIN.
func scanRun(row *sql.Row) (*Run, error) {
	var r Run
	var branch, worktree sql.NullString
	var event sql.NullString
	var exitCode sql.NullInt64
	var cost sql.NullFloat64
	var inTok, outTok, stoppedAt sql.NullInt64
	var model, sessionID sql.NullString

	err := row.Scan(
		&r.Key, &r.TaskKey, &r.Agent, &branch, &worktree,
		&r.PID, &r.Author, &r.StartedAt,
		&event, &exitCode, &cost, &inTok, &outTok, &model, &sessionID, &stoppedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}

	applyNullable(&r, branch, worktree, event, exitCode, cost, inTok, outTok, model, sessionID, stoppedAt)
	return &r, nil
}

// scanRunRow is the multi-row variant of scanRun, taking sql.Rows
// instead of sql.Row. Needed because sql.Row and sql.Rows have
// different Scan signatures despite identical column handling.
func scanRunRow(rows *sql.Rows) (*Run, error) {
	var r Run
	var branch, worktree sql.NullString
	var event sql.NullString
	var exitCode sql.NullInt64
	var cost sql.NullFloat64
	var inTok, outTok, stoppedAt sql.NullInt64
	var model, sessionID sql.NullString

	if err := rows.Scan(
		&r.Key, &r.TaskKey, &r.Agent, &branch, &worktree,
		&r.PID, &r.Author, &r.StartedAt,
		&event, &exitCode, &cost, &inTok, &outTok, &model, &sessionID, &stoppedAt,
	); err != nil {
		return nil, err
	}

	applyNullable(&r, branch, worktree, event, exitCode, cost, inTok, outTok, model, sessionID, stoppedAt)
	return &r, nil
}

// applyNullable translates SQL nullable columns into Go values and
// derives the run's status from the terminal event. When no terminal
// event exists (all event columns are NULL from the LEFT JOIN), the
// run is still active: status is set to "running" and ExitCode
// defaults to -1. When a terminal event is present, PID is zeroed
// because the process is no longer alive and the recorded PID should
// not be used for signal delivery.
func applyNullable(r *Run, branch, worktree, event sql.NullString, exitCode sql.NullInt64, cost sql.NullFloat64, inTok, outTok sql.NullInt64, model, sessionID sql.NullString, stoppedAt sql.NullInt64) {
	if branch.Valid {
		r.Branch = branch.String
	}
	if worktree.Valid {
		r.Worktree = worktree.String
	}

	// No terminal event in the LEFT JOIN means the agent process has
	// not reported an outcome yet, so the run is still active.
	r.ExitCode = -1
	r.Status = "running"
	if event.Valid {
		switch event.String {
		case "completed":
			r.Status = "completed"
		case "failed":
			r.Status = "failed"
		case "stopped":
			r.Status = "stopped"
		}
		r.PID = 0
	}

	if exitCode.Valid {
		r.ExitCode = int(exitCode.Int64)
	}
	if cost.Valid {
		r.MonetaryCost = &cost.Float64
	}
	if inTok.Valid {
		v := int(inTok.Int64)
		r.InputTokens = &v
	}
	if outTok.Valid {
		v := int(outTok.Int64)
		r.OutputTokens = &v
	}
	if model.Valid {
		r.Model = model.String
	}
	if sessionID.Valid {
		r.SessionID = sessionID.String
	}
	if stoppedAt.Valid {
		r.StoppedAt = stoppedAt.Int64
	}
}
