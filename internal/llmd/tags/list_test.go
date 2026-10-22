package tags_test

import (
	"slices"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/pkg/model/tag"
)

// TestList covers listing the tags on one document by path and by key,
// and a document with no tags. Every case starts with a fresh store
// holding "docs/readme".
func TestList(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, s *llmd.Store)
		byKey bool // address the document by key instead of path
		want  int  // number of tags listed
	}{
		{
			name: "two tags",
			setup: func(t *testing.T, s *llmd.Store) {
				for _, name := range []string{"important", "v1"} {
					if _, err := s.Tags.Add(t.Context(), "docs/readme", name, testOpts()); err != nil {
						t.Fatalf("Add(%s) error = %v", name, err)
					}
				}
			},
			want: 2,
		},
		{
			name: "by key",
			setup: func(t *testing.T, s *llmd.Store) {
				if _, err := s.Tags.Add(t.Context(), "docs/readme", "important", testOpts()); err != nil {
					t.Fatalf("Add() error = %v", err)
				}
			},
			byKey: true,
			want:  1,
		},
		{
			name: "empty",
			want: 0,
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

			got, err := s.Tags.List(ctx, value, testOpts())
			if err != nil {
				t.Fatalf("List(%q) error = %v", value, err)
			}
			if len(got) != tt.want {
				t.Errorf("List(%q) returned %d tags, want %d", value, len(got), tt.want)
			}
		})
	}
}

// TestListAll covers the store-wide tag summary: unique names with the
// number of documents carrying each, and the empty store. ListAll orders
// by name, so want is compared exactly.
func TestListAll(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, s *llmd.Store)
		want  []tag.Info
	}{
		{
			name: "unique names with counts",
			setup: func(t *testing.T, s *llmd.Store) {
				writeTagged(t, s, "docs/readme", "important", "v1")
				writeTagged(t, s, "docs/api", "important", "needs-review") // "important" repeated
			},
			want: []tag.Info{
				{Name: "important", Count: 2},
				{Name: "needs-review", Count: 1},
				{Name: "v1", Count: 1},
			},
		},
		{
			name: "empty",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			if tt.setup != nil {
				tt.setup(t, s)
			}

			got, err := s.Tags.ListAll(t.Context())
			if err != nil {
				t.Fatalf("ListAll() error = %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("ListAll() = %v, want %v", got, tt.want)
			}
		})
	}
}
