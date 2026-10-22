package llmd_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/internal/clock/clocktest"
	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/internal/llmd/llmdtest"
	"github.com/jpl-au/llmd/internal/llmd/tags"
)

func testStore(t *testing.T) *llmd.Store {
	t.Helper()
	return llmdtest.Open(t)
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
func seedDeleted(t *testing.T, s *llmd.Store) {
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
	full := llmd.VacuumResult{Documents: 1, Tags: 2, Links: 1, Paths: []string{"docs/a"}}
	none := llmd.VacuumResult{}

	tests := []struct {
		name      string
		seed      bool
		opts      llmd.VacuumOptions
		want      llmd.VacuumResult
		wantGone  bool              // docs/a is no longer in the store at all
		wantKept  bool              // docs/a is still soft-deleted (restorable)
		wantAfter llmd.VacuumResult // a following full vacuum
	}{
		{
			name:      "empty store",
			opts:      llmd.VacuumOptions{},
			want:      none,
			wantAfter: none,
		},
		{
			name:      "purges everything",
			seed:      true,
			opts:      llmd.VacuumOptions{},
			want:      full,
			wantGone:  true,
			wantAfter: none,
		},
		{
			name:      "dry run changes nothing",
			seed:      true,
			opts:      llmd.VacuumOptions{DryRun: true},
			want:      withDryRun(full),
			wantKept:  true,
			wantAfter: full,
		},
		{
			name:      "older-than before the deletion keeps it",
			seed:      true,
			opts:      llmd.VacuumOptions{Before: 1_500},
			want:      none,
			wantKept:  true,
			wantAfter: full,
		},
		{
			name:      "older-than after the deletion purges it",
			seed:      true,
			opts:      llmd.VacuumOptions{Before: 2_500},
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

			after, err := s.Vacuum(ctx, llmd.VacuumOptions{})
			if err != nil {
				t.Fatalf("second Vacuum() error = %v", err)
			}
			if !equalResult(*after, tt.wantAfter) {
				t.Errorf("second Vacuum() = %+v, want %+v", *after, tt.wantAfter)
			}
		})
	}
}

func withDryRun(r llmd.VacuumResult) llmd.VacuumResult {
	r.DryRun = true
	return r
}

func equalResult(a, b llmd.VacuumResult) bool {
	return a.Documents == b.Documents && a.Tags == b.Tags && a.Links == b.Links &&
		a.DryRun == b.DryRun && slices.Equal(a.Paths, b.Paths)
}
