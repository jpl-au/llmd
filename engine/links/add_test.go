package links_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine/documents"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/links"
)

// TestAdd covers linking by path and by key, labels, the self-link and
// duplicate errors, and that different labels make distinct links. Every
// case starts with a fresh store holding docs/api, docs/models and
// docs/auth; setup adds any pre-existing links.
func TestAdd(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T, s *engine.Store)
		from, to     string
		byKey        bool // address both documents by key instead of path
		label        string
		wantErr      error
		wantExisting bool // returned link must be the one already stored
		wantCount    int  // links listed from docs/api after Add
	}{
		{
			name:      "by path",
			from:      "docs/api",
			to:        "docs/models",
			wantCount: 1,
		},
		{
			name:      "with label",
			from:      "docs/api",
			to:        "docs/auth",
			label:     "requires",
			wantCount: 1,
		},
		{
			name:    "missing target",
			from:    "docs/api",
			to:      "docs/missing",
			wantErr: documents.ErrNotFound,
		},
		{
			name:    "missing source",
			from:    "docs/missing",
			to:      "docs/api",
			wantErr: documents.ErrNotFound,
		},
		{
			name:      "by key",
			from:      "docs/api",
			to:        "docs/models",
			byKey:     true,
			wantCount: 1,
		},
		{
			name:    "self link",
			from:    "docs/api",
			to:      "docs/api",
			wantErr: links.ErrSelfLink,
		},
		{
			name: "duplicate",
			setup: func(t *testing.T, s *engine.Store) {
				addLink(t, s, "docs/api", "docs/models", "")
			},
			from:         "docs/api",
			to:           "docs/models",
			wantErr:      links.ErrExists,
			wantExisting: true,
			wantCount:    1,
		},
		{
			name: "multiple labels",
			setup: func(t *testing.T, s *engine.Store) {
				addLink(t, s, "docs/api", "docs/auth", "requires")
			},
			from:      "docs/api",
			to:        "docs/auth",
			label:     "related",
			wantCount: 2,
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

			got, err := s.Links.Add(ctx, from, to, opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Add(%q, %q) error = %v, want %v", from, to, err, tt.wantErr)
			}
			if got == nil {
				return // error case with no link to inspect
			}
			if got.Relation != tt.from {
				t.Errorf("Add(%q, %q) Relation = %q, want %q", from, to, got.Relation, tt.from)
			}
			if got.Value.To != tt.to {
				t.Errorf("Add(%q, %q) Value.To = %q, want %q", from, to, got.Value.To, tt.to)
			}
			if got.Value.Label != tt.label {
				t.Errorf("Add(%q, %q) Value.Label = %q, want %q", from, to, got.Value.Label, tt.label)
			}
			if len(got.Key) != 10 {
				t.Errorf("Add(%q, %q) Key length = %d, want 9", from, to, len(got.Key))
			}

			list, err := s.Links.List(ctx, "docs/api", testOpts())
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(list) != tt.wantCount {
				t.Errorf("List() returned %d links, want %d", len(list), tt.wantCount)
			}
			if tt.wantExisting && (len(list) != 1 || list[0].Key != got.Key) {
				t.Errorf("Add(%q, %q) Key = %q, want existing link %v", from, to, got.Key, list)
			}
		})
	}
}
