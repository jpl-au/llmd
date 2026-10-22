package host

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

func testHost(t *testing.T) {
	t.Helper()
	TestSetup(t, TestMemory)
}

// writeDoc writes a document as alice and fails the test on error.
func writeDoc(t *testing.T, path, content string) {
	t.Helper()
	if err := sdk.Documents.Write(path, []byte(content), sdk.WriteOpts{Author: "alice"}); err != nil {
		t.Fatalf("Write(%q) error = %v", path, err)
	}
}

func TestDocumentsWrite(t *testing.T) {
	tests := []struct {
		name         string
		writes       []string // successive contents written to the same path
		want         string
		wantVersions int
	}{
		{"single write", []string{"# Hello"}, "# Hello", 1},
		{"empty content", []string{""}, "", 1},
		{"overwrite", []string{"first", "second"}, "second", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			for _, content := range tt.writes {
				if err := sdk.Documents.Write("doc", []byte(content), sdk.WriteOpts{Author: "alice", Message: "first"}); err != nil {
					t.Fatalf("Write(%q) error = %v", content, err)
				}
			}
			got, err := sdk.Documents.Read("doc", sdk.ReadOpts{})
			if err != nil {
				t.Fatalf("Read(doc) error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Read(doc) = %q, want %q", got, tt.want)
			}
			versions, err := sdk.Documents.History("doc", 0)
			if err != nil {
				t.Fatalf("History(doc) error = %v", err)
			}
			if len(versions) != tt.wantVersions {
				t.Errorf("History(doc) returned %d versions, want %d", len(versions), tt.wantVersions)
			}
		})
	}
}

func TestDocumentsRead(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		opts     sdk.ReadOpts
		want     string
	}{
		{"latest", []string{"v1", "v2"}, sdk.ReadOpts{}, "v2"},
		{"explicit version", []string{"v1", "v2"}, sdk.ReadOpts{Version: 1}, "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			for _, content := range tt.versions {
				writeDoc(t, "doc", content)
			}
			got, err := sdk.Documents.Read("doc", tt.opts)
			if err != nil {
				t.Fatalf("Read(doc, %+v) error = %v", tt.opts, err)
			}
			if string(got) != tt.want {
				t.Errorf("Read(doc, %+v) = %q, want %q", tt.opts, got, tt.want)
			}
		})
	}
}

