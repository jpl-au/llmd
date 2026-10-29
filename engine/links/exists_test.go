package links_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/links"
)

// TestExists covers link presence checks by path and by key, with and
// without a label filter, for a missing link and a removed one. Every
// case starts with a fresh store holding docs/a and docs/b.
func TestExists(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, s *engine.Store)
		byKey bool // address both documents by key instead of path
		opts  []links.Options
		want  bool
	}{
		{
			name:  "found",
			setup: func(t *testing.T, s *engine.Store) { addLink(t, s, "docs/a", "docs/b", "") },
			want:  true,
		},
		{
			name: "not found",
			want: false,
		},
		{
			name:  "matching label",
			setup: func(t *testing.T, s *engine.Store) { addLink(t, s, "docs/a", "docs/b", "related") },
			opts:  []links.Options{{Label: "related"}},
			want:  true,
		},
		{
			name:  "different label",
			setup: func(t *testing.T, s *engine.Store) { addLink(t, s, "docs/a", "docs/b", "related") },
			opts:  []links.Options{{Label: "other"}},
			want:  false,
		},
		{
			name:  "labelled link without label filter",
			setup: func(t *testing.T, s *engine.Store) { addLink(t, s, "docs/a", "docs/b", "related") },
			want:  true,
		},
		{
			name:  "by key",
			setup: func(t *testing.T, s *engine.Store) { addLink(t, s, "docs/a", "docs/b", "") },
			byKey: true,
			want:  true,
		},
		{
			name: "removed link",
			setup: func(t *testing.T, s *engine.Store) {
				addLink(t, s, "docs/a", "docs/b", "")
				if err := s.Links.Remove(t.Context(), "docs/a", "docs/b", testOpts()); err != nil {
					t.Fatalf("Remove() error = %v", err)
				}
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)

			keys := writeDocs(t, s, "docs/a", "docs/b")
			if tt.setup != nil {
				tt.setup(t, s)
			}

			from, to := "docs/a", "docs/b"
			if tt.byKey {
				from, to = keys[from], keys[to]
			}

			got, err := s.Links.Exists(t.Context(), from, to, tt.opts...)
			if err != nil {
				t.Fatalf("Exists(%q, %q, %v) error = %v", from, to, tt.opts, err)
			}
			if got != tt.want {
				t.Errorf("Exists(%q, %q, %v) = %v, want %v", from, to, tt.opts, got, tt.want)
			}
		})
	}
}
