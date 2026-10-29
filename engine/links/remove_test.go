package links_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/links"
)

// TestRemove covers removing links by path and by key, removing one
// labelled link while another survives, removing every label at once,
// and the missing-link error. Every case starts with a fresh store
// holding docs/api, docs/models and docs/auth; wantLabel, when set, is
// checked against the single link left from docs/api.
func TestRemove(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, s *engine.Store)
		from, to  string
		byKey     bool // address both documents by key instead of path
		label     string
		wantErr   error
		wantCount int // links listed from docs/api after Remove
		wantLabel string
	}{
		{
			name:  "by path",
			setup: func(t *testing.T, s *engine.Store) { addLink(t, s, "docs/api", "docs/models", "") },
			from:  "docs/api",
			to:    "docs/models",
		},
		{
			name: "with label",
			setup: func(t *testing.T, s *engine.Store) {
				addLink(t, s, "docs/api", "docs/auth", "requires")
				addLink(t, s, "docs/api", "docs/auth", "related")
			},
			from:      "docs/api",
			to:        "docs/auth",
			label:     "requires",
			wantCount: 1,
			wantLabel: "related",
		},
		{
			name: "all labels",
			setup: func(t *testing.T, s *engine.Store) {
				addLink(t, s, "docs/api", "docs/auth", "requires")
				addLink(t, s, "docs/api", "docs/auth", "related")
			},
			from: "docs/api",
			to:   "docs/auth",
		},
		{
			name:    "not found",
			from:    "docs/api",
			to:      "docs/models",
			wantErr: links.ErrNotFound,
		},
		{
			name:  "by key",
			setup: func(t *testing.T, s *engine.Store) { addLink(t, s, "docs/api", "docs/models", "") },
			from:  "docs/api",
			to:    "docs/models",
			byKey: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()

			keys := writeDocs(t, s, "docs/api", "docs/models", "docs/auth")
			if tt.setup != nil {
				tt.setup(t, s)
			}

			from, to := tt.from, tt.to
			if tt.byKey {
				from, to = keys[from], keys[to]
			}
			opts := testOpts()
			opts.Label = tt.label

			err := s.Links.Remove(ctx, from, to, opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Remove(%q, %q, label=%q) error = %v, want %v", from, to, tt.label, err, tt.wantErr)
			}

			list, err := s.Links.List(ctx, "docs/api", testOpts())
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(list) != tt.wantCount {
				t.Fatalf("List() after Remove returned %d links, want %d", len(list), tt.wantCount)
			}
			if tt.wantLabel != "" && list[0].Value.Label != tt.wantLabel {
				t.Errorf("List()[0].Value.Label = %q, want %q", list[0].Value.Label, tt.wantLabel)
			}
		})
	}
}
