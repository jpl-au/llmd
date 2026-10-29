package links_test

import (
	"slices"
	"testing"

	"github.com/jpl-au/llmd/engine/links"

	"github.com/jpl-au/llmd/engine"
)

// TestPurge covers hard-deleting removed links: the count returned and
// the links that survive from docs/b afterwards, and the empty store.
func TestPurge(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, s *engine.Store)
		want          int64    // links purged
		wantRemaining []string // targets still linked from docs/b
	}{
		{
			name: "removed links purged",
			setup: func(t *testing.T, s *engine.Store) {
				writeDocs(t, s, "docs/a", "docs/b", "docs/c")
				addLink(t, s, "docs/a", "docs/b", "")
				addLink(t, s, "docs/a", "docs/c", "")
				addLink(t, s, "docs/b", "docs/c", "")
				for _, to := range []string{"docs/b", "docs/c"} {
					if err := s.Links.Remove(t.Context(), "docs/a", to, testOpts()); err != nil {
						t.Fatalf("Remove(docs/a, %s) error = %v", to, err)
					}
				}
			},
			want:          2,
			wantRemaining: []string{"docs/c"},
		},
		{
			name:          "nothing to purge",
			want:          0,
			wantRemaining: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			if tt.setup != nil {
				tt.setup(t, s)
			}

			got, err := s.Links.Purge(ctx, links.PurgeOptions{})
			if err != nil {
				t.Fatalf("Purge() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Purge() = %d, want %d", got, tt.want)
			}

			list, err := s.Links.List(ctx, "docs/b")
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			var remaining []string
			for _, lk := range list {
				remaining = append(remaining, lk.Value.To)
			}
			if !slices.Equal(remaining, tt.wantRemaining) {
				t.Errorf("List() after Purge() = %v, want %v", remaining, tt.wantRemaining)
			}
		})
	}
}
