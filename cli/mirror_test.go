package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// metaMirrorDir is where mirror writes for the default store, relative
// to the store's working directory.
const metaMirrorDir = ".llmd/llmd"

// TestMirror covers pull (default and explicit, unchanged, stale
// removal, prefix, empty store) and push (updated, created, unchanged,
// prefix, missing directory) with the mirror directory under the
// store's working directory, plus author enforcement. Each case runs
// in a fresh working directory holding "docs/a", "docs/sub/b" and
// "other/x"; seed runs before the command under test.
func TestMirror(t *testing.T) {
	pullAll := func(t *testing.T, c sdk.Context, dir string) {
		run(t, c, "mirror", nil, "pull")
	}

	tests := []struct {
		name       string
		author     string
		seed       func(t *testing.T, c sdk.Context, dir string)
		args       []string
		wantText   string            // exact response text
		wantFiles  map[string]string // relative to the mirror dir: expected content
		wantNoFile []string          // relative to the mirror dir
		wantDocs   map[string]string // document path: expected content afterwards
		wantErr    error
		wantAnyErr bool
	}{
		{
			name:      "pull is the default",
			author:    "alice",
			args:      nil,
			wantText:  "Pulled to .llmd/llmd/ (wrote 3)",
			wantFiles: map[string]string{"docs/a.md": "# A\n", "docs/sub/b.md": "# B\n", "other/x.md": "# X\n"},
		},
		{
			name:      "explicit pull",
			author:    "alice",
			args:      []string{"pull"},
			wantText:  "Pulled to .llmd/llmd/ (wrote 3)",
			wantFiles: map[string]string{"docs/a.md": "# A\n"},
		},
		{
			name:     "pull again is unchanged",
			author:   "alice",
			seed:     pullAll,
			args:     []string{"pull"},
			wantText: "Pulled to .llmd/llmd/ (unchanged 3)",
		},
		{
			name:   "pull rewrites a changed document",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				pullAll(t, c, dir)
				writeDoc(t, c, "docs/a", "# A v2\n")
			},
			args:      []string{"pull"},
			wantText:  "Pulled to .llmd/llmd/ (wrote 1, unchanged 2)",
			wantFiles: map[string]string{"docs/a.md": "# A v2\n"},
		},
		{
			name:   "pull removes stale files",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				pullAll(t, c, dir)
				run(t, c, "rm", nil, "other/x")
			},
			args:       []string{"pull"},
			wantText:   "Pulled to .llmd/llmd/ (unchanged 2, removed 1 stale)",
			wantNoFile: []string{"other/x.md", "other"},
		},
		{
			name:       "pull with prefix writes only that subtree",
			author:     "alice",
			args:       []string{"pull", "docs/"},
			wantText:   "Pulled to .llmd/llmd/ (wrote 2)",
			wantFiles:  map[string]string{"docs/a.md": "# A\n", "docs/sub/b.md": "# B\n"},
			wantNoFile: []string{"other/x.md"},
		},
		{
			// A bare word is not treated as a prefix: a typo in the
			// subcommand must not pull or push anything.
			name:    "prefix without a subcommand is rejected",
			author:  "alice",
			args:    []string{"docs/"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "unknown subcommand",
			author:  "alice",
			args:    []string{"sync"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:     "pull without author",
			author:   "",
			args:     []string{"pull"},
			wantText: "Pulled to .llmd/llmd/ (wrote 3)",
		},
		{
			name:   "push updated file",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				pullAll(t, c, dir)
				metaWriteFile(t, filepath.Join(dir, metaMirrorDir, "docs", "a.md"), "# A edited\n")
			},
			args:     []string{"push"},
			wantText: "Pushed from .llmd/llmd/ (updated 1, unchanged 2)",
			wantDocs: map[string]string{"docs/a": "# A edited\n"},
		},
		{
			name:   "push new file",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				pullAll(t, c, dir)
				metaWriteFile(t, filepath.Join(dir, metaMirrorDir, "new", "n.md"), "# N\n")
			},
			args:     []string{"push"},
			wantText: "Pushed from .llmd/llmd/ (created 1, unchanged 3)",
			wantDocs: map[string]string{"new/n": "# N\n"},
		},
		{
			name:     "push unchanged",
			author:   "alice",
			seed:     pullAll,
			args:     []string{"push"},
			wantText: "Pushed from .llmd/llmd/ (unchanged 3)",
		},
		{
			// A prefix limits push to that part of the mirror, as it
			// limits pull; it once copied the whole mirror under it.
			name:   "push with prefix only pushes under it",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				pullAll(t, c, dir)
				metaWriteFile(t, filepath.Join(dir, metaMirrorDir, "docs", "a.md"), "# A edited\n")
				metaWriteFile(t, filepath.Join(dir, metaMirrorDir, "other", "x.md"), "# X edited\n")
			},
			args:     []string{"push", "docs"},
			wantText: "Pushed from .llmd/llmd/ (updated 1, unchanged 1)",
			wantDocs: map[string]string{"docs/a": "# A edited\n", "other/x": "# X\n"},
		},
		{
			name:   "push ignores non-markdown files",
			author: "alice",
			seed: func(t *testing.T, c sdk.Context, dir string) {
				pullAll(t, c, dir)
				metaWriteFile(t, filepath.Join(dir, metaMirrorDir, "notes.txt"), "plain\n")
			},
			args:     []string{"push"},
			wantText: "Pushed from .llmd/llmd/ (unchanged 3)",
		},
		{
			name:       "push without a mirror directory",
			author:     "alice",
			args:       []string{"push"},
			wantAnyErr: true,
		},
		{
			name:    "push without author",
			author:  "",
			seed:    pullAll,
			args:    []string{"push"},
			wantErr: sdk.ErrMissingArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed, dir := metaWorkdir(t, "alice")
			writeDoc(t, seed, "docs/a", "# A\n")
			writeDoc(t, seed, "docs/sub/b", "# B\n")
			writeDoc(t, seed, "other/x", "# X\n")
			if tt.seed != nil {
				tt.seed(t, seed, dir)
			}
			c := seed
			c.Author = tt.author

			if tt.wantErr != nil || tt.wantAnyErr {
				err := runErr(t, c, "mirror", nil, tt.args...)
				if tt.wantAnyErr && err == nil {
					t.Errorf("mirror %v error = nil, want an error", tt.args)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Errorf("mirror %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "mirror", nil, tt.args...)
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("mirror %v response type = %T, want sdk.Text", tt.args, resp)
			}
			if got, want := text(resp), nativeMirrorText(tt.wantText); got != want {
				t.Errorf("mirror %v = %q, want %q", tt.args, got, want)
			}
			for rel, want := range tt.wantFiles {
				if got := metaReadFile(t, filepath.Join(dir, metaMirrorDir, rel)); got != want {
					t.Errorf("after mirror %v, %s = %q, want %q", tt.args, rel, got, want)
				}
			}
			for _, rel := range tt.wantNoFile {
				if _, err := os.Stat(filepath.Join(dir, metaMirrorDir, rel)); err == nil {
					t.Errorf("after mirror %v, %s exists, want absent", tt.args, rel)
				}
			}
			for p, want := range tt.wantDocs {
				if got := text(run(t, seed, "cat", nil, p)); got != want {
					t.Errorf("after mirror %v, cat %s = %q, want %q", tt.args, p, got, want)
				}
			}
		})
	}
}

