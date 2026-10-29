package engine_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/jpl-au/llmd/clock/clocktest"
	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/enginetest"
	"github.com/jpl-au/llmd/engine/links"
	"github.com/jpl-au/llmd/engine/messages"
	"github.com/jpl-au/llmd/engine/tags"
)

func testStore(t *testing.T) *engine.Store {
	t.Helper()
	return enginetest.Open(t)
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Author: "test", Source: "cli"}
}

func testDeleteOpts() documents.DeleteOptions {
	return documents.DeleteOptions{Author: "test", Source: "cli"}
}

func testTagOpts() tags.Options {
	return tags.Options{Author: "test", Source: "cli"}
}

func testLinkOpts() links.Options {
	return links.Options{Author: "test", Source: "cli"}
}

// seedDeleted builds the fixture every vacuum case starts from, at a
// fixed clock: docs/a and docs/b written at t=1000; docs/a carries two
// tags and a link to docs/b; then at t=2000 docs/a is deleted, one of
// its tags is removed and the link is removed.
func seedDeleted(t *testing.T, s *engine.Store) {
	t.Helper()
	ctx := t.Context()

	clocktest.Fixed(t, 1_000)
	for _, p := range []string{"docs/a", "docs/b"} {
		if _, err := s.Documents.Write(ctx, p, "content "+p, testWriteOpts()); err != nil {
			t.Fatalf("Write(%s) error = %v", p, err)
		}
	}
	for _, name := range []string{"important", "review"} {
		if _, err := s.Tags.Add(ctx, "docs/a", name, testTagOpts()); err != nil {
			t.Fatalf("Tags.Add(%s) error = %v", name, err)
		}
	}
	if _, err := s.Links.Add(ctx, "docs/a", "docs/b", testLinkOpts()); err != nil {
		t.Fatalf("Links.Add() error = %v", err)
	}

	clocktest.Fixed(t, 2_000)
	if err := s.Documents.Delete(ctx, "docs/a", testDeleteOpts()); err != nil {
		t.Fatalf("Delete(docs/a) error = %v", err)
	}
	if err := s.Tags.Remove(ctx, "docs/a", "important", testTagOpts()); err != nil {
		t.Fatalf("Tags.Remove() error = %v", err)
	}
	if err := s.Links.Remove(ctx, "docs/a", "docs/b", testLinkOpts()); err != nil {
		t.Fatalf("Links.Remove() error = %v", err)
	}
}

// TestVacuum covers a full purge, a dry run, and the age filter on
// either side of the deletion time. Counts: docs/a is one document; the
// explicitly removed tag plus the tag orphaned by purging docs/a make
// two tags; the removed link is one link.
func TestVacuum(t *testing.T) {
	full := engine.VacuumResult{Documents: 1, Tags: 2, Links: 1, Paths: []string{"docs/a"}}
	none := engine.VacuumResult{}

	tests := []struct {
		name      string
		seed      bool
		opts      engine.VacuumOptions
		want      engine.VacuumResult
		wantGone  bool                // docs/a is no longer in the store at all
		wantKept  bool                // docs/a is still soft-deleted (restorable)
		wantAfter engine.VacuumResult // a following full vacuum
	}{
		{
			name:      "empty store",
			opts:      engine.VacuumOptions{},
			want:      none,
			wantAfter: none,
		},
		{
			name:      "purges everything",
			seed:      true,
			opts:      engine.VacuumOptions{},
			want:      full,
			wantGone:  true,
			wantAfter: none,
		},
		{
			name:      "dry run changes nothing",
			seed:      true,
			opts:      engine.VacuumOptions{DryRun: true},
			want:      withDryRun(full),
			wantKept:  true,
			wantAfter: full,
		},
		{
			name:      "older-than before the deletion keeps it",
			seed:      true,
			opts:      engine.VacuumOptions{Before: 1_500},
			want:      none,
			wantKept:  true,
			wantAfter: full,
		},
		{
			name:      "older-than after the deletion purges it",
			seed:      true,
			opts:      engine.VacuumOptions{Before: 2_500},
			want:      full,
			wantGone:  true,
			wantAfter: none,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			if tt.seed {
				seedDeleted(t, s)
			}

			got, err := s.Vacuum(ctx, tt.opts)
			if err != nil {
				t.Fatalf("Vacuum(%+v) error = %v", tt.opts, err)
			}
			if !equalResult(*got, tt.want) {
				t.Errorf("Vacuum(%+v) = %+v, want %+v", tt.opts, *got, tt.want)
			}

			_, err = s.Documents.Read(ctx, "docs/a")
			if tt.wantGone && !errors.Is(err, documents.ErrNotFound) {
				t.Errorf("Read(docs/a) after vacuum error = %v, want ErrNotFound", err)
			}
			if tt.wantKept && !errors.Is(err, documents.ErrDeleted) {
				t.Errorf("Read(docs/a) after vacuum error = %v, want ErrDeleted (still restorable)", err)
			}

			after, err := s.Vacuum(ctx, engine.VacuumOptions{})
			if err != nil {
				t.Fatalf("second Vacuum() error = %v", err)
			}
			if !equalResult(*after, tt.wantAfter) {
				t.Errorf("second Vacuum() = %+v, want %+v", *after, tt.wantAfter)
			}
		})
	}
}

