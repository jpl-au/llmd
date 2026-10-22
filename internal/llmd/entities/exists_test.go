package entities_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/entities"
)

// TestExists covers presence checks by key for a written entity, an
// unknown key, and a deleted entity. setup returns the key to check.
func TestExists(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, s *llmd.Store) string
		want  bool
	}{
		{
			name: "found",
			setup: func(t *testing.T, s *llmd.Store) string {
				return writeEntity(t, s, "test:item", `{"name":"foo"}`, "")
			},
			want: true,
		},
		{
			name:  "not found",
			setup: func(t *testing.T, s *llmd.Store) string { return "nonexistent" },
			want:  false,
		},
		{
			name: "deleted",
			setup: func(t *testing.T, s *llmd.Store) string {
				key := writeEntity(t, s, "test:item", `{"name":"foo"}`, "")
				if err := s.Entities.Delete(t.Context(), key, testDeleteOpts()); err != nil {
					t.Fatalf("Delete(%s) error = %v", key, err)
				}
				return key
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			key := tt.setup(t, s)

			got, err := s.Entities.Exists(t.Context(), key)
			if err != nil {
				t.Fatalf("Exists(%q) error = %v", key, err)
			}
			if got != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", key, got, tt.want)
			}
		})
	}
}

// TestExistsInNamespace covers the namespace+relation presence check
// against a store holding one "test:item" entity on docs/readme.
func TestExistsInNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		relation  string
		want      bool
	}{
		{"same relation", "test:item", "docs/readme", true},
		{"different relation", "test:item", "docs/other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			writeEntity(t, s, "test:item", `{"x":1}`, "docs/readme")

			got, err := s.Entities.ExistsInNamespace(t.Context(), tt.namespace, tt.relation)
			if err != nil {
				t.Fatalf("ExistsInNamespace(%q, %q) error = %v", tt.namespace, tt.relation, err)
			}
			if got != tt.want {
				t.Errorf("ExistsInNamespace(%q, %q) = %v, want %v", tt.namespace, tt.relation, got, tt.want)
			}
		})
	}
}

// TestFindByValue covers JSON-path lookup against a store holding
// "important" and "draft" tagged entities on docs/readme.
func TestFindByValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string // value matched at $.tag
		want    string // Value of the found entity
		wantErr error
	}{
		{"found", "important", `{"tag":"important"}`, nil},
		{"not found", "nonexistent", "", entities.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			writeEntity(t, s, "test:item", `{"tag":"important"}`, "docs/readme")
			writeEntity(t, s, "test:item", `{"tag":"draft"}`, "docs/readme")

			got, err := s.Entities.FindByValue(t.Context(), "test:item", "docs/readme", "$.tag", tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("FindByValue($.tag, %q) error = %v, want %v", tt.value, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Value != tt.want {
				t.Errorf("FindByValue($.tag, %q) Value = %q, want %q", tt.value, got.Value, tt.want)
			}
		})
	}
}
