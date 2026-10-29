package tags_test

import (
	"testing"

	"github.com/jpl-au/llmd/engine"
)

// TestExists covers tag presence checks by path and by key, including a
// tag that was never added and one that was removed. Every case starts
// with a fresh store holding "docs/test".
func TestExists(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, s *engine.Store)
		byKey bool // address the document by key instead of path
		tag   string
		want  bool
	}{
		{
			name: "found",
			setup: func(t *testing.T, s *engine.Store) {
				if _, err := s.Tags.Add(t.Context(), "docs/test", "important", testOpts()); err != nil {
					t.Fatalf("Add() error = %v", err)
				}
			},
			tag:  "important",
			want: true,
		},
		{
			name: "not found",
			tag:  "nonexistent",
			want: false,
		},
		{
			name: "by key",
			setup: func(t *testing.T, s *engine.Store) {
				if _, err := s.Tags.Add(t.Context(), "docs/test", "important", testOpts()); err != nil {
					t.Fatalf("Add() error = %v", err)
				}
			},
			byKey: true,
			tag:   "important",
			want:  true,
		},
		{
			name: "removed tag",
			setup: func(t *testing.T, s *engine.Store) {
				if _, err := s.Tags.Add(t.Context(), "docs/test", "important", testOpts()); err != nil {
					t.Fatalf("Add() error = %v", err)
				}
				if err := s.Tags.Remove(t.Context(), "docs/test", "important", testOpts()); err != nil {
					t.Fatalf("Remove() error = %v", err)
				}
			},
			tag:  "important",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()

			doc, err := s.Documents.Write(ctx, "docs/test", "content", testWriteOpts())
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if tt.setup != nil {
				tt.setup(t, s)
			}

			value := "docs/test"
			if tt.byKey {
				value = doc.Key
			}

			got, err := s.Tags.Exists(ctx, value, tt.tag)
			if err != nil {
				t.Fatalf("Exists(%q, %q) error = %v", value, tt.tag, err)
			}
			if got != tt.want {
				t.Errorf("Exists(%q, %q) = %v, want %v", value, tt.tag, got, tt.want)
			}
		})
	}
}
