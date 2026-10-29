package search_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/pkg/model/core"
)

// TestIndex checks that the FTS index follows the document lifecycle.
// Each case applies a sequence of document operations to a fresh store
// and then searches for content touched by that sequence. setup returns
// the key the single result is expected to carry, or "" when the key is
// not asserted. Every case then checks that the index is intact and
// holds exactly one row per live document.
func TestIndex(t *testing.T) {
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
		{
			// The index once replayed a removal for the deleted
			// version and corrupted itself.
			name: "write after delete indexes the new content",
			setup: func(t *testing.T, s *llmd.Store) string {
				if _, err := s.Documents.Write(t.Context(), "test/again", "first content", testWriteOpts()); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				if err := s.Documents.Delete(t.Context(), "test/again", ftsDeleteOpts()); err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
				doc, err := s.Documents.Write(t.Context(), "test/again", "second content", testWriteOpts())
				if err != nil {
					t.Fatalf("Write() after delete error = %v", err)
				}
				return doc.Key
			},
			query:    "second",
			want:     1,
			wantPath: "test/again",
		},
		{
			// The first version is history again, but only the
			// latest live version is indexed.
			name: "write after delete indexes only the latest version",
			setup: func(t *testing.T, s *llmd.Store) string {
				if _, err := s.Documents.Write(t.Context(), "test/again", "first content", testWriteOpts()); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				if err := s.Documents.Delete(t.Context(), "test/again", ftsDeleteOpts()); err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
				if _, err := s.Documents.Write(t.Context(), "test/again", "second content", testWriteOpts()); err != nil {
					t.Fatalf("Write() after delete error = %v", err)
				}
				// The write continued the document, so there is
				// nothing left to restore.
				if err := s.Documents.Restore(t.Context(), "test/again", ftsRestoreOpts()); !errors.Is(err, documents.ErrNotDeleted) {
					t.Fatalf("Restore() error = %v, want %v", err, documents.ErrNotDeleted)
				}
				return ""
			},
			query: "first",
			want:  0,
		},
		{
			name: "unchanged write leaves the index alone",
			setup: func(t *testing.T, s *llmd.Store) string {
				for range 2 {
					if _, err := s.Documents.Write(t.Context(), "test/same", "steady content", testWriteOpts()); err != nil {
						t.Fatalf("Write() error = %v", err)
					}
				}
				return ""
			},
			query:    "steady",
			want:     1,
			wantPath: "test/same",
		},
		{
			name: "revert indexes the reverted content",
			setup: func(t *testing.T, s *llmd.Store) string {
				if _, err := s.Documents.Write(t.Context(), "test/revert", "earlier content", testWriteOpts()); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				if _, err := s.Documents.Write(t.Context(), "test/revert", "later content", testWriteOpts()); err != nil {
					t.Fatalf("Write() update error = %v", err)
				}
				if _, err := s.History.Revert(t.Context(), "test/revert", 1, ftsRevertOpts()); err != nil {
					t.Fatalf("Revert() error = %v", err)
				}
				// A write after the revert must replace the
				// reverted version in the index.
				if _, err := s.Documents.Write(t.Context(), "test/revert", "earlier content, edited", testWriteOpts()); err != nil {
					t.Fatalf("Write() after revert error = %v", err)
				}
				return ""
			},
			query:    "earlier",
			want:     1,
			wantPath: "test/revert",
		},
		{
			name: "move then write indexes under the new path",
			setup: func(t *testing.T, s *llmd.Store) string {
				if _, err := s.Documents.Write(t.Context(), "old/path", "moveable content", testWriteOpts()); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				if err := s.Documents.Move(t.Context(), "old/path", "new/path", ftsMoveOpts()); err != nil {
					t.Fatalf("Move() error = %v", err)
				}
				if _, err := s.Documents.Write(t.Context(), "new/path", "moved and edited", testWriteOpts()); err != nil {
					t.Fatalf("Write() after move error = %v", err)
				}
				return ""
			},
			query:    "edited",
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
			checkIndex(t, s)
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

// checkIndex fails the test when the FTS index is damaged or does not
// hold exactly one row per live document. The integrity check runs
// with rank 0, which skips comparing against every row of content:
// older and deleted versions are deliberately not indexed.
func checkIndex(t *testing.T, s *llmd.Store) {
	t.Helper()
	if _, err := s.DB().Query(`INSERT INTO content_fts(content_fts, rank) VALUES ('integrity-check', 0)`).Exec(); err != nil {
		t.Fatalf("FTS integrity-check error = %v", err)
	}
	var indexed, live int
	row, err := s.DB().Query(`
		SELECT (SELECT COUNT(*) FROM content_fts_docsize),
		       (SELECT COUNT(DISTINCT path) FROM content
		        WHERE namespace = 'core:document' AND deleted_at IS NULL)
	`).ReadRow()
	if err != nil {
		t.Fatalf("counting indexed rows: %v", err)
	}
	if err := row.Scan(&indexed, &live); err != nil {
		t.Fatalf("counting indexed rows: %v", err)
	}
	if indexed != live {
		t.Errorf("index holds %d rows, want %d (one per live document)", indexed, live)
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

func ftsRevertOpts() history.RevertOptions {
	return history.RevertOptions{Origin: core.Origin{Author: "test", Source: "cli"}}
}
