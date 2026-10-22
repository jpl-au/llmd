package tags_test

import (
	"slices"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/tags"
)

// writeTagged writes a document and adds the given tags to it.
func writeTagged(t *testing.T, s *llmd.Store, path string, names ...string) {
	t.Helper()
	ctx := t.Context()
	if _, err := s.Documents.Write(ctx, path, "content "+path, testWriteOpts()); err != nil {
		t.Fatalf("Write(%s) error = %v", path, err)
	}
	for _, name := range names {
		if _, err := s.Tags.Add(ctx, path, name, testOpts()); err != nil {
			t.Fatalf("Add(%s, %s) error = %v", path, name, err)
		}
	}
}

// TestFind covers looking up documents by tag, with and without a
// relation prefix, when nothing matches, and after the tag is removed.
// Find orders results by relation, so want is compared exactly.
func TestFind(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, s *llmd.Store)
		tag   string
		opts  []tags.FindOptions
		want  []string
	}{
		{
			name: "all relations",
			setup: func(t *testing.T, s *llmd.Store) {
				writeTagged(t, s, "docs/a", "important")
				writeTagged(t, s, "docs/b", "important")
				writeTagged(t, s, "docs/c", "draft")
				writeTagged(t, s, "notes/d", "important")
			},
			tag:  "important",
			want: []string{"docs/a", "docs/b", "notes/d"},
		},
		{
			name: "with relation prefix",
			setup: func(t *testing.T, s *llmd.Store) {
				writeTagged(t, s, "docs/a", "important")
				writeTagged(t, s, "docs/b", "important")
				writeTagged(t, s, "notes/c", "important")
			},
			tag:  "important",
			opts: []tags.FindOptions{{RelationPrefix: "docs/"}},
			want: []string{"docs/a", "docs/b"},
		},
		{
			name: "no results",
			setup: func(t *testing.T, s *llmd.Store) {
				writeTagged(t, s, "docs/a", "existing")
			},
			tag:  "nonexistent",
			want: nil,
		},
		{
			name: "excludes removed",
			setup: func(t *testing.T, s *llmd.Store) {
				writeTagged(t, s, "docs/a", "important")
				if err := s.Tags.Remove(t.Context(), "docs/a", "important", testOpts()); err != nil {
					t.Fatalf("Remove() error = %v", err)
				}
			},
			tag:  "important",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			tt.setup(t, s)

			got, err := s.Tags.Find(t.Context(), tt.tag, tt.opts...)
			if err != nil {
				t.Fatalf("Find(%q, %v) error = %v", tt.tag, tt.opts, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Find(%q, %v) = %v, want %v", tt.tag, tt.opts, got, tt.want)
			}
		})
	}
}