func TestDocumentsExists(t *testing.T) {
	tests := []struct {
		name  string
		write bool
		want  bool
	}{
		{"missing", false, false},
		{"existing", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			if tt.write {
				writeDoc(t, "doc", "x")
			}
			got, err := sdk.Documents.Exists("doc")
			if err != nil {
				t.Fatalf("Exists(doc) error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Exists(doc) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDocumentsDeleteRestore(t *testing.T) {
	testHost(t)

	writeDoc(t, "doc", "content")

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

	writeDoc(t, "old", "moved")

	if err := sdk.Documents.Move("old", "new", sdk.MoveOpts{Author: "alice"}); err != nil {
		t.Fatalf("Move: %v", err)
	}

	ok, _ := sdk.Documents.Exists("old")
	if ok {
		t.Error("old path still exists after Move")
	}

	content, err := sdk.Documents.Read("new", sdk.ReadOpts{})
	if err != nil {
		t.Fatalf("Read new: %v", err)
	}
	if string(content) != "moved" {
		t.Errorf("content = %q, want %q", content, "moved")
	}
}

func TestDocumentsList(t *testing.T) {
	tests := []struct {
		name    string
		docs    []string
		deleted []string
		prefix  string
		opts    sdk.ListOpts
		want    []string // paths in returned order
	}{
		{"prefix", []string{"a/one", "a/two", "b/three"}, nil, "a/", sdk.ListOpts{}, []string{"a/one", "a/two"}},
		{"all", []string{"a/one", "a/two", "b/three"}, nil, "", sdk.ListOpts{}, []string{"a/one", "a/two", "b/three"}},
		{"reverse", []string{"a", "b"}, nil, "", sdk.ListOpts{Reverse: true}, []string{"b", "a"}},
		{"excludes deleted", []string{"keep", "gone"}, []string{"gone"}, "", sdk.ListOpts{}, []string{"keep"}},
		{"includes deleted", []string{"keep", "gone"}, []string{"gone"}, "", sdk.ListOpts{Deleted: true}, []string{"gone", "keep"}},
		{"empty store", nil, nil, "", sdk.ListOpts{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			for i, path := range tt.docs {
				writeDoc(t, path, string(rune('1'+i)))
			}
			for _, path := range tt.deleted {
				if err := sdk.Documents.Delete(path, sdk.DeleteOpts{Author: "alice"}); err != nil {
					t.Fatalf("Delete(%q) error = %v", path, err)
				}
			}
			docs, err := sdk.Documents.List(tt.prefix, tt.opts)
			if err != nil {
				t.Fatalf("List(%q, %+v) error = %v", tt.prefix, tt.opts, err)
			}
			var got []string
			for _, d := range docs {
				got = append(got, d.Path)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("List(%q, %+v) = %v, want %v", tt.prefix, tt.opts, got, tt.want)
			}
		})
	}
}

func TestDocumentsEdit(t *testing.T) {
	tests := []struct {
		name    string
		content string
		old     string
		new     string
		opts    sdk.EditOpts
		wantErr error
		want    string // content after Edit
	}{
		{"replace unique", "hello world", "world", "Go", sdk.EditOpts{Author: "alice", Message: "fix"}, nil, "hello Go"},
		{"replace all", "foo bar foo baz foo", "foo", "qux", sdk.EditOpts{Author: "alice", ReplaceAll: true}, nil, "qux bar qux baz qux"},
		{"pattern not found", "hello world", "xyz", "abc", sdk.EditOpts{Author: "alice"}, sdk.ErrNoMatch, "hello world"},
		{"not unique", "foo bar foo", "foo", "qux", sdk.EditOpts{Author: "alice"}, sdk.ErrNotUnique, "foo bar foo"},
		{"no-op", "hello", "hello", "hello", sdk.EditOpts{Author: "alice"}, sdk.ErrNoOp, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			writeDoc(t, "doc", tt.content)
			err := sdk.Documents.Edit("doc", tt.old, tt.new, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Edit(doc, %q, %q) error = %v, want %v", tt.old, tt.new, err, tt.wantErr)
			}
			got, err := sdk.Documents.Read("doc", sdk.ReadOpts{})
			if err != nil {
				t.Fatalf("Read(doc) error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Read(doc) after Edit = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDocumentsHistory(t *testing.T) {
	tests := []struct {
		name        string
		writes      []sdk.WriteOpts // one version per entry
		limit       int
		wantLen     int
		wantNumber  int // versions[0].Number (newest first)
		wantAuthor  string
		wantMessage string
	}{
		{
			name:        "newest first",
			writes:      []sdk.WriteOpts{{Author: "alice", Message: "first"}, {Author: "bob", Message: "second"}},
			limit:       0,
			wantLen:     2,
			wantNumber:  2,
			wantAuthor:  "bob",
			wantMessage: "second",
		},
		{
			name:       "limit",
			writes:     []sdk.WriteOpts{{Author: "alice"}, {Author: "alice"}, {Author: "alice"}},
			limit:      2,
			wantLen:    2,
			wantNumber: 3,
			wantAuthor: "alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			for i, opts := range tt.writes {
				if err := sdk.Documents.Write("doc", []byte(string(rune('1'+i))), opts); err != nil {
					t.Fatalf("Write(doc) #%d error = %v", i+1, err)
				}
			}
			versions, err := sdk.Documents.History("doc", tt.limit)
			if err != nil {
				t.Fatalf("History(doc, %d) error = %v", tt.limit, err)
			}
			if len(versions) != tt.wantLen {
				t.Fatalf("History(doc, %d) returned %d versions, want %d", tt.limit, len(versions), tt.wantLen)
			}
			if versions[0].Number != tt.wantNumber {
				t.Errorf("History(doc, %d)[0].Number = %d, want %d", tt.limit, versions[0].Number, tt.wantNumber)
			}
			if versions[0].Author != tt.wantAuthor {
				t.Errorf("History(doc, %d)[0].Author = %q, want %q", tt.limit, versions[0].Author, tt.wantAuthor)
			}
			if versions[0].Message != tt.wantMessage {
				t.Errorf("History(doc, %d)[0].Message = %q, want %q", tt.limit, versions[0].Message, tt.wantMessage)
			}
		})
	}
}

func TestDocumentsDiff(t *testing.T) {
	tests := []struct {
		name        string
		versions    []string
		from, to    string
		wantChanged bool // non-zero added/removed counts and non-empty text
	}{
		{"changed line", []string{"line1\nline2\n", "line1\nchanged\n"}, "doc:1", "doc:2", true},
		{"same version", []string{"same"}, "doc:1", "doc:1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			for _, content := range tt.versions {
				writeDoc(t, "doc", content)
			}
			diff, added, removed, err := sdk.Documents.Diff(tt.from, tt.to, 0)
			if err != nil {
				t.Fatalf("Diff(%q, %q) error = %v", tt.from, tt.to, err)
			}
			if changed := added != 0 || removed != 0; changed != tt.wantChanged {
				t.Errorf("Diff(%q, %q) = added %d, removed %d, want changed %v", tt.from, tt.to, added, removed, tt.wantChanged)
			}
			if tt.wantChanged && diff == "" {
				t.Errorf("Diff(%q, %q) text is empty, want changes", tt.from, tt.to)
			}
		})
	}
}

func TestDocumentsRevert(t *testing.T) {
	testHost(t)

	writeDoc(t, "doc", "original")
	writeDoc(t, "doc", "changed")

	if err := sdk.Documents.Revert("doc", 1, sdk.RevertOpts{Author: "alice", Message: "revert"}); err != nil {
		t.Fatalf("Revert: %v", err)
	}

	content, _ := sdk.Documents.Read("doc", sdk.ReadOpts{})
	if string(content) != "original" {
		t.Errorf("content = %q, want %q", content, "original")
	}
}

func TestDocumentsGlob(t *testing.T) {
	tests := []struct {
		name    string
		docs    []string
		pattern string
		want    []string
	}{
		{"matches", []string{"notes/a.md", "notes/b.md", "other/c.md"}, "notes/*", []string{"notes/a", "notes/b"}},
		{"no match", []string{"notes/a.md"}, "other/*", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			for _, path := range tt.docs {
				writeDoc(t, path, "x")
			}
			got, err := sdk.Documents.Glob(tt.pattern)
			if err != nil {
				t.Fatalf("Glob(%q) error = %v", tt.pattern, err)
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Glob(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

// TestDocumentsGrep checks which documents a query matches. Terms that
// contain words alongside punctuation - hyphens, colons, etc. - must
// work as literal searches without the user having to escape FTS5
// syntax: the host bridge wraps these as phrase queries so they
// tokenise to their constituent words and match. Power users with
// valid FTS5 syntax ("foo OR bar") must not be silently re-quoted.
//
// Note: pure-punctuation searches like "#", "-", "##" cannot be
// matched by the underlying FTS5 unicode61 tokeniser because it
// strips punctuation from the index entirely. Supporting those
// would require switching the FTS table to a trigram tokeniser.
func TestDocumentsGrep(t *testing.T) {
	type doc struct{ path, content string }
	tests := []struct {
		name  string
		docs  []doc
		query string
		opts  sdk.GrepOpts
		want  []string // sorted unique paths hit
	}{
		{"single word", []doc{{"doc1", "the quick brown fox"}, {"doc2", "lazy dog"}}, "quick", sdk.GrepOpts{}, []string{"doc1"}},
		{"hyphen-word", []doc{{"doc", "foo-bar baz"}}, "foo-bar", sdk.GrepOpts{}, []string{"doc"}},
		{"colon-phrase", []doc{{"doc", "Authentication: OAuth2 setup"}}, "Authentication: OAuth2", sdk.GrepOpts{}, []string{"doc"}},
		{"phrase-with-space", []doc{{"doc", "the quick brown fox"}}, "quick brown", sdk.GrepOpts{}, []string{"doc"}},
		{"keeps FTS5 operators", []doc{{"a", "foo only"}, {"b", "bar only"}, {"c", "baz only"}}, "foo OR bar", sdk.GrepOpts{}, []string{"a", "b"}},
		{"path filter", []doc{{"a/doc", "needle"}, {"b/doc", "needle"}}, "needle", sdk.GrepOpts{Path: "a/"}, []string{"a/doc"}},
		{"no match", []doc{{"doc", "hello world"}}, "nonexistent", sdk.GrepOpts{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			for _, d := range tt.docs {
				writeDoc(t, d.path, d.content)
			}
			hits, err := sdk.Documents.Grep(tt.query, tt.opts)
			if err != nil {
				t.Fatalf("Grep(%q, %+v) error = %v", tt.query, tt.opts, err)
			}
			var got []string
			for _, h := range hits {
				if !slices.Contains(got, h.Path) {
					got = append(got, h.Path)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Grep(%q, %+v) paths = %v, want %v", tt.query, tt.opts, got, tt.want)
			}
		})
	}
}

// TestDocumentsGrepModes checks the shape of a single hit in each
// result mode: line numbers and context in GrepLines, heading text in
// GrepSections.
func TestDocumentsGrepModes(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		opts    sdk.GrepOpts
		want    sdk.GrepHit
	}{
		{
			name:    "lines",
			content: "line one\nline two\nline three",
			query:   "two",
			opts:    sdk.GrepOpts{Mode: sdk.GrepLines},
			want:    sdk.GrepHit{Path: "doc", Line: 2, Column: 5, Text: "line two"},
		},
		{
			name:    "lines with context",
			content: "aaa\nbbb\nccc\nddd\neee",
			query:   "ccc",
			opts:    sdk.GrepOpts{Mode: sdk.GrepLines, Context: 1},
			want:    sdk.GrepHit{Path: "doc", Line: 3, Text: "ccc", Before: []string{"bbb"}, After: []string{"ddd"}},
		},
		{
			name:    "sections",
			content: "# Intro\n\nNothing here.\n\n# Details\n\nThe needle is here.\n",
			query:   "needle",
			opts:    sdk.GrepOpts{Mode: sdk.GrepSections},
			want:    sdk.GrepHit{Path: "doc", Line: 5, Text: "# Details\n\nThe needle is here.\n", Section: "Details"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			writeDoc(t, "doc", tt.content)
			hits, err := sdk.Documents.Grep(tt.query, tt.opts)
			if err != nil {
				t.Fatalf("Grep(%q, %+v) error = %v", tt.query, tt.opts, err)
			}
			if len(hits) != 1 {
				t.Fatalf("Grep(%q, %+v) returned %d hits, want 1", tt.query, tt.opts, len(hits))
			}
			got := hits[0]
			if got.Path != tt.want.Path || got.Line != tt.want.Line || got.Column != tt.want.Column ||
				got.Text != tt.want.Text || got.Section != tt.want.Section ||
				!slices.Equal(got.Before, tt.want.Before) || !slices.Equal(got.After, tt.want.After) {
				t.Errorf("Grep(%q, %+v)[0] = %+v, want %+v", tt.query, tt.opts, got, tt.want)
			}
		})
	}
}

func TestDocumentsVacuum(t *testing.T) {
	tests := []struct {
		name     string
		deleted  bool // write and soft-delete a document first
		wantDocs int64
	}{
		{"deleted document", true, 1},
		{"empty store", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			if tt.deleted {
				writeDoc(t, "doc", "x")
				if err := sdk.Documents.Delete("doc", sdk.DeleteOpts{Author: "alice"}); err != nil {
					t.Fatalf("Delete(doc) error = %v", err)
				}
			}
			result, err := sdk.Documents.Vacuum()
			if err != nil {
				t.Fatalf("Vacuum() error = %v", err)
			}
			if result.Documents != tt.wantDocs {
				t.Errorf("Vacuum().Documents = %d, want %d", result.Documents, tt.wantDocs)
			}
		})
	}
}

// TestDocumentsNotFound checks that every document operation on a
// missing path maps the internal error to sdk.ErrNotFound.
func TestDocumentsNotFound(t *testing.T) {
	tests := []struct {
		name string
		call func() error
	}{
		{"Read", func() error {
			_, err := sdk.Documents.Read("nonexistent", sdk.ReadOpts{})
			return err
		}},
		{"Delete", func() error {
			return sdk.Documents.Delete("nonexistent", sdk.DeleteOpts{Author: "alice"})
		}},
		{"Restore", func() error {
			return sdk.Documents.Restore("nonexistent", sdk.RestoreOpts{Author: "alice"})
		}},
		{"Move", func() error {
			return sdk.Documents.Move("nonexistent", "dest", sdk.MoveOpts{Author: "alice"})
		}},
		{"Edit", func() error {
			return sdk.Documents.Edit("nonexistent", "old", "new", sdk.EditOpts{Author: "alice"})
		}},
		{"History", func() error {
			_, err := sdk.Documents.History("nonexistent", 0)
			return err
		}},
		{"Diff", func() error {
			_, _, _, err := sdk.Documents.Diff("nonexistent", "nonexistent", 0)
			return err
		}},
		{"Revert", func() error {
			return sdk.Documents.Revert("nonexistent", 1, sdk.RevertOpts{Author: "alice"})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			if err := tt.call(); !errors.Is(err, sdk.ErrNotFound) {
				t.Errorf("%s(nonexistent) error = %v, want sdk.ErrNotFound", tt.name, err)
			}
		})
	}
}

// TestDocumentsImport reads files from a temp directory. Import strips
// the .md extension, so "one.md" is stored as "one".
func TestDocumentsImport(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string // filename -> content
		opts        sdk.ImportOpts
		wantCreated int
		wantExists  map[string]bool
		wantContent map[string]string
	}{
		{
			name:        "creates documents",
			files:       map[string]string{"one.md": "first", "two.md": "second"},
			opts:        sdk.ImportOpts{},
			wantCreated: 2,
			wantExists:  map[string]bool{"one": true, "two": true},
			wantContent: map[string]string{"one": "first"},
		},
		{
			name:        "dry run",
			files:       map[string]string{"doc.md": "content"},
			opts:        sdk.ImportOpts{DryRun: true},
			wantCreated: 1,
			wantExists:  map[string]bool{"doc.md": false, "doc": false},
		},
		{
			name:        "with prefix",
			files:       map[string]string{"note.md": "x"},
			opts:        sdk.ImportOpts{Prefix: "imported/"},
			wantCreated: 1,
			wantExists:  map[string]bool{"imported/note": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatalf("WriteFile(%q) error = %v", name, err)
				}
			}
			result, err := sdk.Documents.Import(dir, tt.opts)
			if err != nil {
				t.Fatalf("Import(%+v) error = %v", tt.opts, err)
			}
			if len(result.Created) != tt.wantCreated {
				t.Errorf("Import(%+v) created %d, want %d", tt.opts, len(result.Created), tt.wantCreated)
			}
			for path, want := range tt.wantExists {
				ok, err := sdk.Documents.Exists(path)
				if err != nil {
					t.Fatalf("Exists(%q) error = %v", path, err)
				}
				if ok != want {
					t.Errorf("Exists(%q) after Import = %v, want %v", path, ok, want)
				}
			}
			for path, want := range tt.wantContent {
				got, err := sdk.Documents.Read(path, sdk.ReadOpts{})
				if err != nil {
					t.Fatalf("Read(%q) error = %v", path, err)
				}
				if string(got) != want {
					t.Errorf("Read(%q) = %q, want %q", path, got, want)
				}
			}
		})
	}
}

// TestDocumentsExport writes documents under a prefix to a temp
// directory. The prefix must end in "/" for multi-document export and
// each exported file gets a .md extension.
func TestDocumentsExport(t *testing.T) {
	tests := []struct {
		name         string
		docs         map[string]string // path -> content
		existing     map[string]string // files pre-created in the export dir
		prefix       string
		opts         sdk.ExportOpts
		wantExported int
		wantSkipped  int
		wantFiles    map[string]string // filename -> content after export
	}{
		{
			name:         "exports documents",
			docs:         map[string]string{"notes/one": "first", "notes/two": "second"},
			prefix:       "notes/",
			opts:         sdk.ExportOpts{},
			wantExported: 2,
			wantFiles:    map[string]string{"one.md": "first"},
		},
		{
			name:        "skips existing",
			docs:        map[string]string{"exp/doc": "store content"},
			existing:    map[string]string{"doc.md": "existing"},
			prefix:      "exp/",
			opts:        sdk.ExportOpts{Overwrite: false},
			wantSkipped: 1,
			wantFiles:   map[string]string{"doc.md": "existing"},
		},
		{
			name:         "overwrites existing",
			docs:         map[string]string{"exp/doc": "new content"},
			existing:     map[string]string{"doc.md": "old"},
			prefix:       "exp/",
			opts:         sdk.ExportOpts{Overwrite: true},
			wantExported: 1,
			wantFiles:    map[string]string{"doc.md": "new content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHost(t)
			for path, content := range tt.docs {
				writeDoc(t, path, content)
			}
			dir := t.TempDir()
			for name, content := range tt.existing {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatalf("WriteFile(%q) error = %v", name, err)
				}
			}
			result, err := sdk.Documents.Export(tt.prefix, dir, tt.opts)
			if err != nil {
				t.Fatalf("Export(%q, %+v) error = %v", tt.prefix, tt.opts, err)
			}
			if len(result.Exported) != tt.wantExported {
				t.Errorf("Export(%q, %+v) exported %d, want %d", tt.prefix, tt.opts, len(result.Exported), tt.wantExported)
			}
			if len(result.Skipped) != tt.wantSkipped {
				t.Errorf("Export(%q, %+v) skipped %d, want %d", tt.prefix, tt.opts, len(result.Skipped), tt.wantSkipped)
			}
			for name, want := range tt.wantFiles {
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatalf("ReadFile(%q) error = %v", name, err)
				}
				if string(data) != want {
					t.Errorf("exported %s = %q, want %q", name, data, want)
				}
			}
		})
	}
}
