package host

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
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

	other, err := llmd.Open(h.current().Path())
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

	t.Run("a work.db created outside is reported", func(t *testing.T) {
		if _, err := other.Tasks.Add(ctx, "Outside", []byte("# Outside\n\nspec"), tasks.AddOptions{Origin: origin}); err != nil {
			t.Fatal(err)
		}
		got := waitFor(t, 2)
		if got[len(got)-1] != "work.db" {
			t.Errorf("changes = %v, want work.db last", got)
		}
	})

	t.Run("further outside work changes are reported", func(t *testing.T) {
		before := len(snapshot())
		if _, err := other.Tasks.Add(ctx, "Again", []byte("# Again\n\nspec"), tasks.AddOptions{Origin: origin}); err != nil {
			t.Fatal(err)
		}
		got := waitFor(t, before+1)
		if got[len(got)-1] != "work.db" {
			t.Errorf("changes = %v, want work.db last", got)
		}
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

// TestWatchWithoutStore checks that a storeless host's Watch returns
// at once rather than polling nothing.
func TestWatchWithoutStore(t *testing.T) {
	h := New()
	if err := h.Watch(t.Context(), time.Millisecond); err != nil {
		t.Errorf("Watch() = %v, want nil", err)
	}
}
