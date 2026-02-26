package host

import (
	"os"
	"path/filepath"
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
	h := setup(store)
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

func TestDocumentsVacuumEmpty(t *testing.T) {
	testHost(t)

	result, err := sdk.Documents.Vacuum()
	if err != nil {
		t.Fatalf("Vacuum: %v", err)
	}
	if result.Documents != 0 {
		t.Errorf("Vacuum on empty store: documents = %d, want 0", result.Documents)
	}
}

func TestDocumentsReadNotFound(t *testing.T) {
	testHost(t)

	_, err := sdk.Documents.Read("nonexistent", 0)
	if err == nil {
		t.Error("Read returned nil error for missing document")
	}
}

func TestDocumentsDeleteNotFound(t *testing.T) {
	testHost(t)

	err := sdk.Documents.Delete("nonexistent", "alice")
	if err == nil {
		t.Error("Delete returned nil error for missing document")
	}
}

func TestDocumentsMoveNotFound(t *testing.T) {
	testHost(t)

	err := sdk.Documents.Move("nonexistent", "dest", "alice")
	if err == nil {
		t.Error("Move returned nil error for missing source")
	}
}

func TestDocumentsEditNotFound(t *testing.T) {
	testHost(t)

	err := sdk.Documents.Edit("nonexistent", "old", "new", "alice", "")
	if err == nil {
		t.Error("Edit returned nil error for missing document")
	}
}

func TestDocumentsEditPatternNotFound(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("hello world"), "alice", "")

	err := sdk.Documents.Edit("doc", "xyz", "abc", "alice", "")
	if err == nil {
		t.Error("Edit returned nil error when pattern not found")
	}
}

func TestDocumentsWriteEmpty(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("empty", []byte(""), "alice", ""); err != nil {
		t.Fatalf("Write empty: %v", err)
	}

	content, err := sdk.Documents.Read("empty", 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(content) != 0 {
		t.Errorf("content = %q, want empty", content)
	}
}

func TestDocumentsWriteOverwrite(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("first"), "alice", "")
	sdk.Documents.Write("doc", []byte("second"), "alice", "")

	content, _ := sdk.Documents.Read("doc", 0)
	if string(content) != "second" {
		t.Errorf("content = %q, want %q", content, "second")
	}

	// Should have 2 versions
	versions, _ := sdk.Documents.History("doc", 0)
	if len(versions) != 2 {
		t.Errorf("versions = %d, want 2", len(versions))
	}
}

func TestDocumentsGlobNoMatch(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("notes/a.md", []byte("x"), "alice", "")

	paths, err := sdk.Documents.Glob("other/*")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("Glob returned %d paths, want 0", len(paths))
	}
}

func TestDocumentsGrepNoMatch(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("hello world"), "alice", "")

	hits, err := sdk.Documents.Grep("nonexistent", sdk.GrepOpts{})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("Grep returned %d hits, want 0", len(hits))
	}
}

func TestDocumentsGrepLines(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("line one\nline two\nline three"), "alice", "")

	hits, err := sdk.Documents.Grep("two", sdk.GrepOpts{Mode: sdk.GrepLines})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("GrepLines returned no hits")
	}
	if hits[0].Line == 0 {
		t.Error("GrepLines hit has no line number")
	}
}

func TestDocumentsGrepLinesContext(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("aaa\nbbb\nccc\nddd\neee"), "alice", "")

	hits, err := sdk.Documents.Grep("ccc", sdk.GrepOpts{Mode: sdk.GrepLines, Context: 1})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("GrepLines with context returned no hits")
	}
	if len(hits[0].Before) == 0 {
		t.Error("expected Before context lines")
	}
	if len(hits[0].After) == 0 {
		t.Error("expected After context lines")
	}
}

func TestDocumentsGrepSections(t *testing.T) {
	testHost(t)

	content := "# Intro\n\nNothing here.\n\n# Details\n\nThe needle is here.\n"
	sdk.Documents.Write("doc", []byte(content), "alice", "")

	hits, err := sdk.Documents.Grep("needle", sdk.GrepOpts{Mode: sdk.GrepSections})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("GrepSections returned no hits")
	}
	if hits[0].Section == "" {
		t.Error("GrepSections hit has no section heading")
	}
}

