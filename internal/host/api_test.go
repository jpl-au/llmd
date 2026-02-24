package host

import (
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/sdk"
)

// testHost creates an in-memory store, wires up the SDK globals via
// host.New, and returns both for direct access. The store is closed
// automatically when the test finishes.
func testHost(t *testing.T) (*Host, *llmd.Store) {
	t.Helper()
	store, err := llmd.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	h := New(store)
	return h, store
}

func TestDocumentsWriteRead(t *testing.T) {
	testHost(t)

	err := sdk.Documents.Write("notes/hello", []byte("# Hello"), "alice", "first")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	content, err := sdk.Documents.Read("notes/hello", 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(content) != "# Hello" {
		t.Errorf("Read = %q, want %q", content, "# Hello")
	}
}

func TestDocumentsReadVersion(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("v1"), "alice", "")
	sdk.Documents.Write("doc", []byte("v2"), "alice", "")

	content, err := sdk.Documents.Read("doc", 1)
	if err != nil {
		t.Fatalf("Read v1: %v", err)
	}
	if string(content) != "v1" {
		t.Errorf("Read v1 = %q, want %q", content, "v1")
	}

	content, err = sdk.Documents.Read("doc", 0)
	if err != nil {
		t.Fatalf("Read latest: %v", err)
	}
	if string(content) != "v2" {
		t.Errorf("Read latest = %q, want %q", content, "v2")
	}
}

func TestDocumentsExists(t *testing.T) {
	testHost(t)

	ok, err := sdk.Documents.Exists("nope")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if ok {
		t.Error("Exists returned true for missing document")
	}

	sdk.Documents.Write("yes", []byte("x"), "alice", "")
	ok, err = sdk.Documents.Exists("yes")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !ok {
		t.Error("Exists returned false for existing document")
	}
}

func TestDocumentsDeleteRestore(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("content"), "alice", "")

	if err := sdk.Documents.Delete("doc", "alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	ok, _ := sdk.Documents.Exists("doc")
	if ok {
		t.Error("document still exists after Delete")
	}

	if err := sdk.Documents.Restore("doc", "alice"); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	ok, _ = sdk.Documents.Exists("doc")
	if !ok {
		t.Error("document not found after Restore")
	}
}

func TestDocumentsMove(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("old", []byte("moved"), "alice", "")

	if err := sdk.Documents.Move("old", "new", "alice"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	ok, _ := sdk.Documents.Exists("old")
	if ok {
		t.Error("old path still exists after Move")
	}

	content, err := sdk.Documents.Read("new", 0)
	if err != nil {
		t.Fatalf("Read new: %v", err)
	}
	if string(content) != "moved" {
		t.Errorf("content = %q, want %q", content, "moved")
	}
}

func TestDocumentsList(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("a/one", []byte("1"), "alice", "")
	sdk.Documents.Write("a/two", []byte("2"), "alice", "")
	sdk.Documents.Write("b/three", []byte("3"), "alice", "")

	docs, err := sdk.Documents.List("a/", sdk.ListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("List returned %d docs, want 2", len(docs))
	}

	// All docs
	docs, err = sdk.Documents.List("", sdk.ListOpts{})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("List all returned %d docs, want 3", len(docs))
	}
}

func TestDocumentsListReverse(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("a", []byte("1"), "alice", "")
	sdk.Documents.Write("b", []byte("2"), "alice", "")

	docs, _ := sdk.Documents.List("", sdk.ListOpts{Reverse: true})
	if len(docs) != 2 {
		t.Fatalf("got %d docs, want 2", len(docs))
	}
	if docs[0].Path != "b" {
		t.Errorf("first doc = %q, want %q (reversed)", docs[0].Path, "b")
	}
}

