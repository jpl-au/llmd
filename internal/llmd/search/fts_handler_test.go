package search_test

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

// TestFTSHandler checks that the FTS index follows the document
// lifecycle. Each case applies a sequence of document operations to a
// fresh store and then searches for content touched by that sequence.
// setup returns the key the single result is expected to carry, or ""
// when the key is not asserted.
func TestFTSHandler(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, s *llmd.Store) string
		query    string
		want     int    // number of results
		wantPath string // path of the single result, checked when want == 1
	}{
		{
			name: "write indexes content",
			setup: func(t *testing.T, s *llmd.Store) string {
				doc, err := s.Documents.Write(t.Context(), "test/fts", "hello world", testWriteOpts())
				if err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				return doc.Key
			},
			query:    "hello",
			want:     1,
			wantPath: "test/fts",
		},
		{
			name: "update drops original content",
			setup: func(t *testing.T, s *llmd.Store) string {
				if _, err := s.Documents.Write(t.Context(), "test/update", "original content", testWriteOpts()); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				if _, err := s.Documents.Write(t.Context(), "test/update", "updated content", testWriteOpts()); err != nil {
					t.Fatalf("Write() update error = %v", err)
				}
				return ""
			},
			query: "original",
			want:  0,
		},
		{
			name: "update indexes new content",
			setup: func(t *testing.T, s *llmd.Store) string {
				if _, err := s.Documents.Write(t.Context(), "test/update", "original content", testWriteOpts()); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				doc, err := s.Documents.Write(t.Context(), "test/update", "updated content", testWriteOpts())
				if err != nil {
					t.Fatalf("Write() update error = %v", err)
				}
				return doc.Key
			},
			query:    "updated",
			want:     1,
			wantPath: "test/update",
		},
		{
			name: "delete removes from index",
			setup: func(t *testing.T, s *llmd.Store) string {
				if _, err := s.Documents.Write(t.Context(), "test/delete", "searchable content", testWriteOpts()); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				if err := s.Documents.Delete(t.Context(), "test/delete", ftsDeleteOpts()); err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
				return ""
			},
			query: "searchable",
			want:  0,
		},
		{
			name: "restore returns to index",
			setup: func(t *testing.T, s *llmd.Store) string {
				if _, err := s.Documents.Write(t.Context(), "test/restore", "restorable content", testWriteOpts()); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				if err := s.Documents.Delete(t.Context(), "test/restore", ftsDeleteOpts()); err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
				if err := s.Documents.Restore(t.Context(), "test/restore", ftsRestoreOpts()); err != nil {
					t.Fatalf("Restore() error = %v", err)
				}
				return ""
			},
			query:    "restorable",
			want:     1,
			wantPath: "test/restore",
		},
		{
			name: "move keeps content under new path",
			setup: func(t *testing.T, s *llmd.Store) string {
				if _, err := s.Documents.Write(t.Context(), "old/path", "moveable content", testWriteOpts()); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				if err := s.Documents.Move(t.Context(), "old/path", "new/path", ftsMoveOpts()); err != nil {
					t.Fatalf("Move() error = %v", err)
				}
				return ""
			},
			query:    "moveable",
			want:     1,
			wantPath: "new/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			wantKey := tt.setup(t, s)

			results, err := s.Search.FullText(t.Context(), tt.query)
			if err != nil {
				t.Fatalf("FullText(%q) error = %v", tt.query, err)
			}
			if len(results) != tt.want {
				t.Fatalf("FullText(%q) returned %d results, want %d", tt.query, len(results), tt.want)
			}
			if tt.want == 0 {
				return
			}
			if results[0].Path != tt.wantPath {
				t.Errorf("FullText(%q) Path = %q, want %q", tt.query, results[0].Path, tt.wantPath)
			}
			if wantKey != "" && results[0].Key != wantKey {
				t.Errorf("FullText(%q) Key = %q, want %q", tt.query, results[0].Key, wantKey)
			}
		})
	}
}

// Test helpers
func ftsDeleteOpts() documents.DeleteOptions {
	return documents.DeleteOptions{Author: "test", Source: "cli"}
}

func ftsRestoreOpts() documents.RestoreOptions {
	return documents.RestoreOptions{Author: "test", Source: "cli"}
}

func ftsMoveOpts() documents.MoveOptions {
	return documents.MoveOptions{Author: "test", Source: "cli"}
}
