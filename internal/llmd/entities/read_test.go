package entities_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/entities"
)

// TestRead covers reading an entity back by key, with and without a
// relation, and the unknown-key error. setup returns the key to read.
func TestRead(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, s *llmd.Store) string
		wantErr       error
		wantNamespace string
		wantValue     string
		wantRelation  string
	}{
		{
			name: "found",
			setup: func(t *testing.T, s *llmd.Store) string {
				return writeEntity(t, s, "test:item", `{"name":"foo"}`, "")
			},
			wantNamespace: "test:item",
			wantValue:     `{"name":"foo"}`,
		},
		{
			name:    "not found",
			setup:   func(t *testing.T, s *llmd.Store) string { return "nonexistent" },
			wantErr: entities.ErrNotFound,
		},
		{
			name: "with relation",
			setup: func(t *testing.T, s *llmd.Store) string {
				return writeEntity(t, s, "test:item", `{"x":1}`, "docs/readme")
			},
			wantNamespace: "test:item",
			wantValue:     `{"x":1}`,
			wantRelation:  "docs/readme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			key := tt.setup(t, s)

			got, err := s.Entities.Read(t.Context(), key)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Read(%q) error = %v, want %v", key, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Key != key {
				t.Errorf("Read(%q) Key = %q, want %q", key, got.Key, key)
			}
			if got.Namespace != tt.wantNamespace {
				t.Errorf("Read(%q) Namespace = %q, want %q", key, got.Namespace, tt.wantNamespace)
			}
			if got.Value != tt.wantValue {
				t.Errorf("Read(%q) Value = %q, want %q", key, got.Value, tt.wantValue)
			}
			if got.Relation != tt.wantRelation {
				t.Errorf("Read(%q) Relation = %q, want %q", key, got.Relation, tt.wantRelation)
			}
		})
	}
}
