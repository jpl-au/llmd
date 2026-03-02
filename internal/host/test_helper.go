// testhelper.go exports test setup for packages that need SDK globals
// wired without reaching into internal packages directly.

package host

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
)

// TestMode controls how TestSetup initialises the store.
type TestMode int

const (
	// TestMemory uses an in-memory store. Fast, no disk I/O.
	TestMemory TestMode = iota

	// TestDisk creates a disk-backed store in a temporary directory
	// and changes the working directory to it. Use when tests need
	// a real filesystem (e.g. git operations alongside the store).
	TestDisk
)

// TestSetup initialises a store, wires SDK globals, and registers
// cleanup. Returns the working directory (empty for TestMemory).
func TestSetup(t *testing.T, mode TestMode) string {
	t.Helper()

	switch mode {
	case TestMemory:
		store, err := llmd.OpenMemory()
		if err != nil {
			t.Fatalf("OpenMemory: %v", err)
		}
		t.Cleanup(func() {
			if err := store.Close(); err != nil {
				t.Errorf("closing store: %v", err)
			}
		})
		setup(store)
		return ""

	case TestDisk:
		dir := t.TempDir()
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

		dbPath := filepath.Join(dir, ".llmd", "llmd.db")
		store, err := llmd.Init(dbPath)
		if err != nil {
			t.Fatalf("llmd.Init: %v", err)
		}
		if err := store.Close(); err != nil {
			t.Fatalf("closing init store: %v", err)
		}

		h, err := Open(dbPath)
		if err != nil {
			t.Fatalf("host.Open: %v", err)
		}
		t.Cleanup(func() {
			if err := h.Close(); err != nil {
				t.Errorf("closing host: %v", err)
			}
		})
		return dir

	default:
		t.Fatalf("unknown TestMode %d", mode)
		return ""
	}
}
