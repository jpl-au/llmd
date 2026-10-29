package entities_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/entities"
)

// TestPurge covers hard-deleting soft-deleted entities: the count
// returned and how many "test:item" entities remain, for a mix of
// deleted and live entities, an empty store, and live entities only.
func TestPurge(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, s *engine.Store)
		want          int64 // entities purged
		wantRemaining int   // test:item entities listed after Purge
	}{
		{
			name: "deleted entities purged",
			setup: func(t *testing.T, s *engine.Store) {
				for _, v := range []string{`{"name":"a"}`, `{"name":"b"}`} {
					key := writeEntity(t, s, "test:item", v, "")
					if err := s.Entities.Delete(t.Context(), key, testDeleteOpts()); err != nil {
						t.Fatalf("Delete(%s) error = %v", key, err)
					}
				}
				writeEntity(t, s, "test:item", `{"name":"c"}`, "")
			},
			want:          2,
			wantRemaining: 1,
		},
		{
			name:          "nothing to purge",
			want:          0,
			wantRemaining: 0,
		},
		{
			name: "live entities untouched",
			setup: func(t *testing.T, s *engine.Store) {
				writeEntity(t, s, "test:item", `{"name":"a"}`, "")
				writeEntity(t, s, "test:item", `{"name":"b"}`, "")
			},
			want:          0,
			wantRemaining: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			if tt.setup != nil {
				tt.setup(t, s)
			}

			got, err := s.Entities.Purge(ctx)
			if err != nil {
				t.Fatalf("Purge() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Purge() = %d, want %d", got, tt.want)
			}

			list, err := s.Entities.List(ctx, "test:item", entities.ListOptions{})
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(list) != tt.wantRemaining {
				t.Errorf("List() after Purge() returned %d entities, want %d", len(list), tt.wantRemaining)
			}
		})
	}
}
