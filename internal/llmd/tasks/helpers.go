// helpers.go contains small utility functions used across task operations.

package tasks

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// addFlag appends a flag to the comma-separated flags string. Returns
// the original string unchanged if the flag is already present.
func addFlag(flags, flag string) string {
	if flags == "" {
		return flag
	}
	for f := range strings.SplitSeq(flags, ",") {
		if f == flag {
			return flags // Already set
		}
	}
	return flags + "," + flag
}

// removeFlag removes a flag from the comma-separated flags string.
// Returns the remaining flags joined by commas, or empty string if
// no flags remain.
func removeFlag(flags, flag string) string {
	if flags == "" {
		return ""
	}
	var result []string
	for f := range strings.SplitSeq(flags, ",") {
		if f != flag {
			result = append(result, f)
		}
	}
	return strings.Join(result, ",")
}

// recordTx inserts an audit record within an existing transaction.
// Mirrors audit.Log.Record but uses the provided tx so the audit
// entry commits or rolls back with the surrounding operation.
func recordTx(ctx context.Context, tx *sql.Tx, actor, action, subject, oldValue, newValue string) {
	now := time.Now().UnixMilli()
	var oldV, newV sql.NullString
	if oldValue != "" {
		oldV = sql.NullString{String: oldValue, Valid: true}
	}
	if newValue != "" {
		newV = sql.NullString{String: newValue, Valid: true}
	}
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO history (timestamp, actor, action, subject, old_value, new_value)
		VALUES (?, ?, ?, ?, ?, ?)
	`, now, actor, action, subject, oldV, newV)
}

// nullStr converts a Go string to sql.NullString. Empty strings become
// NULL in the database; non-empty strings become valid values.
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
