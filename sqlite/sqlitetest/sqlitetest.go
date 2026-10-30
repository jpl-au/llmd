// Package sqlitetest opens throwaway on-disk databases for tests with
// the same settings as production, except that writes are not flushed
// to disk (database.synchronous off). Tests never use in-memory SQLite: an in-memory database skips
// WAL, checkpointing and the filesystem, which is exactly what the store
// depends on.
package sqlitetest

import (
	"path/filepath"
	"testing"

	"github.com/jpl-au/qwr"

	"github.com/jpl-au/llmd/sqlite"
)

// Settings returns the settings test databases are opened with. They
// skip the flush to disk because a flush costs tens of milliseconds on
// Windows and a test run closes over a thousand databases.
func Settings() sqlite.Settings {
	return sqlite.Settings{Synchronous: sqlite.SyncOff}
}

// Open creates name inside a fresh temporary directory, opens it with
// [Settings], and closes it when the test ends.
func Open(tb testing.TB, name string) *qwr.Manager {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), name)
	db, err := sqlite.Open(path, Settings())
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
