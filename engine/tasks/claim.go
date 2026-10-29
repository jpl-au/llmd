// claim.go hands the next ready task to a worker in one write.

package tasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jpl-au/llmd/errs"

	"github.com/jpl-au/llmd/clock"
	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/pkg/model/task"
)

// ErrNoneReady is returned by Claim when no task in the source column
// can be taken.
var ErrNoneReady = errors.New("no task is ready.")

// Flags that keep a task out of Claim's reach. A human task is work
// for the user; hold and blocked mark work that must wait.
const (
	flagHuman   = "human"
	flagHold    = "hold"
	flagBlocked = "blocked"
)

// ClaimOptions configures a Claim. Empty fields take the defaults.
type ClaimOptions struct {
	From       string // column to take from; default "up-next"
	To         string // column to move to; default "in-progress"
	AssignedTo string // assignee; default the author
}

// Claim takes the first ready task of a column for a worker: in a
// single transaction it moves the task to the target column and
// assigns it, so two workers claiming at once never take the same
// task. Tasks are considered in the column's order (priority first by
// default, then position) and passed over when they are flagged human,
// hold or blocked,
// when a dependency is not done, or, for the backlog, when they have
// no spec. Returns ErrNoneReady when nothing can be taken.
func (t *Tasks) Claim(ctx context.Context, author string, opts ClaimOptions) (*task.Task, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}

	from, to := opts.From, opts.To
	if from == "" {
		from = "up-next"
	}
	if to == "" {
		to = "in-progress"
	}
	cols, err := t.Columns(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range []string{from, to} {
		if !slices.Contains(cols, c) {
			return nil, errs.New(ErrInvalidCol, "there is no column %s. Run llmd task column list to see them.", c)
		}
	}
	if from == to {
		return nil, errs.New(ErrInvalidCol, "--from and --to are both %s.", from)
	}
	assignee := opts.AssignedTo
	if assignee == "" {
		assignee = author
	}

	// Spec gating applies when leaving the backlog, as in Move. The
	// documents live in llmd.db, so this is checked before the work.db
	// transaction and only for the column it applies to.
	specced := func(path string) bool {
		if from != "backlog" {
			return true
		}
		ok, err := t.HasSpec(ctx, path)
		return err == nil && ok
	}

	result, err := t.db.TransactionFunc(func(tx *sql.Tx) (any, error) {
		candidates, err := claimCandidates(ctx, tx, from, t.orderBy())
		if err != nil {
			return nil, err
		}
		for _, c := range candidates {
			if hasAnyFlag(c.flags, flagHuman, flagHold, flagBlocked) || !specced(c.path) {
				continue
			}
			ready, err := readyTx(ctx, tx, c.dependsOn)
			if err != nil {
				return nil, err
			}
			if !ready {
				continue
			}

			var maxPos int
			if err := tx.QueryRowContext(ctx, `
				SELECT COALESCE(MAX(position), -1) FROM tasks
				WHERE status = ? AND deleted_at IS NULL
			`, to).Scan(&maxPos); err != nil {
				return nil, fmt.Errorf("getting position: %w", err)
			}

			// The status check makes the claim exclusive: a task another
			// worker took between the select and this update is skipped.
			res, err := tx.ExecContext(ctx, `
				UPDATE tasks SET status = ?, position = ?, assigned_to = ?
				WHERE key = ? AND status = ? AND deleted_at IS NULL
			`, to, maxPos+1, assignee, c.key, from)
			if err != nil {
				return nil, fmt.Errorf("claiming task: %w", err)
			}
			if n, err := res.RowsAffected(); err != nil || n != 1 {
				continue
			}

			recordTx(ctx, tx, author, "moved", c.key, from, to)
			if c.assignedTo != assignee {
				recordTx(ctx, tx, author, "edited:assigned_to", c.key, c.assignedTo, assignee)
			}
			return c.key, nil
		}
		return "", ErrNoneReady
	}).WithContext(ctx).Exec()
	if err != nil {
		return nil, err
	}

	tsk, err := t.Read(ctx, result.Value.(string))
	if err != nil {
		return nil, err
	}

	if t.bus != nil {
		t.bus.Emit(ctx, events.Event{
			Type:      events.TaskMoved,
			Path:      tsk.Path,
			Key:       tsk.Key,
			Author:    author,
			Timestamp: clock.Now(),
			Metadata:  map[string]any{"from": from, "to": to, "assigned_to": assignee, "claimed": true},
		})
	}

	return tsk, nil
}

// candidate is the little a Claim needs to know about a task before
// deciding to take it.
type candidate struct {
	key, flags, dependsOn, path, assignedTo string
}

// claimCandidates lists the tasks of a column in the given order. The
// rows are read fully before any other statement runs on the
// transaction.
func claimCandidates(ctx context.Context, tx *sql.Tx, status, orderBy string) ([]candidate, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT key, flags, depends_on, path, assigned_to FROM tasks
		WHERE status = ? AND deleted_at IS NULL
	`+orderBy, status)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", status, err)
	}
	defer rows.Close()

	var out []candidate
	for rows.Next() {
		var c candidate
		var flags, dependsOn, assignedTo sql.NullString
		if err := rows.Scan(&c.key, &flags, &dependsOn, &c.path, &assignedTo); err != nil {
			return nil, err
		}
		c.flags, c.dependsOn, c.assignedTo = flags.String, dependsOn.String, assignedTo.String
		out = append(out, c)
	}
	return out, rows.Err()
}

// readyTx reports whether the dependency chain starting at dep is
// done, reading on the transaction. An empty dep is ready; a missing
// or cyclic dependency is not.
func readyTx(ctx context.Context, tx *sql.Tx, dep string) (bool, error) {
	seen := map[string]bool{}
	for dep != "" {
		if seen[dep] {
			return false, nil
		}
		seen[dep] = true
		var status string
		var next sql.NullString
		err := tx.QueryRowContext(ctx, `
			SELECT status, depends_on FROM tasks WHERE key = ? AND deleted_at IS NULL
		`, dep).Scan(&status, &next)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("reading dependency %s: %w", dep, err)
		}
		if status != "done" {
			return false, nil
		}
		dep = next.String
	}
	return true, nil
}

// hasAnyFlag reports whether the comma-separated flags hold any of names.
func hasAnyFlag(flags string, names ...string) bool {
	if flags == "" {
		return false
	}
	for f := range strings.SplitSeq(flags, ",") {
		if slices.Contains(names, strings.TrimSpace(f)) {
			return true
		}
	}
	return false
}
