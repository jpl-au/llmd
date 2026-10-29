package host

import (
	"context"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/tasks"
	"github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/sdk"
)

// TestWatch covers the store watcher through the host: a change this
// process makes is not repeated as store.changed, a change another
// store handle makes to llmd.db is reported, a work.db that another
// handle creates is reported, and further work.db changes are too.
// The second handle stands in for another process: it has its own
// connections and its own bus, as a CLI run beside the server would.
func TestWatch(t *testing.T) {
	const interval = 20 * time.Millisecond
	h := TestSetup(t, TestIsolated)

	var mu sync.Mutex
	var seen []string
	h.Subscribe(func(e events.Event) {
		if e.Type != events.StoreChanged {
			return
		}
		mu.Lock()
		seen = append(seen, e.Metadata["db"].(string))
		mu.Unlock()
	})
	snapshot := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
	// waitFor polls until the seen list is at least n long or gives up.
	waitFor := func(t *testing.T, n int) []string {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if got := snapshot(); len(got) >= n {
				return got
			}
			time.Sleep(interval)
		}
		t.Fatalf("saw %v, want %d store.changed events", snapshot(), n)
		return nil
	}

	// waitForDB polls until db is among the changes after the first
	// from, or gives up.
	waitForDB := func(t *testing.T, from int, db string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if got := snapshot(); len(got) > from && slices.Contains(got[from:], db) {
				return
			}
			time.Sleep(interval)
		}
		t.Fatalf("saw %v, want %s among the changes after the first %d", snapshot(), db, from)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.Watch(ctx, interval) }()
	// Let the watcher take its baseline before anything changes.
	time.Sleep(5 * interval)

	t.Run("own changes are not repeated", func(t *testing.T) {
		c := h.Context(ctx, "alice")
		if err := c.Documents.Write("docs/mine", []byte("# Mine\n\nbody"), sdk.WriteOpts{Author: "alice"}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * interval)
		if got := snapshot(); len(got) != 0 {
			t.Errorf("after an own write saw %v, want nothing", got)
		}
	})

	other, err := engine.Open(h.current().Path())
	if err != nil {
		t.Fatalf("opening a second store handle: %v", err)
	}
	defer func() {
		if err := other.Close(); err != nil {
			t.Errorf("closing the second store handle: %v", err)
		}
	}()
	origin := core.Origin{Author: "bob", Source: "test"}

	t.Run("an outside document write is reported", func(t *testing.T) {
		if _, err := other.Documents.Write(ctx, "docs/theirs", "# Theirs\n\nbody", documents.WriteOptions{Origin: origin}); err != nil {
			t.Fatal(err)
		}
		got := waitFor(t, 1)
		if got[0] != "llmd.db" {
			t.Errorf("first change = %q, want llmd.db", got[0])
		}
	})

	// A task add writes both files, the spec to llmd.db and the task
	// to work.db, and polls may see them in either order or in
	// different ticks: wait for work.db among the new changes rather
	// than assume where it falls.
	t.Run("a work.db created outside is reported", func(t *testing.T) {
		before := len(snapshot())
		if _, err := other.Tasks.Add(ctx, "Outside", []byte("# Outside\n\nspec"), tasks.AddOptions{Origin: origin}); err != nil {
			t.Fatal(err)
		}
		waitForDB(t, before, "work.db")
	})

	t.Run("further outside work changes are reported", func(t *testing.T) {
		before := len(snapshot())
		if _, err := other.Tasks.Add(ctx, "Again", []byte("# Again\n\nspec"), tasks.AddOptions{Origin: origin}); err != nil {
			t.Fatal(err)
		}
		waitForDB(t, before, "work.db")
	})

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Watch() = %v, want nil after cancel", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Watch() did not return after cancel")
	}
}

// TestWatchBeforeStore covers a host that starts without a store, as
// "llmd serve" does before "init": Watch waits rather than returning,
// stops cleanly when cancelled, and once a store is attached reports
// the changes another handle makes to it.
func TestWatchBeforeStore(t *testing.T) {
	const interval = 20 * time.Millisecond

	t.Run("cancelled while waiting", func(t *testing.T) {
		h := New()
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- h.Watch(ctx, interval) }()
		select {
		case err := <-done:
			t.Fatalf("Watch() returned %v before cancel, want it to wait for a store", err)
		case <-time.After(3 * interval):
		}
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Watch() = %v, want nil after cancel", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("Watch() did not return after cancel")
		}
	})

	t.Run("store attached later is watched", func(t *testing.T) {
		h := New()
		var mu sync.Mutex
		var seen []string
		h.Subscribe(func(e events.Event) {
			if e.Type != events.StoreChanged {
				return
			}
			mu.Lock()
			seen = append(seen, e.Metadata["db"].(string))
			mu.Unlock()
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- h.Watch(ctx, interval) }()
		time.Sleep(3 * interval)

		// What Exec does after a storeless "init" succeeds.
		dbPath := filepath.Join(t.TempDir(), ".llmd", "llmd.db")
		store, err := engine.Init(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		h.attach(store)
		t.Cleanup(func() {
			if err := h.Close(); err != nil {
				t.Errorf("closing host: %v", err)
			}
		})
		time.Sleep(5 * interval) // let the watcher take its baseline

		other, err := engine.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := other.Close(); err != nil {
				t.Errorf("closing the second store handle: %v", err)
			}
		}()
		origin := core.Origin{Author: "bob", Source: "test"}
		if _, err := other.Documents.Write(ctx, "docs/theirs", "# Theirs\n\nbody", documents.WriteOptions{Origin: origin}); err != nil {
			t.Fatal(err)
		}

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			n := len(seen)
			mu.Unlock()
			if n > 0 {
				break
			}
			time.Sleep(interval)
		}
		mu.Lock()
		defer mu.Unlock()
		if len(seen) == 0 || seen[0] != "llmd.db" {
			t.Errorf("after attach saw %v, want a store.changed for llmd.db", seen)
		}
	})
}

// TestWatchInterval checks that a non-positive interval is refused
// rather than spun on.
func TestWatchInterval(t *testing.T) {
	if err := New().Watch(t.Context(), 0); err == nil {
		t.Error("Watch(0) = nil, want an error")
	}
}
