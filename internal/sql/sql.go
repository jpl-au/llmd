// Package sql provides embedded SQL schema files.
package sql

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed *.sql
var schemas embed.FS

// Exec executes all embedded schema files in alphabetical order.
func Exec(db *sql.DB) error {
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
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("exec %s: %w", entry.Name(), err)
		}
	}
	return nil
}
