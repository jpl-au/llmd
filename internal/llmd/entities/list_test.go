package entities_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/entities"
)

// TestList covers listing by namespace, filtering by relation, an empty
// namespace, the Limit option, and exclusion of deleted entities.
func TestList(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, s *llmd.Store)
		namespace string
		opts      entities.ListOptions
		want      int // number of entities listed
	}{
		{
			name: "by namespace",
			setup: func(t *testing.T, s *llmd.Store) {
				writeEntity(t, s, "test:item", `{"name":"a"}`, "")
				writeEntity(t, s, "test:item", `{"name":"b"}`, "")
				writeEntity(t, s, "other:item", `{"name":"c"}`, "")
			},
			namespace: "test:item",
			want:      2,
		},
		{
			name: "with relation",
			setup: func(t *testing.T, s *llmd.Store) {
				writeEntity(t, s, "test:item", `{"x":1}`, "docs/a")
				writeEntity(t, s, "test:item", `{"x":2}`, "docs/a") // same relation
				writeEntity(t, s, "test:item", `{"x":3}`, "docs/b") // different relation
			},
			namespace: "test:item",
			opts:      entities.ListOptions{Relation: "docs/a"},
			want:      2,
		},
		{
			name:      "empty",
			namespace: "nonexistent:ns",
			want:      0,
		},
		{
			name: "limit",
			setup: func(t *testing.T, s *llmd.Store) {
				for range 5 {
					writeEntity(t, s, "test:item", `{"i":1}`, "")
				}
			},
			namespace: "test:item",
			opts:      entities.ListOptions{Limit: 3},
			want:      3,
		},
		{
			name: "excludes deleted",
			setup: func(t *testing.T, s *llmd.Store) {
				key := writeEntity(t, s, "test:item", `{"name":"a"}`, "")
				writeEntity(t, s, "test:item", `{"name":"b"}`, "")
				if err := s.Entities.Delete(t.Context(), key, testDeleteOpts()); err != nil {
					t.Fatalf("Delete(%s) error = %v", key, err)
				}
			},
			namespace: "test:item",
			want:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			if tt.setup != nil {
				tt.setup(t, s)
			}

			got, err := s.Entities.List(t.Context(), tt.namespace, tt.opts)
			if err != nil {
				t.Fatalf("List(%q, %+v) error = %v", tt.namespace, tt.opts, err)
			}
			if len(got) != tt.want {
				t.Errorf("List(%q, %+v) returned %d entities, want %d", tt.namespace, tt.opts, len(got), tt.want)
			}
		})
	}
}
