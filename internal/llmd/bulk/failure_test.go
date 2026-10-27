package bulk_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/bulk"
)

// TestImportEdges covers the import paths the happy-path tables do not
// reach: a missing source, a hidden directory skipped unless asked
// for, a file whose name cannot become a document path, and a dry run
// over a changed document.
func TestImportEdges(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string // written to the temp directory
		before      map[string]string // documents written to the store first
		path        string            // relative to the temp directory; empty means the directory
		opts        func(bulk.ImportOptions) bulk.ImportOptions
		wantErr     bool
		wantCreated []string
		wantUpdated []string
		wantSkipped []string
		wantErrors  []string // paths in the result's Errors
	}{
		{
			name:    "missing path",
			path:    "nope",
			wantErr: true,
		},
		{
			name:        "hidden directory skipped",
			files:       map[string]string{".git/notes.md": "x", "readme.md": "y"},
			wantCreated: []string{"readme"},
		},
		{
			name:        "hidden directory imported when asked",
			files:       map[string]string{".git/notes.md": "x", "readme.md": "y"},
			opts:        func(o bulk.ImportOptions) bulk.ImportOptions { o.Hidden = true; return o },
			wantCreated: []string{".git/notes", "readme"},
		},
		{
			name:        "name that is not a valid path is reported",
			files:       map[string]string{"a..b.md": "x", "ok.md": "y"},
			wantCreated: []string{"ok"},
			wantErrors:  []string{"a..b.md"},
		},
		{
			name:        "dry run reports an update without writing",
			files:       map[string]string{"readme.md": "new"},
			before:      map[string]string{"readme": "old"},
			opts:        func(o bulk.ImportOptions) bulk.ImportOptions { o.DryRun = true; return o },
			wantUpdated: []string{"readme"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()
			for p, c := range tt.before {
				if _, err := s.Documents.Write(ctx, p, c, testWriteOpts()); err != nil {
					t.Fatalf("Write(%q) error = %v", p, err)
				}
			}
			dir := t.TempDir()
			writeFiles(t, dir, tt.files)

			opts := testImportOpts()
			if tt.opts != nil {
				opts = tt.opts(opts)
			}
			result, err := s.Bulk.Import(ctx, filepath.Join(dir, tt.path), opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Import() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !sameSet(result.Created, tt.wantCreated) {
				t.Errorf("Created = %v, want %v", result.Created, tt.wantCreated)
			}
			if !sameSet(result.Updated, tt.wantUpdated) {
				t.Errorf("Updated = %v, want %v", result.Updated, tt.wantUpdated)
			}
			if !sameSet(result.Skipped, tt.wantSkipped) {
				t.Errorf("Skipped = %v, want %v", result.Skipped, tt.wantSkipped)
			}
			var errPaths []string
			for _, e := range result.Errors {
				errPaths = append(errPaths, e.Path)
			}
			if !sameSet(errPaths, tt.wantErrors) {
				t.Errorf("Errors = %v, want paths %v", result.Errors, tt.wantErrors)
			}
			if opts.DryRun {
				for p, c := range tt.before {
					doc, err := s.Documents.Read(ctx, p)
					if err != nil || doc.Content != c {
						t.Errorf("after dry run, %s = %q (%v), want unchanged %q", p, doc.Content, err, c)
					}
				}
			}
		})
	}
}

// sameSet reports whether two slices hold the same elements in any
// order. nil and empty are the same.
func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, g := range got {
		seen[g]++
	}
	for _, w := range want {
		if seen[w] == 0 {
			return false
		}
		seen[w]--
	}
	return true
}

// TestExportPrefixSkipsExisting checks that a prefix export leaves
// files that already exist alone and reports them as skipped.
func TestExportPrefixSkipsExisting(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	for _, d := range []doc{{"docs/a", "new a"}, {"docs/b", "new b"}} {
		if _, err := s.Documents.Write(ctx, d.path, d.content, testWriteOpts()); err != nil {
			t.Fatalf("Write(%q) error = %v", d.path, err)
		}
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a.md": "old a"})

	result, err := s.Bulk.Export(ctx, "docs/", dir, bulk.ExportOptions{})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(result.Exported) != 1 || len(result.Skipped) != 1 || !strings.HasSuffix(result.Skipped[0], "a.md") {
		t.Errorf("Export() = exported %v skipped %v, want one of each with a.md skipped", result.Exported, result.Skipped)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.md"))
	if err != nil || string(got) != "old a" {
		t.Errorf("a.md = %q (%v), want untouched %q", got, err, "old a")
	}
}

// TestMirrorDirectoryIsFile checks that a mirror target that is a
// file, not a directory, is an error rather than a silent no-op.
func TestMirrorDirectoryIsFile(t *testing.T) {
	s := testStore(t)
	if _, err := s.Documents.Write(t.Context(), "docs/a", "a", testWriteOpts()); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := s.Bulk.Mirror(t.Context(), "", target)
	if err == nil || !strings.Contains(err.Error(), "mirror directory") {
		t.Errorf("Mirror(file) error = %v, want a creating mirror directory error", err)
	}
	if _, ok := errors.AsType[*os.PathError](err); !ok {
		t.Errorf("Mirror(file) error = %v, want to wrap the os error", err)
	}
}