// TestVacuumResult_Total checks Total sums the three purge counts and
// ignores the path list and dry-run flag.
func TestVacuumResult_Total(t *testing.T) {
	tests := []struct {
		name   string
		result engine.VacuumResult
		want   int64
	}{
		{"zero", engine.VacuumResult{}, 0},
		{"documents only", engine.VacuumResult{Documents: 3, Paths: []string{"a", "b", "c"}}, 3},
		{"tags only", engine.VacuumResult{Tags: 2}, 2},
		{"links only", engine.VacuumResult{Links: 1}, 1},
		{"all three", engine.VacuumResult{Documents: 1, Tags: 2, Links: 3}, 6},
		{"dry run counts the same", engine.VacuumResult{Documents: 1, Tags: 2, Links: 3, DryRun: true}, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Total(); got != tt.want {
				t.Errorf("VacuumResult%+v.Total() = %d, want %d", tt.result, got, tt.want)
			}
		})
	}
}

func withDryRun(r engine.VacuumResult) engine.VacuumResult {
	r.DryRun = true
	return r
}

func equalResult(a, b engine.VacuumResult) bool {
	return a.Documents == b.Documents && a.Tags == b.Tags && a.Links == b.Links &&
		a.DryRun == b.DryRun && slices.Equal(a.Paths, b.Paths)
}

// TestVacuumQueue covers the queue trim: with a retention set, messages
// older than it go (acknowledged or not) and newer ones stay; a dry run
// counts without deleting; without a retention nothing is touched; and
// a store that never opened work.db does not create it just to look.
func TestVacuumQueue(t *testing.T) {
	const hour = int64(60 * 60 * 1000)
	send := func(t *testing.T, s *engine.Store, at int64, text string) string {
		t.Helper()
		clocktest.Fixed(t, at)
		m, err := s.Messages.Send(t.Context(), messages.SendOptions{Type: "note", Payload: text, Author: "alice", Source: "test"})
		if err != nil {
			t.Fatal(err)
		}
		return m.Key
	}
	remaining := func(t *testing.T, s *engine.Store) int {
		t.Helper()
		msgs, err := s.Messages.History(t.Context(), "", 0)
		if err != nil {
			t.Fatal(err)
		}
		return len(msgs)
	}

	t.Run("older than the retention go", func(t *testing.T) {
		s := testStore(t)
		s.SetQueueRetention(time.Hour)
		first := send(t, s, 1_000, "old and acknowledged")
		send(t, s, 1_000+hour, "old")
		send(t, s, 1_000+3*hour, "new")
		if err := s.Messages.Ack(t.Context(), first, "claude"); err != nil {
			t.Fatal(err)
		}
		clocktest.Fixed(t, 1_000+3*hour)

		dry, err := s.Vacuum(t.Context(), engine.VacuumOptions{DryRun: true})
		if err != nil {
			t.Fatal(err)
		}
		if dry.Messages != 2 || remaining(t, s) != 3 {
			t.Errorf("dry run: Messages = %d, %d remain; want 2 counted and 3 remaining", dry.Messages, remaining(t, s))
		}
		r, err := s.Vacuum(t.Context(), engine.VacuumOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Messages != 2 || r.Total() != 2 || remaining(t, s) != 1 {
			t.Errorf("Messages = %d, Total = %d, %d remain; want 2, 2, 1", r.Messages, r.Total(), remaining(t, s))
		}
		// The acknowledgement went with its message: the consumer's
		// pending list is just the message that stayed.
		pending, err := s.Messages.Pending(t.Context(), "claude", 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) != 1 || pending[0].Payload != "new" {
			t.Errorf("pending after vacuum = %+v, want only the new message", pending)
		}
	})

	t.Run("no retention keeps everything", func(t *testing.T) {
		s := testStore(t)
		send(t, s, 1_000, "old")
		clocktest.Fixed(t, 1_000+100*hour)
		r, err := s.Vacuum(t.Context(), engine.VacuumOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Messages != 0 || remaining(t, s) != 1 {
			t.Errorf("Messages = %d, %d remain; want 0 and 1", r.Messages, remaining(t, s))
		}
	})

	t.Run("no work.db is left alone", func(t *testing.T) {
		s := testStore(t)
		s.SetQueueRetention(time.Hour)
		if _, err := s.Vacuum(t.Context(), engine.VacuumOptions{}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(s.Dir(), "work.db")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("work.db after vacuum: stat = %v, want it not to exist", err)
		}
	})
}
