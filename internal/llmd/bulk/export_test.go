package bulk_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{Author: "test", Source: "cli"}
}

// doc is a store document written before an export or expected after
// an import.
type doc struct {
	path    string
	content string
}

// TestExport covers single-document and prefix exports into a fresh
// temp directory: destination resolution, the .md extension, nested
// directories, version selection, and the Overwrite option against
// pre-existing files. Paths in existing, wantFiles and wantAbsent are
// relative to the temp directory; an empty dest exports into the temp
// directory itself.
func TestExport(t *testing.T) {
	tests := []struct {
		name         string
		docs         []doc             // written to the store in order
		existing     map[string]string // files created before the export
		path         string
		dest         string
		opts         bulk.ExportOptions
		wantExported int
		wantSkipped  int
		wantFiles    map[string]string
		wantAbsent   []string
	}{
		{
			name:         "single doc to file",
			docs:         []doc{{"docs/readme", "# Hello"}},
			path:         "docs/readme",
			dest:         "readme.md",
			wantExported: 1,
			wantFiles:    map[string]string{"readme.md": "# Hello"},
		},
		{
			// Should use doc name as filename.
			name:         "single doc to directory",
			docs:         []doc{{"docs/readme", "content"}},
			path:         "docs/readme",
			wantExported: 1,
			wantFiles:    map[string]string{"readme.md": "content"},
		},
		{
			name:         "prefix",
			docs:         []doc{{"docs/readme", "readme"}, {"docs/guide", "guide"}, {"notes/todo", "todo"}},
			path:         "docs/",
			wantExported: 2,
			wantFiles:    map[string]string{"readme.md": "readme", "guide.md": "guide"},
			wantAbsent:   []string{"todo.md"},
		},
		{
			name:        "skips existing",
			docs:        []doc{{"readme", "new content"}},
			existing:    map[string]string{"readme.md": "existing"},
			path:        "readme",
			dest:        "readme.md",
			wantSkipped: 1,
			wantFiles:   map[string]string{"readme.md": "existing"},
		},
		{
			name:         "overwrite",
			docs:         []doc{{"readme", "new content"}},
			existing:     map[string]string{"readme.md": "existing"},
			path:         "readme",
			dest:         "readme.md",
			opts:         bulk.ExportOptions{Overwrite: true},
			wantExported: 1,
			wantFiles:    map[string]string{"readme.md": "new content"},
		},
		{
			name:         "specific version",
			docs:         []doc{{"readme", "version 1"}, {"readme", "version 2"}},
			path:         "readme:1",
			dest:         "readme.md",
			wantExported: 1,
			wantFiles:    map[string]string{"readme.md": "version 1"},
		},
		{
			name:         "creates directories",
			docs:         []doc{{"readme", "content"}},
			path:         "readme",
			dest:         "sub/deep/readme.md",
			wantExported: 1,
			wantFiles:    map[string]string{"sub/deep/readme.md": "content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()

			for _, d := range tt.docs {
				if _, err := s.Documents.Write(ctx, d.path, d.content, testWriteOpts()); err != nil {
					t.Fatalf("Write(%q) error = %v", d.path, err)
				}
			}

			dir := t.TempDir()
			for name, content := range tt.existing {
				if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(content), 0644); err != nil {
					t.Fatalf("WriteFile(%q) error = %v", name, err)
				}
			}
			dest := dir
			if tt.dest != "" {
				dest = filepath.Join(dir, filepath.FromSlash(tt.dest))
			}

			result, err := s.Bulk.Export(ctx, tt.path, dest, tt.opts)
			if err != nil {
				t.Fatalf("Export(%q, %q) error = %v", tt.path, tt.dest, err)
			}
			if len(result.Exported) != tt.wantExported {
				t.Errorf("Export(%q).Exported = %d, want %d", tt.path, len(result.Exported), tt.wantExported)
			}
			if len(result.Skipped) != tt.wantSkipped {
				t.Errorf("Export(%q).Skipped = %d, want %d", tt.path, len(result.Skipped), tt.wantSkipped)
			}
			if len(result.Errors) != 0 {
				t.Errorf("Export(%q).Errors = %v, want none", tt.path, result.Errors)
			}

			for name, want := range tt.wantFiles {
				content, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
				if err != nil {
					t.Errorf("ReadFile(%q) error = %v", name, err)
					continue
				}
				if string(content) != want {
					t.Errorf("%s content = %q, want %q", name, string(content), want)
				}
			}
			for _, name := range tt.wantAbsent {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err == nil {
					t.Errorf("%s exists, want absent", name)
				}
			}
		})
	}
}

// TestExport_TraversalRejected verifies that a raw traversal path in the
// store (normalisation lives in the host layer, so the internal store can
// hold one) never produces a file outside the destination directory.
// Single-document export uses only the base name; prefix export keeps the
// relative path, so os.Root must reject it and Export must report it.
func TestExport_TraversalRejected(t *testing.T) {
	tests := []struct {
		name       string
		docPath    string
		export     string
		wantInside string // file expected inside dest, "" for none
		wantErrors int
	}{
		{
			name:       "single document uses base name",
			docPath:    "../escape",
			export:     "../escape",
			wantInside: "escape.md",
		},
		{
			name:       "prefix export rejected by root",
			docPath:    "docs/../escape",
			export:     "docs/",
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			ctx := t.Context()

			if _, err := s.Documents.Write(ctx, tt.docPath, "pwned", testWriteOpts()); err != nil {
				t.Fatalf("Write(%q) error = %v", tt.docPath, err)
			}

			dir := t.TempDir()
			outside := filepath.Join(filepath.Dir(dir), "escape.md")
			t.Cleanup(func() { _ = os.Remove(outside) })

			result, err := s.Bulk.Export(ctx, tt.export, dir, bulk.ExportOptions{})
			if err != nil {
				t.Fatalf("Export(%q) error = %v", tt.export, err)
			}
			if len(result.Errors) != tt.wantErrors {
				t.Errorf("Export(%q) errors = %v, want %d", tt.export, result.Errors, tt.wantErrors)
			}
			if _, err := os.Stat(outside); err == nil {
				t.Errorf("Export(%q) wrote outside the destination: %s", tt.export, outside)
			}
			if tt.wantInside != "" {
				if _, err := os.Stat(filepath.Join(dir, tt.wantInside)); err != nil {
					t.Errorf("Export(%q) did not write %s: %v", tt.export, tt.wantInside, err)
				}
			}
		})
	}
}
