package documents_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestPurge(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, s *llmd.Store)
		want     []string          // paths reported purged
		wantKept map[string]string // path -> content still readable after the purge
		wantGone []string          // paths that must read as ErrNotFound after the purge
	}{
		{
			name: "removes soft-deleted documents",
			setup: func(t *testing.T, s *llmd.Store) {
				mustWrite(t, s, "docs/a", "content a")
				mustWrite(t, s, "docs/b", "content b")
				mustWrite(t, s, "docs/c", "content c")
				mustDelete(t, s, "docs/a")
				mustDelete(t, s, "docs/b")
			},
			want:     []string{"docs/a", "docs/b"},
			wantKept: map[string]string{"docs/c": "content c"},
			wantGone: []string{"docs/a", "docs/b"},
		},
		{
			name: "nothing to purge",
			want: nil,
		},
		{
			// Delete soft-deletes every version, so Purge must remove
			// all of them.
			name: "removes all versions",
			setup: func(t *testing.T, s *llmd.Store) {
				mustWrite(t, s, "docs/versioned", "v1")
				mustWrite(t, s, "docs/versioned", "v2")
				mustWrite(t, s, "docs/versioned", "v3")
				mustDelete(t, s, "docs/versioned")
			},
			want:     []string{"docs/versioned"},
			wantGone: []string{"docs/versioned"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			if tt.setup != nil {
				tt.setup(t, s)
			}

			got, err := s.Documents.Purge(ctx, documents.PurgeOptions{})
			if err != nil {
				t.Fatalf("Purge() error = %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Purge() = %v, want %v", got, tt.want)
			}

			for path, content := range tt.wantKept {
				doc, err := s.Documents.Read(ctx, path)
				if err != nil {
					t.Errorf("Read(%q) after Purge error = %v", path, err)
					continue
				}
				if doc.Content != content {
					t.Errorf("Read(%q) after Purge: Content = %q, want %q", path, doc.Content, content)
				}
			}
			for _, path := range tt.wantGone {
				if _, err := s.Documents.Read(ctx, path); !errors.Is(err, documents.ErrNotFound) {
					t.Errorf("Read(%q) after Purge error = %v, want %v", path, err, documents.ErrNotFound)
				}
			}
		})
	}
}
