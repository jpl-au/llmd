// log_storage.go implements SQLite-based persistent audit logging.
//
// Separated from log.go to isolate database concerns. The main log.go provides
// the fluent API for building log entries, while this file handles persistence.
// Using SQLite enables cross-project log queries and structured filtering that
// plain text logs cannot provide. The project field uses a hash of the directory
// path to enable aggregation while preserving privacy.
//
// Design: Errors during logging are silently ignored (best-effort). This prevents
// log failures from breaking the main operation - a document write should succeed
// even if we can't record it in the audit log.

package log

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/blake2b"
	_ "modernc.org/sqlite"
)

// Logger writes audit log entries to a SQLite database.
type Logger struct {
	db      *sql.DB
	project string
}

func (l *Logger) log(e Entry) {
	var detail *string
	if len(e.Detail) > 0 {
		if b, err := json.Marshal(e.Detail); err == nil {
			s := string(b)
			detail = &s
		}
	}

	success := 0
	if e.Success {
		success = 1
	}

	_, err := l.db.Exec(`
		INSERT INTO log (start, end, project, source, author, action, path, version,
		                 resolved_path, result_version, success, error, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Start, e.End, l.project, e.Source, nilIfEmpty(e.Author), e.Action,
		nilIfEmpty(e.Path), nilIfZero(e.Version),
		nilIfEmpty(e.ResolvedPath), nilIfZero(e.ResultVersion),
		success, nilIfEmpty(e.Error), detail,
	)
	if err != nil {
		// Best-effort logging: don't break main operation, but report failure
		_, _ = fmt.Fprintf(os.Stderr, "llmd: audit log write failed: %v\n", err)
	}
}

// dbPathFunc is the function that returns the database path.
// Tests can override this to use a temp directory.
var dbPathFunc = defaultDBPath

func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back to current directory if home cannot be determined.
		// This allows logging to work in unusual environments (containers, etc.)
		// rather than silently failing.
		return filepath.Join(".llmd", "log", "llmd-log.db")
	}
	return filepath.Join(home, ".llmd", "log", "llmd-log.db")
}

func dbPath() string {
	return dbPathFunc()
}

// DBPath returns the path to the log database.
func DBPath() string {
	return dbPath()
}

// hash creates a project identifier from the directory path, enabling
// cross-project log queries while preserving privacy.
func hash(s string) string {
	h, err := blake2b.New(8, nil) // 64-bit = 16 hex chars
	if err != nil {
		// Should never happen with nil key, but don't silently ignore
		panic("blake2b.New failed: " + err.Error())
	}
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// migrate creates the log table if it doesn't exist. Safe for concurrent access.
func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS log (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			start          INTEGER NOT NULL,
			end            INTEGER NOT NULL,
			project        TEXT NOT NULL,
			source         TEXT NOT NULL,
			author         TEXT,
			action         TEXT NOT NULL,
			path           TEXT,
			version        INTEGER,
			resolved_path  TEXT,
			result_version INTEGER,
			success        INTEGER NOT NULL,
			error          TEXT,
			detail         TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_log_start ON log(start);
		CREATE INDEX IF NOT EXISTS idx_log_project ON log(project);
		CREATE INDEX IF NOT EXISTS idx_log_source ON log(source);
	`)
	if err != nil {
		return err
	}

	// Add columns for existing databases (ignore errors if columns exist)
	db.Exec(`ALTER TABLE log ADD COLUMN start INTEGER`)
	db.Exec(`ALTER TABLE log ADD COLUMN end INTEGER`)

	return nil
}

// nilIfEmpty returns nil for empty strings, reducing NULL checks in queries.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nilIfZero returns nil for zero values, indicating "no version" in queries.
func nilIfZero(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}
