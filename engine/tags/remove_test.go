package tags_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/tags"
)

// TestRemove covers removing a tag by document path and by key, and the
// missing-tag error. Every case starts with a fresh store holding
// "docs/readme"; after Remove the document must list no tags.
func TestRemove(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, s *engine.Store)
		byKey   bool // address the document by key instead of path
		tag     string
		wantErr error
	}{
		{
			name: "by path",
			setup: func(t *testing.T, s *engine.Store) {
				if _, err := s.Tags.Add(t.Context(), "docs/readme", "important", testOpts()); err != nil {
					t.Fatalf("Add() error = %v", err)
				}
			},
			tag: "important",
		},
		{
			name:    "not found",
			tag:     "nonexistent",
			wantErr: tags.ErrNotFound,
		},
		{
			name: "by key",
			setup: func(t *testing.T, s *engine.Store) {
				if _, err := s.Tags.Add(t.Context(), "docs/readme", "important", testOpts()); err != nil {
					t.Fatalf("Add() error = %v", err)
				}
			},
			byKey: true,
			tag:   "important",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()

			doc, err := s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if tt.setup != nil {
				tt.setup(t, s)
			}

			value := "docs/readme"
			if tt.byKey {
				value = doc.Key
			}

			err = s.Tags.Remove(ctx, value, tt.tag, testOpts())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Remove(%q, %q) error = %v, want %v", value, tt.tag, err, tt.wantErr)
			}

			list, err := s.Tags.List(ctx, "docs/readme", testOpts())
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(list) != 0 {
				t.Errorf("List() after Remove(%q, %q) returned %d tags, want 0", value, tt.tag, len(list))
			}
		})
	}
}
