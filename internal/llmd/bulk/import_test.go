package bulk_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

// writeFiles creates the given files under dir, making parent
// directories as needed. Names are slash-separated and relative.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", name, err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
	}
}

// TestImport covers a first import from a fresh temp directory: single
// file and directory sources, prefix and flatten path rewriting,
// hidden-file handling, dry run, and the markdown-only filter. The
// Origin in opts is filled from testImportOpts; an empty target
// imports the whole temp directory.
func TestImport(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string // created under the temp directory
		target      string            // file to import, relative to the temp directory
		opts        bulk.ImportOptions
		wantCreated int
		wantDocs    []doc    // documents that must exist with this content
		wantMissing []string // documents that must not exist
	}{
		{
			name:        "single file",
			files:       map[string]string{"readme.md": "# Hello"},
			target:      "readme.md",
			wantCreated: 1,
			wantDocs:    []doc{{"readme", "# Hello"}},
		},
		{
			// Nested files keep their relative path.
			name:        "directory",
			files:       map[string]string{"readme.md": "readme", "guide.md": "guide", "sub/nested.md": "nested"},
			wantCreated: 3,
			wantDocs:    []doc{{"readme", "readme"}, {"guide", "guide"}, {"sub/nested", "nested"}},
		},
		{
			name:        "with prefix",
			files:       map[string]string{"readme.md": "content"},
			opts:        bulk.ImportOptions{Prefix: "docs/"},
			wantCreated: 1,
			wantDocs:    []doc{{"docs/readme", "content"}},
		},
		{
			// Should be flattened to just "file".
			name:        "flatten",
			files:       map[string]string{"sub/deep/file.md": "content"},
			opts:        bulk.ImportOptions{Flatten: true},
			wantCreated: 1,
			wantDocs:    []doc{{"file", "content"}},
		},
		{
			name:        "skips hidden",
			files:       map[string]string{"visible.md": "visible", ".hidden.md": "hidden"},
			wantCreated: 1,
			wantDocs:    []doc{{"visible", "visible"}},
			wantMissing: []string{".hidden"},
		},
		{
			name:        "includes hidden",
			files:       map[string]string{"visible.md": "visible", ".hidden.md": "hidden"},
			opts:        bulk.ImportOptions{Hidden: true},
			wantCreated: 2,
			wantDocs:    []doc{{"visible", "visible"}, {".hidden", "hidden"}},
		},
		{
			// Reports what would be created without writing anything.
			name:        "dry run",
			files:       map[string]string{"readme.md": "content"},
			opts:        bulk.ImportOptions{DryRun: true},
			wantCreated: 1,
			wantMissing: []string{"readme"},
		},
		{
			name:        "only markdown",
			files:       map[string]string{"readme.md": "markdown", "readme.txt": "text", "readme.json": "{}"},
			wantCreated: 1,
			wantDocs:    []doc{{"readme", "markdown"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()

			dir := t.TempDir()
			writeFiles(t, dir, tt.files)
			target := dir
			if tt.target != "" {
				target = filepath.Join(dir, filepath.FromSlash(tt.target))
			}
			opts := tt.opts
			opts.Origin = testImportOpts().Origin

			result, err := s.Bulk.Import(ctx, target, opts)
			if err != nil {
				t.Fatalf("Import(%q) error = %v", tt.target, err)
			}
			if len(result.Created) != tt.wantCreated {
				t.Errorf("Import(%q).Created = %d, want %d", tt.target, len(result.Created), tt.wantCreated)
			}
			if len(result.Errors) != 0 {
				t.Errorf("Import(%q).Errors = %v, want none", tt.target, result.Errors)
			}

			for _, d := range tt.wantDocs {
				got, err := s.Documents.Read(ctx, d.path)
				if err != nil {
					t.Errorf("Read(%q) error = %v", d.path, err)
					continue
				}
				if got.Content != d.content {
					t.Errorf("Read(%q).Content = %q, want %q", d.path, got.Content, d.content)
				}
			}
			for _, path := range tt.wantMissing {
				if _, err := s.Documents.Read(ctx, path); !errors.Is(err, documents.ErrNotFound) {
					t.Errorf("Read(%q) error = %v, want ErrNotFound", path, err)
				}
			}
		})
	}
}

// TestImport_Reimport covers importing the same directory twice: an
// unchanged file is skipped, a modified file produces a new version,
// and Force bypasses bulk's skip check (Documents.Write still
// deduplicates identical content, so the version does not advance).
func TestImport_Reimport(t *testing.T) {
	tests := []struct {
		name        string
		initial     string
		modified    string // rewrites readme.md before the second import; empty leaves it unchanged
		force       bool
		wantCreated int
		wantUpdated int
		wantSkipped int
		wantContent string
		wantVersion int
	}{
		{
			name:        "skips unchanged",
			initial:     "content",
			wantSkipped: 1,
			wantContent: "content",
			wantVersion: 1,
		},
		{
			name:        "updates changed",
			initial:     "version 1",
			modified:    "version 2",
			wantUpdated: 1,
			wantContent: "version 2",
			wantVersion: 2,
		},
		{
			name:        "force bypasses skip",
			initial:     "content",
			force:       true,
			wantUpdated: 1,
			wantContent: "content",
			wantVersion: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()

			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{"readme.md": tt.initial})

			// First import creates the document.
			first, err := s.Bulk.Import(ctx, dir, testImportOpts())
			if err != nil {
				t.Fatalf("first Import() error = %v", err)
			}
			if len(first.Created) != 1 {
				t.Errorf("first Import().Created = %d, want 1", len(first.Created))
			}

			if tt.modified != "" {
				writeFiles(t, dir, map[string]string{"readme.md": tt.modified})
			}
			opts := testImportOpts()
			opts.Force = tt.force

			second, err := s.Bulk.Import(ctx, dir, opts)
			if err != nil {
				t.Fatalf("second Import() error = %v", err)
			}
			if len(second.Created) != tt.wantCreated {
				t.Errorf("second Import().Created = %d, want %d", len(second.Created), tt.wantCreated)
			}
			if len(second.Updated) != tt.wantUpdated {
				t.Errorf("second Import().Updated = %d, want %d", len(second.Updated), tt.wantUpdated)
			}
			if len(second.Skipped) != tt.wantSkipped {
				t.Errorf("second Import().Skipped = %d, want %d", len(second.Skipped), tt.wantSkipped)
			}

			got, err := s.Documents.Read(ctx, "readme")
			if err != nil {
				t.Fatalf("Read(readme) error = %v", err)
			}
			if got.Content != tt.wantContent {
				t.Errorf("Read(readme).Content = %q, want %q", got.Content, tt.wantContent)
			}
			if got.Version != tt.wantVersion {
				t.Errorf("Read(readme).Version = %d, want %d", got.Version, tt.wantVersion)
			}
		})
	}
}
