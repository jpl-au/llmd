package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// metaWriteFile creates path (and its parents) with content, failing
// the test on error.
func metaWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// metaImportTree lays out the fixture an import test reads: two
// markdown files (one nested), a non-markdown file, and hidden
// entries that must be ignored. It returns the directory.
func metaImportTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	metaWriteFile(t, filepath.Join(dir, "a.md"), "# A\n")
	metaWriteFile(t, filepath.Join(dir, "sub", "b.md"), "# B\n")
	metaWriteFile(t, filepath.Join(dir, "notes.txt"), "not markdown\n")
	metaWriteFile(t, filepath.Join(dir, ".hidden.md"), "# Hidden\n")
	metaWriteFile(t, filepath.Join(dir, ".git", "c.md"), "# In hidden dir\n")
	return dir
}

// TestImport covers importing a directory and a single file, the
// --prefix and --dry-run flags, the skipped/updated
// classification on re-import, and the error cases. Each case runs
// against a fresh store; seed runs before the command under test.
func TestImport(t *testing.T) {
	tests := []struct {
		name       string
		author     string
		seed       func(t *testing.T, c sdk.Context, dir string)
		args       func(dir string) []string // {dir} is the fixture directory
		wantText   []string
		wantAbsent []string
		wantDocs   []string // document paths that must exist afterwards
		wantNoDocs []string // document paths that must not exist afterwards
		wantData   func(t *testing.T, r *sdk.ImportResult)
		wantErr    error
		wantAnyErr bool
	}{
		{
			name:       "directory",
			author:     "alice",
			args:       func(dir string) []string { return []string{dir} },
			wantText:   []string{"created a", "created sub/b"},
			wantAbsent: []string{"notes", "hidden", ".git", "skipped", "updated"},
			wantDocs:   []string{"a", "sub/b"},
			wantNoDocs: []string{"notes", ".hidden", ".git/c"},
			wantData: func(t *testing.T, r *sdk.ImportResult) {
				if len(r.Created) != 2 || len(r.Updated) != 0 || len(r.Skipped) != 0 {
					t.Errorf("ImportResult = %+v, want 2 created", *r)
				}
			},
		},
		{
			name:     "prefix",
			author:   "alice",
			args:     func(dir string) []string { return []string{"--prefix", "docs", dir} },
			wantText: []string{"created docs/a", "created docs/sub/b"},
			wantDocs: []string{"docs/a", "docs/sub/b"},
		},
		{
			name:     "prefix with trailing slash",
			author:   "alice",
			args:     func(dir string) []string { return []string{"--prefix", "docs/", dir} },
			wantText: []string{"created docs/a"},
			wantDocs: []string{"docs/a"},
		},
		{
			name:     "single file",
			author:   "alice",
			args:     func(dir string) []string { return []string{filepath.Join(dir, "sub", "b.md")} },
			wantText: []string{"created b"},
			wantDocs: []string{"b"},
		},
		{
			name:       "dry run creates nothing",
			author:     "alice",
			args:       func(dir string) []string { return []string{"--dry-run", dir} },
			wantText:   []string{"created a", "created sub/b"},
			wantNoDocs: []string{"a", "sub/b"},
			wantData: func(t *testing.T, r *sdk.ImportResult) {
				if len(r.Created) != 2 {
					t.Errorf("ImportResult = %+v, want 2 created", *r)
				}
			},
		},
		{
			name:   "unchanged files are skipped",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				run(t, c, "import", nil, dir)
			},
			args:       func(dir string) []string { return []string{dir} },
			wantText:   []string{"skipped 2 unchanged"},
			wantAbsent: []string{"created", "updated"},
			wantData: func(t *testing.T, r *sdk.ImportResult) {
				if len(r.Skipped) != 2 || len(r.Created) != 0 || len(r.Updated) != 0 {
					t.Errorf("ImportResult = %+v, want 2 skipped", *r)
				}
			},
		},
		{
			name:   "changed file is updated",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				run(t, c, "import", nil, dir)
				metaWriteFile(t, filepath.Join(dir, "a.md"), "# A v2\n")
			},
			args:     func(dir string) []string { return []string{dir} },
			wantText: []string{"updated a", "skipped 1 unchanged"},
			wantData: func(t *testing.T, r *sdk.ImportResult) {
				if len(r.Updated) != 1 || r.Updated[0] != "a" || len(r.Skipped) != 1 {
					t.Errorf("ImportResult = %+v, want a updated and 1 skipped", *r)
				}
			},
		},
		{
			name:   "dry run reports update without writing",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				run(t, c, "import", nil, dir)
				metaWriteFile(t, filepath.Join(dir, "a.md"), "# A v2\n")
			},
			args:     func(dir string) []string { return []string{"--dry-run", dir} },
			wantText: []string{"updated a"},
		},
		{
			name:    "missing directory argument",
			author:  "alice",
			args:    func(string) []string { return nil },
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			author:  "alice",
			args:    func(dir string) []string { return []string{"--bogus", dir} },
			wantErr: sdk.ErrInvalidArg,
		},
		{
			// The bulk importer supports flattening, but the command
			// does not expose it.
			name:    "flatten flag is not defined",
			author:  "alice",
			args:    func(dir string) []string { return []string{"--flatten", dir} },
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:       "nonexistent directory",
			author:     "alice",
			args:       func(dir string) []string { return []string{filepath.Join(dir, "nope")} },
			wantAnyErr: true,
		},
		{
			name:    "without author",
			author:  "",
			args:    func(dir string) []string { return []string{dir} },
			wantErr: sdk.ErrMissingArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed := newCLI(t, "alice")
			dir := metaImportTree(t)
			if tt.seed != nil {
				tt.seed(t, seed, dir)
			}
			c := seed
			c.Author = tt.author
			args := tt.args(dir)

			if tt.wantErr != nil || tt.wantAnyErr {
				err := runErr(t, c, "import", nil, args...)
				if tt.wantAnyErr && err == nil {
					t.Errorf("import %v error = nil, want an error", args)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("import %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "import", nil, args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("import %v = %q, want to contain %q", args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("import %v = %q, must not contain %q", args, got, absent)
				}
			}
			r, ok := data(resp).(*sdk.ImportResult)
			if !ok {
				t.Fatalf("import %v Data = %T, want *sdk.ImportResult", args, data(resp))
			}
			if tt.wantData != nil {
				tt.wantData(t, r)
			}
			for _, p := range tt.wantDocs {
				if ok, err := seed.Documents.Exists(p); err != nil || !ok {
					t.Errorf("after import %v, Exists(%s) = %v, %v, want true", args, p, ok, err)
				}
			}
			for _, p := range tt.wantNoDocs {
				if ok, _ := seed.Documents.Exists(p); ok {
					t.Errorf("after import %v, document %s exists, want absent", args, p)
				}
			}
		})
	}
}

