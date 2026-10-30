// Package enginetest opens throwaway on-disk stores for tests. Each store
// lives in its own temporary .llmd directory created through engine.Init,
// the same path production takes, so tests exercise WAL files,
// checkpoints and the on-demand work.db exactly as a user would.
package enginetest

import (
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/sqlite/sqlitetest"
)

// Open initialises a new store under a temporary directory and closes
// it when the test ends. The working directory is left untouched.
func Open(tb testing.TB) *engine.Store {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), ".llmd", "llmd.db")
	s, err := engine.Init(path, engine.WithDatabase(sqlitetest.Settings()))
	if err != nil {
		tb.Fatalf("engine.Init(%s): %v", path, err)
	}
	tb.Cleanup(func() {
		if err := s.Close(); err != nil {
			tb.Errorf("closing store: %v", err)
		}
	})
	return s
}
