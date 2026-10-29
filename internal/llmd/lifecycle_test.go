package llmd_test

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/internal/llmd/llmdtest"
	"github.com/jpl-au/llmd/internal/llmd/tags"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/pkg/model/document"
)

var lifeOrigin = core.Origin{Author: "test", Source: "cli"}

// outcome names the result of an operation the way the rule is
// written: ok, or which refusal.
func outcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, documents.ErrDeleted):
		return "deleted"
	case errors.Is(err, documents.ErrNotDeleted):
		return "not deleted"
	case errors.Is(err, documents.ErrExists):
		return "exists"
	case errors.Is(err, documents.ErrNotFound), errors.Is(err, history.ErrNotFound):
		return "not found"
	default:
		return err.Error()
	}
}

// TestDocumentStates holds every document operation to one rule: a
// document is live, deleted or absent as a whole, and each operation
// treats each state the same way whether it is reached by path or by
// key. "p" is set up in the named state with two versions; "o" is a
// live document to move, copy and link with. After every operation no
// path may be left part live, part deleted, and the search index must
// hold one row per live document.
func TestDocumentStates(t *testing.T) {
	v1 := 1
	ops := []struct {
		name string
		run  func(t *testing.T, s *llmd.Store) error
		// want per state: live, deleted, absent
		live, deleted, absent string
	}{
		{"read", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Documents.Read(t.Context(), "p")
			return err
		}, "ok", "deleted", "not found"},
		{"read version", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Documents.Read(t.Context(), "p", documents.ReadOptions{Version: &v1})
			return err
		}, "ok", "deleted", "not found"},
		{"stat", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Documents.Stat(t.Context(), "p")
			return err
		}, "ok", "deleted", "not found"},
		{"history", func(t *testing.T, s *llmd.Store) error {
			_, err := s.History.List(t.Context(), "p")
			return err
		}, "ok", "deleted", "not found"},
		{"edit", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Documents.Edit(t.Context(), "p", "two", "2", documents.EditOptions{Origin: lifeOrigin})
			return err
		}, "ok", "deleted", "not found"},
		{"revert", func(t *testing.T, s *llmd.Store) error {
			_, err := s.History.Revert(t.Context(), "p", 1, history.RevertOptions{Origin: lifeOrigin})
			return err
		}, "ok", "deleted", "not found"},
		{"delete", func(t *testing.T, s *llmd.Store) error {
			return s.Documents.Delete(t.Context(), "p", documents.DeleteOptions{Origin: lifeOrigin})
		}, "ok", "deleted", "not found"},
		{"restore", func(t *testing.T, s *llmd.Store) error {
			return s.Documents.Restore(t.Context(), "p", documents.RestoreOptions{Origin: lifeOrigin})
		}, "not deleted", "ok", "not found"},
		{"move from", func(t *testing.T, s *llmd.Store) error {
			return s.Documents.Move(t.Context(), "p", "moved", documents.MoveOptions{Origin: lifeOrigin})
		}, "ok", "deleted", "not found"},
		{"move onto", func(t *testing.T, s *llmd.Store) error {
			return s.Documents.Move(t.Context(), "o", "p", documents.MoveOptions{Origin: lifeOrigin})
		}, "exists", "exists", "ok"},
		{"copy from", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Documents.Copy(t.Context(), "p", "copied", documents.CopyOptions{Origin: lifeOrigin})
			return err
		}, "ok", "deleted", "not found"},
		{"copy onto", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Documents.Copy(t.Context(), "o", "p", documents.CopyOptions{Origin: lifeOrigin})
			return err
		}, "exists", "ok", "ok"},
		{"write", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Documents.Write(t.Context(), "p", "three", documents.WriteOptions{Origin: lifeOrigin})
			return err
		}, "ok", "ok", "ok"},
		{"tag", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Tags.Add(t.Context(), "p", "red", tags.Options{Origin: lifeOrigin})
			return err
		}, "ok", "deleted", "not found"},
		{"link from", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Links.Add(t.Context(), "p", "o", links.Options{Origin: lifeOrigin})
			return err
		}, "ok", "deleted", "not found"},
		{"link to", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Links.Add(t.Context(), "o", "p", links.Options{Origin: lifeOrigin})
			return err
		}, "ok", "deleted", "not found"},
		{"task spec", func(t *testing.T, s *llmd.Store) error {
			_, err := s.Tasks.Add(t.Context(), "spec", nil, tasks.AddOptions{Origin: lifeOrigin, Path: "p"})
			return err
		}, "ok", "deleted", "not found"},
	}

	for _, op := range ops {
		for _, st := range []struct{ state, want string }{
			{"live", op.live}, {"deleted", op.deleted}, {"absent", op.absent},
		} {
			t.Run(op.name+"/"+st.state, func(t *testing.T) {
				s := llmdtest.Open(t)
				write(t, s, "o", "other")
				if st.state != "absent" {
					write(t, s, "p", "one")
					write(t, s, "p", "two")
				}
				if st.state == "deleted" {
					if err := s.Documents.Delete(t.Context(), "p", documents.DeleteOptions{Origin: lifeOrigin}); err != nil {
						t.Fatalf("Delete(p) error = %v", err)
					}
				}

				if got := outcome(op.run(t, s)); got != st.want {
					t.Errorf("%s on a %s document = %s, want %s", op.name, st.state, got, st.want)
				}
				checkWhole(t, s)
			})
		}
	}
}