func TestDocumentsDiffSameVersion(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("doc", []byte("same"), "alice", "")

	_, added, removed, err := sdk.Documents.Diff("doc:1", "doc:1", 0)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if added != 0 || removed != 0 {
		t.Errorf("same-version diff: added=%d removed=%d, want 0/0", added, removed)
	}
}

func TestDocumentsListEmpty(t *testing.T) {
	testHost(t)

	docs, err := sdk.Documents.List("", sdk.ListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("List on empty store: got %d, want 0", len(docs))
	}
}

func TestDocumentsImportExport(t *testing.T) {
	testHost(t)

	// Create a temp directory with some files to import
	importDir := t.TempDir()
	os.WriteFile(filepath.Join(importDir, "one.md"), []byte("first"), 0644)
	os.WriteFile(filepath.Join(importDir, "two.md"), []byte("second"), 0644)

	result, err := sdk.Documents.Import(importDir, sdk.ImportOpts{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(result.Created) != 2 {
		t.Errorf("Import created %d, want 2", len(result.Created))
	}

	// Import strips the .md extension, so documents are stored as "one" and "two"
	content, err := sdk.Documents.Read("one", 0)
	if err != nil {
		t.Fatalf("Read imported: %v", err)
	}
	if string(content) != "first" {
		t.Errorf("imported content = %q, want %q", content, "first")
	}
}

func TestDocumentsImportDryRun(t *testing.T) {
	testHost(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "doc.md"), []byte("content"), 0644)

	result, err := sdk.Documents.Import(dir, sdk.ImportOpts{DryRun: true})
	if err != nil {
		t.Fatalf("Import dry run: %v", err)
	}
	if len(result.Created) != 1 {
		t.Errorf("dry run created %d, want 1", len(result.Created))
	}

	// Document should NOT actually exist
	ok, _ := sdk.Documents.Exists("doc.md")
	if ok {
		t.Error("dry run imported the document for real")
	}
}

func TestDocumentsImportWithPrefix(t *testing.T) {
	testHost(t)

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "note.md"), []byte("x"), 0644)

	result, err := sdk.Documents.Import(dir, sdk.ImportOpts{Prefix: "imported/"})
	if err != nil {
		t.Fatalf("Import with prefix: %v", err)
	}
	if len(result.Created) != 1 {
		t.Errorf("created %d, want 1", len(result.Created))
	}

	// Import strips .md extension, so stored as "imported/note"
	ok, _ := sdk.Documents.Exists("imported/note")
	if !ok {
		t.Error("document not found at prefixed path")
	}
}

func TestDocumentsExport(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("notes/one", []byte("first"), "alice", "")
	sdk.Documents.Write("notes/two", []byte("second"), "alice", "")

	dir := t.TempDir()
	// Export uses prefix ending in "/" for multi-doc export,
	// and appends .md to each exported file
	result, err := sdk.Documents.Export("notes/", dir, sdk.ExportOpts{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(result.Exported) != 2 {
		t.Errorf("exported %d, want 2", len(result.Exported))
	}

	data, err := os.ReadFile(filepath.Join(dir, "one.md"))
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	if string(data) != "first" {
		t.Errorf("exported content = %q, want %q", data, "first")
	}
}

func TestDocumentsExportSkipsExisting(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("exp/doc", []byte("store content"), "alice", "")

	dir := t.TempDir()
	// Export appends .md, so pre-create doc.md to trigger skip
	os.WriteFile(filepath.Join(dir, "doc.md"), []byte("existing"), 0644)

	result, err := sdk.Documents.Export("exp/", dir, sdk.ExportOpts{Overwrite: false})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("skipped %d, want 1", len(result.Skipped))
	}

	// Original file should be untouched
	data, _ := os.ReadFile(filepath.Join(dir, "doc.md"))
	if string(data) != "existing" {
		t.Errorf("file was overwritten: %q", data)
	}
}

func TestDocumentsExportOverwrite(t *testing.T) {
	testHost(t)

	sdk.Documents.Write("exp/doc", []byte("new content"), "alice", "")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "doc.md"), []byte("old"), 0644)

	result, err := sdk.Documents.Export("exp/", dir, sdk.ExportOpts{Overwrite: true})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(result.Exported) != 1 {
		t.Errorf("exported %d, want 1", len(result.Exported))
	}

	data, _ := os.ReadFile(filepath.Join(dir, "doc.md"))
	if string(data) != "new content" {
		t.Errorf("file not overwritten: %q", data)
	}
}