func TestDocumentsListDeleted(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("keep", []byte("x"), "alice", "")
	sdk.Documents.Write("gone", []byte("x"), "alice", "")
	sdk.Documents.Delete("gone", "alice")

	docs, _ := sdk.Documents.List("", sdk.ListOpts{})
	if len(docs) != 1 {
		t.Errorf("List without deleted: got %d, want 1", len(docs))
	}

	docs, _ = sdk.Documents.List("", sdk.ListOpts{Deleted: true})
	if len(docs) != 2 {
		t.Errorf("List with deleted: got %d, want 2", len(docs))
	}
}

func TestDocumentsEdit(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("hello world"), "alice", "")

	if err := sdk.Documents.Edit("doc", "world", "Go", "alice", "fix"); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	content, _ := sdk.Documents.Read("doc", 0)
	if string(content) != "hello Go" {
		t.Errorf("content = %q, want %q", content, "hello Go")
	}
}

func TestDocumentsHistory(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("v1"), "alice", "first")
	sdk.Documents.Write("doc", []byte("v2"), "bob", "second")

	versions, err := sdk.Documents.History("doc", 0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(versions))
	}
	// Newest first
	if versions[0].Author != "bob" {
		t.Errorf("latest author = %q, want %q", versions[0].Author, "bob")
	}
	if versions[0].Message != "second" {
		t.Errorf("latest message = %q, want %q", versions[0].Message, "second")
	}
}

func TestDocumentsHistoryLimit(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("v1"), "alice", "")
	sdk.Documents.Write("doc", []byte("v2"), "alice", "")
	sdk.Documents.Write("doc", []byte("v3"), "alice", "")

	versions, _ := sdk.Documents.History("doc", 2)
	if len(versions) != 2 {
		t.Errorf("got %d versions, want 2", len(versions))
	}
}

func TestDocumentsDiff(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("line1\nline2\n"), "alice", "")
	sdk.Documents.Write("doc", []byte("line1\nchanged\n"), "alice", "")

	diff, added, removed, err := sdk.Documents.Diff("doc:1", "doc:2", 0)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if added == 0 && removed == 0 {
		t.Error("diff reported no changes")
	}
	if diff == "" {
		t.Error("diff text is empty")
	}
}

func TestDocumentsRevert(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("original"), "alice", "")
	sdk.Documents.Write("doc", []byte("changed"), "alice", "")

	if err := sdk.Documents.Revert("doc", 1, "alice", "revert"); err != nil {
		t.Fatalf("Revert: %v", err)
	}

	content, _ := sdk.Documents.Read("doc", 0)
	if string(content) != "original" {
		t.Errorf("content = %q, want %q", content, "original")
	}
}

func TestDocumentsGlob(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("notes/a.md", []byte("x"), "alice", "")
	sdk.Documents.Write("notes/b.md", []byte("x"), "alice", "")
	sdk.Documents.Write("other/c.md", []byte("x"), "alice", "")

	paths, err := sdk.Documents.Glob("notes/*")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("Glob returned %d paths, want 2", len(paths))
	}
}

func TestDocumentsGrep(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc1", []byte("the quick brown fox"), "alice", "")
	sdk.Documents.Write("doc2", []byte("lazy dog"), "alice", "")

	hits, err := sdk.Documents.Grep("quick", sdk.GrepOpts{})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(hits) == 0 {
		t.Error("Grep returned no hits")
	}
	if hits[0].Path != "doc1" {
		t.Errorf("hit path = %q, want %q", hits[0].Path, "doc1")
	}
}

func TestDocumentsGrepPathFilter(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("a/doc", []byte("needle"), "alice", "")
	sdk.Documents.Write("b/doc", []byte("needle"), "alice", "")

	hits, _ := sdk.Documents.Grep("needle", sdk.GrepOpts{Path: "a/"})
	if len(hits) != 1 {
		t.Errorf("got %d hits, want 1 (path-filtered)", len(hits))
	}
}

func TestDocumentsVacuum(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("x"), "alice", "")
	sdk.Documents.Delete("doc", "alice")

	result, err := sdk.Documents.Vacuum()
	if err != nil {
		t.Fatalf("Vacuum: %v", err)
	}
	if result.Documents == 0 {
		t.Error("Vacuum reported 0 documents cleaned")
	}
}
