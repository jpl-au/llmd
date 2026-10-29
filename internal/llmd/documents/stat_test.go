package documents_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestStat(t *testing.T) {
	tests := []struct {
		name string
		// setup prepares the store and returns the value to stat and
		// the key the result must carry.
		setup       func(t *testing.T, s *llmd.Store) (value, key string)
		wantErr     error
		wantStat    bool // a Stat is returned, even alongside ErrDeleted
		wantPath    string
		wantVersion int
		wantLines   int // Meta.Lines; 0 skips the Meta check
	}{
		{
			name: "by path",
			setup: func(t *testing.T, s *llmd.Store) (string, string) {
				doc := mustWrite(t, s, "docs/readme", "# Hello World\n\nThis is content.")
				return "docs/readme", doc.Key
			},
			wantStat:    true,
			wantPath:    "docs/readme",
			wantVersion: 1,
			wantLines:   3,
		},
		{
			name: "by key",
			setup: func(t *testing.T, s *llmd.Store) (string, string) {
				doc := mustWrite(t, s, "docs/readme", "content")
				return doc.Key, doc.Key
			},
			wantStat:    true,
			wantPath:    "docs/readme",
			wantVersion: 1,
			wantLines:   1,
		},
		{
			name:    "not found",
			setup:   func(t *testing.T, s *llmd.Store) (string, string) { return "nonexistent", "" },
			wantErr: documents.ErrNotFound,
		},
		{
			// Stat by key still returns the full record, including
			// Meta, for a soft-deleted document, alongside ErrDeleted.
			name: "deleted by key",
			setup: func(t *testing.T, s *llmd.Store) (string, string) {
				doc := mustWrite(t, s, "docs/deleted", "content")
				mustDelete(t, s, "docs/deleted")
				return doc.Key, doc.Key
			},
			wantErr:     documents.ErrDeleted,
			wantStat:    true,
			wantPath:    "docs/deleted",
			wantVersion: 1,
			wantLines:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			value, key := tt.setup(t, s)

			stat, err := s.Documents.Stat(t.Context(), value)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Stat(%q) error = %v, want %v", value, err, tt.wantErr)
			}
			if (stat != nil) != tt.wantStat {
				t.Fatalf("Stat(%q) = %v, wantStat %v", value, stat, tt.wantStat)
			}
			if !tt.wantStat {
				return
			}

			if stat.Path != tt.wantPath {
				t.Errorf("Stat(%q).Path = %q, want %q", value, stat.Path, tt.wantPath)
			}
			if stat.Key != key {
				t.Errorf("Stat(%q).Key = %q, want %q", value, stat.Key, key)
			}
			if stat.Version != tt.wantVersion {
				t.Errorf("Stat(%q).Version = %d, want %d", value, stat.Version, tt.wantVersion)
			}
			if tt.wantLines == 0 {
				return
			}
			if stat.Meta == nil {
				t.Errorf("Stat(%q).Meta = nil, want non-nil", value)
			} else if stat.Meta.Lines != tt.wantLines {
				t.Errorf("Stat(%q).Meta.Lines = %d, want %d", value, stat.Meta.Lines, tt.wantLines)
			}
		})
	}
}
