package agents

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/key"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
)

// Run is the materialised view of an agent execution, joining the
// immutable run identity from agent_runs with the latest state from
// agent_events.
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

// RecordOpts holds values for creating a new agent run record.
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

// CompleteOpts holds values for completing an agent run.
type CompleteOpts struct {
	ExitCode     int
	MonetaryCost *float64
	InputTokens  *int
	OutputTokens *int
	Model        string
	SessionID    string
}

// Complete inserts a completion or failure event for the task's
// current run and emits the appropriate bus event.
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

// MarkStopped inserts a "stopped" event for the task's current run
// and emits an AgentStopped bus event.
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

// runQuery is the base SELECT that materialises a Run by joining
// agent_runs with the latest terminal event from agent_events.
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

// RunByTask returns the most recent run for a task.
func (a *Agents) RunByTask(ctx context.Context, taskKey string) (*Run, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	row, err := a.db.Query(runQuery+`
		WHERE r.task_key = ?
		ORDER BY r.started_at DESC
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

	// Status filtering is applied after materialisation since status
	// is derived from the presence/type of a terminal event.
	query += ` ORDER BY r.started_at DESC`

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

// scanRun reads a single materialised Run from a sql.Row.
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

// scanRunRow reads a single materialised Run from sql.Rows (used by List).
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

// applyNullable populates a Run's optional fields and derives status
// from the terminal event.
func applyNullable(r *Run, branch, worktree, event sql.NullString, exitCode sql.NullInt64, cost sql.NullFloat64, inTok, outTok sql.NullInt64, model, sessionID sql.NullString, stoppedAt sql.NullInt64) {
	if branch.Valid {
		r.Branch = branch.String
	}
	if worktree.Valid {
		r.Worktree = worktree.String
	}

	// Derive status from the terminal event. No terminal event means
	// the run is still active.
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
