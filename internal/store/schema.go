// schema.go defines the SQLite database schema and provides schema execution helpers.
//
// Schema files are embedded from the sql/ directory and executed in alphabetical
// order (hence the numeric prefixes like 001_, 002_). This approach:
//
//   - Makes each table's schema self-contained and reviewable
//   - Enables extensions to follow the same pattern for custom tables
//   - Produces cleaner git diffs when schema changes
//   - Ensures deterministic execution order via numbered prefixes
//
// Extensions can create their own embedded schemas:
//
//	//go:embed sql/*.sql
//	var extensionSchemas embed.FS
//
//	func (e *Extension) Init(ctx extension.Context) error {
//	    if err := store.ExecEmbedded(ctx.DB(), extensionSchemas, "sql"); err != nil {
//	        return err
//	    }
//	    return nil
//	}

package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed sql/*.sql
var schemas embed.FS

var (
	// ErrNotFound indicates the requested document or version does not exist.
	// Callers should check for this to distinguish missing data from other errors.
	ErrNotFound = errors.New("document not found")
	// ErrAlreadyExists prevents overwriting existing documents during create/move.
	// Callers should check existence first or handle this error gracefully.
	ErrAlreadyExists = errors.New("document already exists")
	// ErrContentTooLarge is returned when document content exceeds the configured limit.
	ErrContentTooLarge = errors.New("document content too large")
)

// ExecEmbedded executes all .sql files from an embedded filesystem in alphabetical order.
// The dir parameter specifies the directory within the embed.FS to read from.
//
// This function is exported so extensions can use the same pattern for their own
// embedded schemas. Each .sql file should use IF NOT EXISTS clauses for idempotency.
func ExecEmbedded(db *sql.DB, fsys embed.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("read schema directory: %w", err)
	}

	// Sort entries to ensure deterministic order (should already be sorted, but be explicit)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := dir + "/" + entry.Name()
		data, err := fsys.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("exec %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// execSchema executes the embedded core schema files.
func execSchema(db *sql.DB) error {
	return ExecEmbedded(db, schemas, "sql")
}
