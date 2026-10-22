package tags_test

import (
	"slices"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/tags"

	"github.com/jpl-au/llmd/internal/llmd"
)

// TestPurge covers hard-deleting removed tags: the count returned and the
// tags that survive on "docs/test" afterwards, and the empty store.
func TestPurge(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, s *llmd.Store)
		want          int64    // tags purged
		wantRemaining []string // tag names still on docs/test
	}{
		{
			name: "removed tags purged",
			setup: func(t *testing.T, s *llmd.Store) {
				writeTagged(t, s, "docs/test", "important", "review", "draft")
				for _, name := range []string{"important", "review"} {
					if err := s.Tags.Remove(t.Context(), "docs/test", name, testOpts()); err != nil {
						t.Fatalf("Remove(%s) error = %v", name, err)
					}
				}
			},
			want:          2,
			wantRemaining: []string{"draft"},
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

			got, err := s.Tags.Purge(ctx, tags.PurgeOptions{})
			if err != nil {
				t.Fatalf("Purge() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Purge() = %d, want %d", got, tt.want)
			}

			list, err := s.Tags.List(ctx, "docs/test")
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			var remaining []string
			for _, tg := range list {
				remaining = append(remaining, tg.Value.Tag)
			}
			if !slices.Equal(remaining, tt.wantRemaining) {
				t.Errorf("List() after Purge() = %v, want %v", remaining, tt.wantRemaining)
			}
		})
	}
}
