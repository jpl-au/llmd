// testhelper.go exports test setup for packages that need SDK globals
// wired without reaching into internal packages directly.

package host

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/sqlite/sqlitetest"
)

// TestMode controls how TestSetup initialises the store. Both modes
// create a real on-disk store under a temporary directory; tests never
// run against in-memory SQLite because WAL files, checkpoints and the
// on-demand work.db are part of the behaviour under test.
type TestMode int

const (
	// TestIsolated creates the store in a temporary directory and
	// leaves the working directory alone. Use for everything that
	// does not read files relative to the current directory.
	TestIsolated TestMode = iota

	// TestWorkdir creates the store in a temporary directory and
	// changes the working directory to it. Use when tests need
	// cwd-relative behaviour (git operations, .llmd/config,
	// .llmd/.gitignore).
	TestWorkdir
)

// TestHome points the user's home directory at an empty temporary
// directory for the rest of the test, so config.Load never sees the
// global config of whoever runs the tests. It sets both HOME and
// USERPROFILE because os.UserHomeDir reads the latter on Windows.
func TestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// TestSetup initialises a store and returns a Host over it, with
// cleanup registered. The store is opened with [sqlitetest.Settings]. Use h.Context(t.Context(), author) for the
// sdk.Context a command would receive and h.Dir() for the directory
// that holds the .llmd store.
func TestSetup(t *testing.T, mode TestMode, opts ...Option) *Host {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, ".llmd", "llmd.db")
	db := engine.WithDatabase(sqlitetest.Settings())

	switch mode {
	case TestIsolated:
		store, err := engine.Init(dbPath, db)
		if err != nil {
			t.Fatalf("engine.Init: %v", err)
		}
		t.Cleanup(func() {
			if err := store.Close(); err != nil {
				t.Errorf("closing store: %v", err)
			}
		})
		return setup(store, opts...)

	case TestWorkdir:
		orig, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chdir(orig); err != nil {
				t.Errorf("restoring working directory: %v", err)
			}
		})
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}

		store, err := engine.Init(dbPath, db)
		if err != nil {
			t.Fatalf("engine.Init: %v", err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("closing init store: %v", err)
		}

		store, err = engine.Open(dbPath, db)
		if err != nil {
			t.Fatalf("engine.Open: %v", err)
		}
		h := setup(store, opts...)
		t.Cleanup(func() {
			if err := h.Close(); err != nil {
				t.Errorf("closing host: %v", err)
			}
		})
		return h

	default:
		t.Fatalf("unknown TestMode %d", mode)
		return nil
	}
}