// TestMirrorEmptyStore checks the messages when there is nothing to
// move in either direction.
func TestMirrorEmptyStore(t *testing.T) {
	c, dir := metaWorkdir(t, "alice")

	if got := text(run(t, c, "mirror", nil)); got != "Nothing to mirror" {
		t.Errorf("mirror on empty store = %q, want %q", got, "Nothing to mirror")
	}
	// Pull creates the directory even when empty, so push finds it.
	if _, err := os.Stat(filepath.Join(dir, metaMirrorDir)); err != nil {
		t.Fatalf("mirror directory after empty pull: %v", err)
	}
	if got := text(run(t, c, "mirror", nil, "push")); got != "Nothing to push" {
		t.Errorf("mirror push on empty mirror = %q, want %q", got, "Nothing to push")
	}
}

// TestMirrorPrefixPullKeepsOtherFiles checks that pulling one subtree
// does not delete mirrored files belonging to documents outside it:
// only documents since deleted or renamed are stale.
func TestMirrorPrefixPullKeepsOtherFiles(t *testing.T) {
	c, dir := metaWorkdir(t, "alice")
	writeDoc(t, c, "docs/a", "# A\n")
	writeDoc(t, c, "other/x", "# X\n")
	run(t, c, "mirror", nil, "pull")

	got := text(run(t, c, "mirror", nil, "pull", "docs/"))
	if _, err := os.Stat(filepath.Join(dir, metaMirrorDir, "other", "x.md")); err != nil {
		t.Fatalf("other/x.md after prefix pull: %v (output %q)", err, got)
	}
	if got != nativeMirrorText("Pulled to .llmd/llmd/ (unchanged 1)") {
		t.Errorf("mirror pull docs/ = %q, want %q", got, "Pulled to .llmd/llmd/ (unchanged 1)")
	}
}

// nativeMirrorText rewrites the mirror directory in an expected message
// with the platform's separators, which is how the command prints it.
func nativeMirrorText(want string) string {
	return strings.ReplaceAll(want, metaMirrorDir+"/", filepath.FromSlash(metaMirrorDir)+string(filepath.Separator))
}