// TestImportContent checks that imported content round-trips through
// cat, that a changed file produces a new version, and that importing
// identical content again is reported as skipped and keeps the version.
func TestImportContent(t *testing.T) {
	c := newCLI(t, "alice")
	dir := metaImportTree(t)

	run(t, c, "import", nil, dir)
	if got := text(run(t, c, "cat", nil, "sub/b")); got != "# B\n" {
		t.Errorf("cat sub/b = %q, want %q", got, "# B\n")
	}

	version := func(path string) int {
		t.Helper()
		docs, err := c.Documents.List(path, sdk.ListOpts{})
		if err != nil || len(docs) != 1 {
			t.Fatalf("List(%s) = %v, %v", path, docs, err)
		}
		return docs[0].Version
	}

	metaWriteFile(t, filepath.Join(dir, "a.md"), "# A v2\n")
	run(t, c, "import", nil, dir)
	if got := text(run(t, c, "cat", nil, "a")); got != "# A v2\n" {
		t.Errorf("cat a after re-import = %q, want %q", got, "# A v2\n")
	}
	if v := version("a"); v != 2 {
		t.Errorf("version of a after changed re-import = %d, want 2", v)
	}

	if got := text(run(t, c, "import", nil, dir)); !strings.Contains(got, "skipped 2 unchanged") {
		t.Errorf("import of identical content = %q, want skipped 2 unchanged", got)
	}
	if v := version("a"); v != 2 {
		t.Errorf("version of a after identical re-import = %d, want 2", v)
	}
}