// TestDeletedDocumentByKey checks that a deleted document's key reaches
// it as its path does: restore works by key, and reads say deleted
// rather than not found.
func TestDeletedDocumentByKey(t *testing.T) {
	s := llmdtest.Open(t)
	doc := write(t, s, "p", "one")
	if err := s.Documents.Delete(t.Context(), "p", documents.DeleteOptions{Origin: lifeOrigin}); err != nil {
		t.Fatalf("Delete(p) error = %v", err)
	}

	path, err := s.Documents.KeyToPath(t.Context(), doc.Key)
	if err != nil || path != "p" {
		t.Fatalf("KeyToPath(%s) = %q, %v, want %q, nil", doc.Key, path, err, "p")
	}
	if _, err := s.Documents.Stat(t.Context(), doc.Key); !errors.Is(err, documents.ErrDeleted) {
		t.Errorf("Stat(%s) error = %v, want %v", doc.Key, err, documents.ErrDeleted)
	}
}

// TestWriteContinuesDeletedDocument is the sequence that once left
// hidden history: write, delete, write again. The second write
// continues the document, so nothing is hidden, restore has nothing to
// do and vacuum has nothing to purge.
func TestWriteContinuesDeletedDocument(t *testing.T) {
	s := llmdtest.Open(t)
	ctx := t.Context()
	write(t, s, "p", "one")
	if err := s.Documents.Delete(ctx, "p", documents.DeleteOptions{Origin: lifeOrigin}); err != nil {
		t.Fatalf("Delete(p) error = %v", err)
	}
	doc := write(t, s, "p", "two")
	if doc.Version != 2 {
		t.Errorf("Write() after delete version = %d, want 2", doc.Version)
	}

	versions, err := s.History.List(ctx, "p")
	if err != nil || len(versions) != 2 {
		t.Fatalf("History.List(p) = %d versions, %v, want 2, nil", len(versions), err)
	}
	purged, err := s.Documents.Purge(ctx, documents.PurgeOptions{DryRun: true})
	if err != nil || len(purged) != 0 {
		t.Errorf("Purge(dry run) = %v, %v, want nothing to purge", purged, err)
	}
}

// TestWriteRestoresUnchangedDocument checks that writing a deleted
// document's last content back brings it back without a new version.
func TestWriteRestoresUnchangedDocument(t *testing.T) {
	s := llmdtest.Open(t)
	write(t, s, "p", "one")
	if err := s.Documents.Delete(t.Context(), "p", documents.DeleteOptions{Origin: lifeOrigin}); err != nil {
		t.Fatalf("Delete(p) error = %v", err)
	}
	if doc := write(t, s, "p", "one"); doc.Version != 1 {
		t.Errorf("Write() of the deleted content version = %d, want 1", doc.Version)
	}
	if err := s.Documents.Check(t.Context(), "p"); err != nil {
		t.Errorf("Check(p) after writing it back = %v, want live", err)
	}
	checkWhole(t, s)
}

// TestTaskPathSkipsDeletedDocument checks that a new task does not take
// over a deleted document's path for its spec: the spec would continue
// that unrelated document.
func TestTaskPathSkipsDeletedDocument(t *testing.T) {
	s := llmdtest.Open(t)
	write(t, s, "tasks/fix-it", "old notes")
	if err := s.Documents.Delete(t.Context(), "tasks/fix-it", documents.DeleteOptions{Origin: lifeOrigin}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	tsk, err := s.Tasks.Add(t.Context(), "Fix it", []byte("# Fix it\n\nSpec."), tasks.AddOptions{Origin: lifeOrigin})
	if err != nil {
		t.Fatalf("Tasks.Add() error = %v", err)
	}
	if tsk.Path != "tasks/fix-it-2" {
		t.Errorf("Tasks.Add() path = %q, want %q", tsk.Path, "tasks/fix-it-2")
	}
}

func write(t *testing.T, s *llmd.Store, path, content string) *document.Document {
	t.Helper()
	doc, err := s.Documents.Write(t.Context(), path, content, documents.WriteOptions{Origin: lifeOrigin})
	if err != nil {
		t.Fatalf("Write(%s) error = %v", path, err)
	}
	return doc
}

// checkWhole fails the test when a path is left part live, part
// deleted, or the search index is damaged or out of step.
func checkWhole(t *testing.T, s *llmd.Store) {
	t.Helper()
	var mixed, indexed, live int
	row, err := s.DB().Query(`
		SELECT
		  (SELECT COUNT(*) FROM (
		     SELECT path FROM content WHERE namespace = 'core:document'
		     GROUP BY path HAVING COUNT(deleted_at) NOT IN (0, COUNT(*)))),
		  (SELECT COUNT(*) FROM content_fts_docsize),
		  (SELECT COUNT(DISTINCT path) FROM content
		   WHERE namespace = 'core:document' AND deleted_at IS NULL)
	`).ReadRow()
	if err != nil {
		t.Fatalf("checking document states: %v", err)
	}
	if err := row.Scan(&mixed, &indexed, &live); err != nil {
		t.Fatalf("checking document states: %v", err)
	}
	if mixed != 0 {
		t.Errorf("%d paths are part live, part deleted", mixed)
	}
	if indexed != live {
		t.Errorf("search index holds %d rows, want %d (one per live document)", indexed, live)
	}
	if _, err := s.DB().Query(`INSERT INTO content_fts(content_fts, rank) VALUES ('integrity-check', 0)`).Exec(); err != nil {
		t.Errorf("search index integrity-check error = %v", err)
	}
}
