package links_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/links"
)

// TestList covers listing a document's links by path and by key, the
// empty case, and each Direction. Every case starts with a fresh store
// holding docs/api, docs/models and docs/auth. wantTo and wantRelation,
// when set, are checked against the single listed link.
func TestList(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T, s *llmd.Store)
		path         string
		byKey        bool // address the document by key instead of path
		direction    links.Direction
		want         int // number of links listed
		wantTo       string
		wantRelation string
	}{
		{
			name: "two outgoing",
			setup: func(t *testing.T, s *llmd.Store) {
				addLink(t, s, "docs/api", "docs/models", "")
				addLink(t, s, "docs/api", "docs/auth", "")
			},
			path: "docs/api",
			want: 2,
		},
		{
			name:  "by key",
			setup: func(t *testing.T, s *llmd.Store) { addLink(t, s, "docs/api", "docs/models", "") },
			path:  "docs/api",
			byKey: true,
			want:  1,
		},
		{
			name: "empty",
			path: "docs/api",
			want: 0,
		},
		{
			name:      "outgoing",
			setup:     func(t *testing.T, s *llmd.Store) { addLink(t, s, "docs/api", "docs/models", "") },
			path:      "docs/api",
			direction: links.Outgoing,
			want:      1,
			wantTo:    "docs/models",
		},
		{
			// Query incoming links TO docs/models.
			name:         "incoming",
			setup:        func(t *testing.T, s *llmd.Store) { addLink(t, s, "docs/api", "docs/models", "") },
			path:         "docs/models",
			direction:    links.Incoming,
			want:         1,
			wantRelation: "docs/api",
		},
		{
			// api -> models and auth -> api both involve docs/api.
			name: "both",
			setup: func(t *testing.T, s *llmd.Store) {
				addLink(t, s, "docs/api", "docs/models", "")
				addLink(t, s, "docs/auth", "docs/api", "")
			},
			path:      "docs/api",
			direction: links.Both,
			want:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)

			keys := writeDocs(t, s, "docs/api", "docs/models", "docs/auth")
			if tt.setup != nil {
				tt.setup(t, s)
			}

			value := tt.path
			if tt.byKey {
				value = keys[value]
			}
			opts := testOpts()
			opts.Direction = tt.direction

			got, err := s.Links.List(t.Context(), value, opts)
			if err != nil {
				t.Fatalf("List(%q) error = %v", value, err)
			}
			if len(got) != tt.want {
				t.Fatalf("List(%q) returned %d links, want %d", value, len(got), tt.want)
			}
			if tt.wantTo != "" && got[0].Value.To != tt.wantTo {
				t.Errorf("List(%q)[0].Value.To = %q, want %q", value, got[0].Value.To, tt.wantTo)
			}
			if tt.wantRelation != "" && got[0].Relation != tt.wantRelation {
				t.Errorf("List(%q)[0].Relation = %q, want %q", value, got[0].Relation, tt.wantRelation)
			}
		})
	}
}
