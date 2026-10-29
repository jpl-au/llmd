package entities_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/entities"
)

// TestDelete covers soft-deleting an entity, a key that never existed,
// and a second delete of the same key. setup returns the key to delete;
// after Delete the key must not be readable in any case.
func TestDelete(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, s *engine.Store) string
		wantErr error
	}{
		{
			name: "existing",
			setup: func(t *testing.T, s *engine.Store) string {
				return writeEntity(t, s, "test:item", `{"name":"foo"}`, "")
			},
		},
		{
			name:    "not found",
			setup:   func(t *testing.T, s *engine.Store) string { return "nonexistent" },
			wantErr: entities.ErrNotFound,
		},
		{
			name: "already deleted",
			setup: func(t *testing.T, s *engine.Store) string {
				key := writeEntity(t, s, "test:item", `{"name":"foo"}`, "")
				if err := s.Entities.Delete(t.Context(), key, testDeleteOpts()); err != nil {
					t.Fatalf("Delete(%s) error = %v", key, err)
				}
				return key
			},
			wantErr: entities.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			key := tt.setup(t, s)

			err := s.Entities.Delete(ctx, key, testDeleteOpts())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Delete(%q) error = %v, want %v", key, err, tt.wantErr)
			}

			if _, err := s.Entities.Read(ctx, key); !errors.Is(err, entities.ErrNotFound) {
				t.Errorf("Read(%q) after Delete error = %v, want %v", key, err, entities.ErrNotFound)
			}
		})
	}
}
