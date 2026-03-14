// Package sql provides embedded SQL schema files for llmd's database.
//
// Schema files are named alphabetically (content.sql, entities.sql,
// fts.sql, help.sql) and executed in that order. Alphabetical ordering
// matters because later schemas may reference tables created by earlier
// ones (e.g. fts.sql depends on tables from content.sql).
//
// All statements use CREATE IF NOT EXISTS and INSERT OR IGNORE, making
// the migration idempotent — it runs on every store open, not just init.
package sql

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jpl-au/qwr"
)

//go:embed *.sql
var schemas embed.FS

//go:embed help.md
var help string

// Exec executes all embedded .sql schema files in alphabetical order,
// then inserts the embedded help.md content into the help table.
//
// The help table stores documentation inside the database itself so
// that LLMs or humans exploring the SQLite file directly (e.g. via
// sqlite3 CLI) can discover what the schema represents and how to
// query it, without needing access to the llmd binary or source.
func Exec(db *qwr.Manager) error {
	entries, err := fs.ReadDir(schemas, ".")
	if err != nil {
		return fmt.Errorf("read schema directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := schemas.ReadFile(entry.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		if _, err := db.Query(string(data)).Write(); err != nil {
			return fmt.Errorf("exec %s: %w", entry.Name(), err)
		}
	}

	// Embed help documentation inside the database for discoverability.
	_, err = db.Query("INSERT OR IGNORE INTO help (content) VALUES (?)", help).Write()
	return err
}
