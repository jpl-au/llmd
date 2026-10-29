// Package sqlitetest opens throwaway on-disk databases for tests with
// the same settings as production. Tests never use in-memory SQLite: an
// in-memory database skips WAL, checkpointing and the filesystem, which
// is exactly what the store depends on.
package sqlitetest

import (
	"path/filepath"
	"testing"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/internal/sqlite"
)

// Open creates name inside a fresh temporary directory, opens it with
// the production settings, and closes it when the test ends.
func Open(tb testing.TB, name string) *qwr.Manager {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), name)
	db, err := sqlite.Open(path, sqlite.Settings{})
	if err != nil {
		tb.Fatalf("opening test database %s: %v", path, err)
	}
	tb.Cleanup(func() {
		if err := db.Close(); err != nil {
			tb.Errorf("closing test database %s: %v", path, err)
		}
	})
	return db
}
