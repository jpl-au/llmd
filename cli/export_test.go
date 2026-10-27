package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/line"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// metaWorkdir initialises a store in a temporary directory, changes
// into that directory for the rest of the test, and returns the
// sdk.Context a command receives as author plus the directory. Use for
// commands that read or write relative to the working directory
// (export's default target, mirror, config, config git).
func metaWorkdir(t *testing.T, author string) (sdk.Context, string) {
	t.Helper()
	h := host.TestSetup(t, host.TestWorkdir)
	return h.Context(t.Context(), author), h.Dir()
}

// metaReadFile returns the content of path, failing the test if it
// cannot be read.
func metaReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

// TestExport covers single-document and prefix exports, the target
// directory argument and its default, --overwrite, skipped existing
// files, per-document errors, and the error cases. Each case runs
// against a fresh store holding "docs/a", "docs/sub/b" and "other/x";
// {dir} in args is a fresh temporary directory.
func TestExport(t *testing.T) {
	tests := []struct {
		name       string
		author     string
		workdir    bool // run from the store directory (for the default target)
		seed       func(t *testing.T, c sdk.Context, dir string)
		args       []string
		wantText   []string
		wantAbsent []string
		wantFiles  map[string]string // relative to {dir}: expected content
		wantNoFile []string          // relative to {dir}
		wantData   func(t *testing.T, dir string, r *sdk.ExportResult)
		wantErr    error
		wantAnyErr bool
	}{
		{
			name:      "single document to directory",
			author:    "alice",
			args:      []string{"docs/a", "{dir}"},
			wantText:  []string{"exported {dir}/a.md"},
			wantFiles: map[string]string{"a.md": "# A\n"},
			wantData: func(t *testing.T, dir string, r *sdk.ExportResult) {
				if len(r.Exported) != 1 || r.Exported[0] != filepath.Join(dir, "a.md") || len(r.Skipped) != 0 || len(r.Errors) != 0 {
					t.Errorf("ExportResult = %+v, want only %s exported", *r, filepath.Join(dir, "a.md"))
				}
			},
		},
		{
			name:      "single document to file path",
			author:    "alice",
			args:      []string{"docs/a", "{dir}/renamed.md"},
			wantText:  []string{"exported {dir}/renamed.md"},
			wantFiles: map[string]string{"renamed.md": "# A\n"},
		},
		{
			name:       "prefix exports the subtree",
			author:     "alice",
			args:       []string{"docs/", "{dir}"},
			wantText:   []string{"exported {dir}/a.md", "exported {dir}/sub/b.md"},
			wantAbsent: []string{"other", "skipped", "failed"},
			wantFiles:  map[string]string{"a.md": "# A\n", "sub/b.md": "# B\n"},
			wantNoFile: []string{"x.md", "other/x.md"},
			wantData: func(t *testing.T, dir string, r *sdk.ExportResult) {
				if len(r.Exported) != 2 {
					t.Errorf("ExportResult = %+v, want 2 exported", *r)
				}
			},
		},
		{
			name:       "prefix with no documents",
			author:     "alice",
			args:       []string{"nothing/", "{dir}"},
			wantAbsent: []string{"exported", "skipped", "failed"},
			wantData: func(t *testing.T, dir string, r *sdk.ExportResult) {
				if len(r.Exported)+len(r.Skipped)+len(r.Errors) != 0 {
					t.Errorf("ExportResult = %+v, want empty", *r)
				}
			},
		},
		{
			name:   "existing file is skipped",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				metaWriteFile(t, filepath.Join(dir, "a.md"), line.Native("local edit\n"))
			},
			args:       []string{"docs/a", "{dir}"},
			wantText:   []string{"skipped 1 existing"},
			wantAbsent: []string{"exported"},
			wantFiles:  map[string]string{"a.md": "local edit\n"},
			wantData: func(t *testing.T, dir string, r *sdk.ExportResult) {
				if len(r.Skipped) != 1 || r.Skipped[0] != filepath.Join(dir, "a.md") || len(r.Exported) != 0 {
					t.Errorf("ExportResult = %+v, want only %s skipped", *r, filepath.Join(dir, "a.md"))
				}
			},
		},
		{
			name:   "overwrite replaces existing file",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				metaWriteFile(t, filepath.Join(dir, "a.md"), line.Native("local edit\n"))
			},
			args:       []string{"--overwrite", "docs/a", "{dir}"},
			wantText:   []string{"exported {dir}/a.md"},
			wantAbsent: []string{"skipped"},
			wantFiles:  map[string]string{"a.md": "# A\n"},
		},
		{
			name:   "prefix skips existing and exports the rest",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				metaWriteFile(t, filepath.Join(dir, "a.md"), line.Native("local edit\n"))
			},
			args:      []string{"docs/", "{dir}"},
			wantText:  []string{"exported {dir}/sub/b.md", "skipped 1 existing"},
			wantFiles: map[string]string{"a.md": "local edit\n", "sub/b.md": "# B\n"},
		},
		{
			// A directory in the way of a file cannot be overwritten;
			// the prefix export reports it and carries on.
			name:   "prefix reports per-document errors",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, "a.md"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			args:      []string{"--overwrite", "docs/", "{dir}"},
			wantText:  []string{"failed docs/a: ", "exported {dir}/sub/b.md"},
			wantFiles: map[string]string{"sub/b.md": "# B\n"},
			wantData: func(t *testing.T, dir string, r *sdk.ExportResult) {
				if len(r.Errors) != 1 || !strings.HasPrefix(r.Errors[0], "docs/a: ") || len(r.Exported) != 1 {
					t.Errorf("ExportResult = %+v, want one error for docs/a and one export", *r)
				}
			},
		},
		{
			name:   "single document error is the call error",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, "a.md"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			args:       []string{"--overwrite", "docs/a", "{dir}"},
			wantAnyErr: true,
		},
		{
			name:      "default target is the working directory",
			author:    "alice",
			workdir:   true,
			args:      []string{"docs/a"},
			wantText:  []string{"exported a.md"},
			wantFiles: map[string]string{"a.md": "# A\n"},
		},
		{
			name:      "prefix export creates the destination",
			author:    "alice",
			args:      []string{"docs/", "{dir}/missing"},
			wantText:  []string{"exported "},
			wantFiles: map[string]string{"missing/a.md": "# A\n"},
		},
		{
			name:    "missing document",
			author:  "alice",
			args:    []string{"docs/missing", "{dir}"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "no arguments",
			author:  "alice",
			args:    nil,
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			author:  "alice",
			args:    []string{"--bogus", "docs/a", "{dir}"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:      "without author",
			author:    "",
			args:      []string{"docs/a", "{dir}"},
			wantText:  []string{"exported {dir}/a.md"},
			wantFiles: map[string]string{"a.md": "# A\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seed sdk.Context
			var dir string
			if tt.workdir {
				seed, dir = metaWorkdir(t, "alice")
			} else {
				seed = newCLI(t, "alice")
				dir = t.TempDir()
			}
			writeDoc(t, seed, "docs/a", "# A\n")
			writeDoc(t, seed, "docs/sub/b", "# B\n")
			writeDoc(t, seed, "other/x", "# X\n")
			if tt.seed != nil {
				tt.seed(t, seed, dir)
			}
			c := seed
			c.Author = tt.author
			expand := func(s string) string { return strings.ReplaceAll(s, "{dir}", dir) }
			args := make([]string, len(tt.args))
			for i, a := range tt.args {
				args[i] = expand(a)
			}

			if tt.wantErr != nil || tt.wantAnyErr {
				err := runErr(t, c, "export", nil, args...)
				if tt.wantAnyErr && err == nil {
					t.Errorf("export %v error = nil, want an error", args)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("export %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "export", nil, args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(filepath.ToSlash(got), filepath.ToSlash(expand(want))) {
					t.Errorf("export %v = %q, want to contain %q", args, got, expand(want))
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("export %v = %q, must not contain %q", args, got, absent)
				}
			}
			r, ok := data(resp).(*sdk.ExportResult)
			if !ok {
				t.Fatalf("export %v Data = %T, want *sdk.ExportResult", args, data(resp))
			}
			if tt.wantData != nil {
				tt.wantData(t, dir, r)
			}
			for rel, want := range tt.wantFiles {
				// Export writes the platform's line endings.
				if got, want := metaReadFile(t, filepath.Join(dir, rel)), line.Native(want); got != want {
					t.Errorf("after export %v, %s = %q, want %q", args, rel, got, want)
				}
			}
			for _, rel := range tt.wantNoFile {
				if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
					t.Errorf("after export %v, %s exists, want absent", args, rel)
				}
			}
		})
	}
}

// TestExportByKey checks that a single-document export accepts a
// document key and names the file after the key.
func TestExportByKey(t *testing.T) {
	c := newCLI(t, "alice")
	writeDoc(t, c, "docs/a", "# A\n")
	docs, err := c.Documents.List("docs/a", sdk.ListOpts{})
	if err != nil || len(docs) != 1 {
		t.Fatalf("List(docs/a) = %v, %v", docs, err)
	}
	dir := t.TempDir()

	got := text(run(t, c, "export", nil, docs[0].Key, dir))
	want := "exported " + filepath.Join(dir, docs[0].Key+".md")
	if got != want {
		t.Errorf("export %s = %q, want %q", docs[0].Key, got, want)
	}
	if content, want := metaReadFile(t, filepath.Join(dir, docs[0].Key+".md")), line.Native("# A\n"); content != want {
		t.Errorf("exported content = %q, want %q", content, want)
	}
}
