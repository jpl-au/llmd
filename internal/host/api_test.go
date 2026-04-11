package host

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

func testHost(t *testing.T) {
	t.Helper()
	TestSetup(t, TestMemory)
}

func TestDocumentsWriteRead(t *testing.T) {
	testHost(t)

	err := sdk.Documents.Write("notes/hello", []byte("# Hello"), sdk.WriteOpts{Author: "alice", Message: "first"})
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

	if err := sdk.Documents.Write("doc", []byte("v1"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write v1: %v", err)
	}
	if err := sdk.Documents.Write("doc", []byte("v2"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write v2: %v", err)
	}

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

	if err := sdk.Documents.Write("yes", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}
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

	if err := sdk.Documents.Write("doc", []byte("content"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

	if err := sdk.Documents.Delete("doc", sdk.DeleteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	ok, _ := sdk.Documents.Exists("doc")
	if ok {
		t.Error("document still exists after Delete")
	}

	if err := sdk.Documents.Restore("doc", sdk.RestoreOpts{Author: "alice"}); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	ok, _ = sdk.Documents.Exists("doc")
	if !ok {
		t.Error("document not found after Restore")
	}
}

func TestDocumentsMove(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("old", []byte("moved"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

	if err := sdk.Documents.Move("old", "new", sdk.MoveOpts{Author: "alice"}); err != nil {
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

	if err := sdk.Documents.Write("a/one", []byte("1"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write a/one: %v", err)
	}
	if err := sdk.Documents.Write("a/two", []byte("2"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write a/two: %v", err)
	}
	if err := sdk.Documents.Write("b/three", []byte("3"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write b/three: %v", err)
	}

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

	if err := sdk.Documents.Write("a", []byte("1"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("2"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write b: %v", err)
	}

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

	if err := sdk.Documents.Write("keep", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write keep: %v", err)
	}
	if err := sdk.Documents.Write("gone", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write gone: %v", err)
	}
	if err := sdk.Documents.Delete("gone", sdk.DeleteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

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

	if err := sdk.Documents.Write("doc", []byte("hello world"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

	if err := sdk.Documents.Edit("doc", "world", "Go", sdk.EditOpts{Author: "alice", Message: "fix"}); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	content, _ := sdk.Documents.Read("doc", 0)
	if string(content) != "hello Go" {
		t.Errorf("content = %q, want %q", content, "hello Go")
	}
}

func TestDocumentsHistory(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("v1"), sdk.WriteOpts{Author: "alice", Message: "first"}); err != nil {
		t.Fatalf("Write v1: %v", err)
	}
	if err := sdk.Documents.Write("doc", []byte("v2"), sdk.WriteOpts{Author: "bob", Message: "second"}); err != nil {
		t.Fatalf("Write v2: %v", err)
	}

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

	if err := sdk.Documents.Write("doc", []byte("v1"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write v1: %v", err)
	}
	if err := sdk.Documents.Write("doc", []byte("v2"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write v2: %v", err)
	}
	if err := sdk.Documents.Write("doc", []byte("v3"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write v3: %v", err)
	}

	versions, _ := sdk.Documents.History("doc", 2)
	if len(versions) != 2 {
		t.Errorf("got %d versions, want 2", len(versions))
	}
}

func TestDocumentsDiff(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("line1\nline2\n"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write v1: %v", err)
	}
	if err := sdk.Documents.Write("doc", []byte("line1\nchanged\n"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write v2: %v", err)
	}

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

	if err := sdk.Documents.Write("doc", []byte("original"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write original: %v", err)
	}
	if err := sdk.Documents.Write("doc", []byte("changed"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write changed: %v", err)
	}

	if err := sdk.Documents.Revert("doc", 1, sdk.RevertOpts{Author: "alice", Message: "revert"}); err != nil {
		t.Fatalf("Revert: %v", err)
	}

	content, _ := sdk.Documents.Read("doc", 0)
	if string(content) != "original" {
		t.Errorf("content = %q, want %q", content, "original")
	}
}

func TestDocumentsGlob(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("notes/a.md", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write notes/a.md: %v", err)
	}
	if err := sdk.Documents.Write("notes/b.md", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write notes/b.md: %v", err)
	}
	if err := sdk.Documents.Write("other/c.md", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write other/c.md: %v", err)
	}

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

	if err := sdk.Documents.Write("doc1", []byte("the quick brown fox"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc1: %v", err)
	}
	if err := sdk.Documents.Write("doc2", []byte("lazy dog"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc2: %v", err)
	}

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

func TestDocumentsGrepLiteralPunctuation(t *testing.T) {
	// Terms that contain words alongside punctuation - hyphens,
	// colons, etc. - must work as literal searches without the user
	// having to escape FTS5 syntax. The host bridge wraps these as
	// phrase queries so they tokenise to their constituent words and
	// match.
	//
	// Note: pure-punctuation searches like "#", "-", "##" cannot be
	// matched by the underlying FTS5 unicode61 tokeniser because it
	// strips punctuation from the index entirely. Supporting those
	// would require switching the FTS table to a trigram tokeniser.
	cases := []struct {
		name    string
		content string
		query   string
	}{
		{"hyphen-word", "foo-bar baz", "foo-bar"},
		{"colon-phrase", "Authentication: OAuth2 setup", "Authentication: OAuth2"},
		{"phrase-with-space", "the quick brown fox", "quick brown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testHost(t)
			if err := sdk.Documents.Write("doc", []byte(tc.content), sdk.WriteOpts{Author: "alice"}); err != nil {
				t.Fatalf("Write: %v", err)
			}
			hits, err := sdk.Documents.Grep(tc.query, sdk.GrepOpts{})
			if err != nil {
				t.Fatalf("Grep(%q): %v", tc.query, err)
			}
			if len(hits) == 0 {
				t.Errorf("Grep(%q) returned no hits, want at least 1", tc.query)
			}
		})
	}
}

func TestDocumentsGrepKeepsFTS5Operators(t *testing.T) {
	// Power users with valid FTS5 syntax must not be silently
	// re-quoted. "foo OR bar" is a valid FTS5 query and should hit
	// documents containing either word.
	testHost(t)
	if err := sdk.Documents.Write("a", []byte("foo only"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if err := sdk.Documents.Write("b", []byte("bar only"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write b: %v", err)
	}
	if err := sdk.Documents.Write("c", []byte("baz only"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write c: %v", err)
	}
	hits, err := sdk.Documents.Grep("foo OR bar", sdk.GrepOpts{})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	paths := make(map[string]bool)
	for _, h := range hits {
		paths[h.Path] = true
	}
	if !paths["a"] || !paths["b"] {
		t.Errorf("got paths %v, want both a and b", paths)
	}
	if paths["c"] {
		t.Errorf("got path c in results, want only a and b")
	}
}

func TestDocumentsGrepPathFilter(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("a/doc", []byte("needle"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write a/doc: %v", err)
	}
	if err := sdk.Documents.Write("b/doc", []byte("needle"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write b/doc: %v", err)
	}

	hits, _ := sdk.Documents.Grep("needle", sdk.GrepOpts{Path: "a/"})
	if len(hits) != 1 {
		t.Errorf("got %d hits, want 1 (path-filtered)", len(hits))
	}
}

func TestDocumentsVacuum(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}
	if err := sdk.Documents.Delete("doc", sdk.DeleteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

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
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("Read error = %v, want sdk.ErrNotFound", err)
	}
}

func TestDocumentsDeleteNotFound(t *testing.T) {
	testHost(t)

	err := sdk.Documents.Delete("nonexistent", sdk.DeleteOpts{Author: "alice"})
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("Delete error = %v, want sdk.ErrNotFound", err)
	}
}

func TestDocumentsMoveNotFound(t *testing.T) {
	testHost(t)

	err := sdk.Documents.Move("nonexistent", "dest", sdk.MoveOpts{Author: "alice"})
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("Move error = %v, want sdk.ErrNotFound", err)
	}
}

func TestDocumentsEditNotFound(t *testing.T) {
	testHost(t)

	err := sdk.Documents.Edit("nonexistent", "old", "new", sdk.EditOpts{Author: "alice"})
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("Edit error = %v, want sdk.ErrNotFound", err)
	}
}

func TestDocumentsHistoryNotFound(t *testing.T) {
	testHost(t)

	_, err := sdk.Documents.History("nonexistent", 0)
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("History error = %v, want sdk.ErrNotFound", err)
	}
}

func TestDocumentsEditPatternNotFound(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("hello world"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

	err := sdk.Documents.Edit("doc", "xyz", "abc", sdk.EditOpts{Author: "alice"})
	if !errors.Is(err, sdk.ErrNoMatch) {
		t.Errorf("Edit error = %v, want sdk.ErrNoMatch", err)
	}
}

func TestDocumentsEditNotUnique(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("foo bar foo"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

	err := sdk.Documents.Edit("doc", "foo", "qux", sdk.EditOpts{Author: "alice"})
	if !errors.Is(err, sdk.ErrNotUnique) {
		t.Errorf("Edit error = %v, want sdk.ErrNotUnique", err)
	}

	// Document should be unchanged.
	content, _ := sdk.Documents.Read("doc", 0)
	if string(content) != "foo bar foo" {
		t.Errorf("content = %q, want %q (no edit should have applied)", content, "foo bar foo")
	}
}

func TestDocumentsEditReplaceAll(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("foo bar foo baz foo"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

	if err := sdk.Documents.Edit("doc", "foo", "qux", sdk.EditOpts{Author: "alice", ReplaceAll: true}); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	content, _ := sdk.Documents.Read("doc", 0)
	if string(content) != "qux bar qux baz qux" {
		t.Errorf("content = %q, want %q", content, "qux bar qux baz qux")
	}
}

func TestDocumentsEditNoOp(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("doc", []byte("hello"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

	err := sdk.Documents.Edit("doc", "hello", "hello", sdk.EditOpts{Author: "alice"})
	if !errors.Is(err, sdk.ErrNoOp) {
		t.Errorf("Edit error = %v, want sdk.ErrNoOp", err)
	}
}

func TestDocumentsWriteEmpty(t *testing.T) {
	testHost(t)

	if err := sdk.Documents.Write("empty", []byte(""), sdk.WriteOpts{Author: "alice"}); err != nil {
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

	if err := sdk.Documents.Write("doc", []byte("first"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write first: %v", err)
	}
	if err := sdk.Documents.Write("doc", []byte("second"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write second: %v", err)
	}

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

	if err := sdk.Documents.Write("notes/a.md", []byte("x"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write notes/a.md: %v", err)
	}

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

	if err := sdk.Documents.Write("doc", []byte("hello world"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

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

	if err := sdk.Documents.Write("doc", []byte("line one\nline two\nline three"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

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

	if err := sdk.Documents.Write("doc", []byte("aaa\nbbb\nccc\nddd\neee"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

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
	if err := sdk.Documents.Write("doc", []byte(content), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

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

	if err := sdk.Documents.Write("doc", []byte("same"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write doc: %v", err)
	}

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
	if err := os.WriteFile(filepath.Join(importDir, "one.md"), []byte("first"), 0644); err != nil {
		t.Fatalf("WriteFile one.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(importDir, "two.md"), []byte("second"), 0644); err != nil {
		t.Fatalf("WriteFile two.md: %v", err)
	}

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
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile doc.md: %v", err)
	}

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
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

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

	if err := sdk.Documents.Write("notes/one", []byte("first"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write notes/one: %v", err)
	}
	if err := sdk.Documents.Write("notes/two", []byte("second"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write notes/two: %v", err)
	}

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

	if err := sdk.Documents.Write("exp/doc", []byte("store content"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write exp/doc: %v", err)
	}

	dir := t.TempDir()
	// Export appends .md, so pre-create doc.md to trigger skip
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

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

	if err := sdk.Documents.Write("exp/doc", []byte("new content"), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write exp/doc: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte("old"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

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
