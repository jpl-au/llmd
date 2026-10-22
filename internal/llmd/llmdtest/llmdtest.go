// Package llmdtest opens throwaway on-disk stores for tests. Each store
// lives in its own temporary .llmd directory created through llmd.Init,
// the same path production takes, so tests exercise WAL files,
// checkpoints and the on-demand work.db exactly as a user would.
package llmdtest

import (
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
)

// Open initialises a new store under a temporary directory and closes
// it when the test ends. The working directory is left untouched.
func Open(tb testing.TB) *llmd.Store {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), ".llmd", "llmd.db")
	s, err := llmd.Init(path)
	if err != nil {
		tb.Fatalf("llmd.Init(%s): %v", path, err)
	}
	tb.Cleanup(func() {
		if err := s.Close(); err != nil {
			tb.Errorf("closing store: %v", err)
		}
	})
	return s
}
