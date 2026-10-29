package documents

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jpl-au/llmd/errs"
)

// Check reports the state of the document at path: nil when it is
// live, ErrDeleted when it is deleted, ErrNotFound when the path has no
// versions. Every operation that needs a document to act on refuses a
// deleted one with ErrDeleted, so the error always says that restore
// brings it back.
func (d *Documents) Check(ctx context.Context, path string) error {
	var total, deleted int
	row, err := d.db.Query(stateQuery, namespace, path).WithContext(ctx).ReadRow()
	if err != nil {
		return fmt.Errorf("checking document: %w", err)
	}
	if err := row.Scan(&total, &deleted); err != nil {
		return fmt.Errorf("checking document: %w", err)
	}
	return state(path, total, deleted)
}

// CheckTx is Check on an existing transaction, for operations that
// must check and change a document atomically.
func CheckTx(ctx context.Context, tx *sql.Tx, path string) error {
	var total, deleted int
	if err := tx.QueryRowContext(ctx, stateQuery, namespace, path).Scan(&total, &deleted); err != nil {
		return fmt.Errorf("checking document: %w", err)
	}
	return state(path, total, deleted)
}

const stateQuery = `
	SELECT COUNT(*), COUNT(deleted_at) FROM content
	WHERE namespace = ? AND path = ?
`

// state maps version counts to Check's result. Every version of a path
// shares one state, so a path with any live version is live.
func state(path string, total, deleted int) error {
	switch {
	case total == 0:
		return NotFound(path)
	case deleted == total:
		return Deleted(path)
	default:
		return nil
	}
}

// NotFound is ErrNotFound for a named document, in the words a reader
// sees: "notes/plan does not exist."
func NotFound(name string) error {
	return errs.New(ErrNotFound, "%s does not exist.", name)
}

// Deleted is ErrDeleted for a named document, with the way back.
func Deleted(name string) error {
	return errs.New(ErrDeleted, "%s is deleted. Run llmd restore %s to bring it back.", name, name)
}

// NoVersion is the error for a version a document does not have, of
// the given kind: ErrNotFound when reading, history.ErrVersionInvalid
// when reverting. The wording is the same either way.
func NoVersion(kind error, name string, version int) error {
	return errs.New(kind, "%s has no version %d. Run llmd history %s to list them.", name, version, name)
}
