package tags_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/engine/documents"

	"github.com/jpl-au/llmd/engine"
	"github.com/jpl-au/llmd/engine/tags"
)

// TestAdd covers adding a tag to a document by path and by key, the
// duplicate case, and accumulating several tags on one document. Every
// case starts with a fresh store holding "docs/readme"; setup adds any
// further precondition state.
func TestAdd(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T, s *engine.Store)
		value        string // document identifier; defaults to docs/readme
		byKey        bool   // address the document by key instead of path
		tag          string
		wantErr      error
		wantExisting bool // returned tag must be the one already stored
		wantCount    int  // tags listed on docs/readme after Add
	}{
		{
			name:      "by path",
			tag:       "important",
			wantCount: 1,
		},
		{
			name:      "by key",
			byKey:     true,
			tag:       "important",
			wantCount: 1,
		},
		{
			name: "duplicate",
			setup: func(t *testing.T, s *engine.Store) {
				if _, err := s.Tags.Add(t.Context(), "docs/readme", "important", testOpts()); err != nil {
					t.Fatalf("Add(important) error = %v", err)
				}
			},
			tag:          "important",
			wantErr:      tags.ErrExists,
			wantExisting: true,
			wantCount:    1,
		},
		{
			name:    "missing document",
			value:   "docs/missing",
			tag:     "important",
			wantErr: documents.ErrNotFound,
		},
		{
			name: "multiple tags",
			setup: func(t *testing.T, s *engine.Store) {
				for _, name := range []string{"important", "v1"} {
					if _, err := s.Tags.Add(t.Context(), "docs/readme", name, testOpts()); err != nil {
						t.Fatalf("Add(%s) error = %v", name, err)
					}
				}
			},
			tag:       "needs-review",
			wantCount: 3,
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
			if tt.value != "" {
				value = tt.value
			}
			if tt.byKey {
				value = doc.Key
			}

			got, err := s.Tags.Add(ctx, value, tt.tag, testOpts())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Add(%q, %q) error = %v, want %v", value, tt.tag, err, tt.wantErr)
			}
			if got == nil {
				return // error case with no tag to inspect
			}
			if got.Value.Tag != tt.tag {
				t.Errorf("Add(%q, %q) Value.Tag = %q, want %q", value, tt.tag, got.Value.Tag, tt.tag)
			}
			if got.Relation != "docs/readme" {
				t.Errorf("Add(%q, %q) Relation = %q, want %q", value, tt.tag, got.Relation, "docs/readme")
			}
			if len(got.Key) != 10 {
				t.Errorf("Add(%q, %q) Key length = %d, want 9", value, tt.tag, len(got.Key))
			}

			list, err := s.Tags.List(ctx, "docs/readme", testOpts())
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(list) != tt.wantCount {
				t.Errorf("List() returned %d tags, want %d", len(list), tt.wantCount)
			}
			if tt.wantExisting && (len(list) != 1 || list[0].Key != got.Key) {
				t.Errorf("Add(%q, %q) Key = %q, want existing tag %v", value, tt.tag, got.Key, list)
			}
		})
	}
}
