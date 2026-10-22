package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/host"
	"github.com/jpl-au/llmd/sdk"
)

// metaStoreless changes into an empty temporary directory (restored
// when the test ends) and returns a host with no store, as the binary
// has before init runs, plus the directory. The host is closed at the
// end of the test so a store opened by a successful init is released.
func metaStoreless(t *testing.T) (*host.Host, string) {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	h := host.New()
	t.Cleanup(func() {
		if err := h.Close(); err != nil {
			t.Errorf("closing host: %v", err)
		}
	})
	return h, dir
}

// metaInit dispatches init through a storeless host with the given
// --db value, as the binary does.
func metaInit(t *testing.T, h *host.Host, author, dbPath string) (sdk.Response, error) {
	t.Helper()
	c := h.Context(t.Context(), author)
	return c.Runtime.Dispatch(t.Context(), "init", nil, author, nil, dbPath)
}

// TestInit covers creating the default store, the --db shorthand and
// explicit paths, the gitignore whitelist, the refusal to initialise
// twice, and running without an author. Each case starts in an empty
// directory with a storeless host.
func TestInit(t *testing.T) {
	whitelist := []string{"*", "!llmd.db", "!.gitignore"}

	tests := []struct {
		name          string
		author        string
		db            string // --db value; {dir} is the working directory
		wantText      string // {dir} expanded
		wantDB        string // store file relative to the working directory, {dir} expanded
		wantGitignore bool   // .llmd/.gitignore holds the whitelist
		wantAnyErr    bool
	}{
		{
			name:          "default path",
			author:        "alice",
			wantText:      "Initialised llmd store at .llmd/llmd.db",
			wantDB:        filepath.Join(".llmd", "llmd.db"),
			wantGitignore: true,
		},
		{
			name:          "db shorthand",
			author:        "alice",
			db:            "docs",
			wantText:      "Initialised llmd store at .llmd/llmd-docs.db",
			wantDB:        filepath.Join(".llmd", "llmd-docs.db"),
			wantGitignore: true,
		},
		{
			name:          "db shorthand with spaces is sanitised",
			author:        "alice",
			db:            "my  docs",
			wantText:      "Initialised llmd store at .llmd/llmd-my-docs.db",
			wantDB:        filepath.Join(".llmd", "llmd-my-docs.db"),
			wantGitignore: true,
		},
		{
			name:     "db explicit path",
			author:   "alice",
			db:       filepath.Join("{dir}", "custom", "store.db"),
			wantText: "Initialised llmd store at {dir}/custom/store.db",
			wantDB:   filepath.Join("custom", "store.db"),
		},
		{
			name:       "db shorthand with illegal character",
			author:     "alice",
			db:         "a<b",
			wantAnyErr: true,
		},
		{
			name:       "db shorthand with traversal",
			author:     "alice",
			db:         "..",
			wantAnyErr: true,
		},
		{
			// init is not attributed to an author.
			name:          "without author",
			author:        "",
			wantText:      "Initialised llmd store at .llmd/llmd.db",
			wantDB:        filepath.Join(".llmd", "llmd.db"),
			wantGitignore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, dir := metaStoreless(t)
			expand := func(s string) string { return strings.ReplaceAll(s, "{dir}", dir) }
			db := expand(tt.db)

			resp, err := metaInit(t, h, tt.author, db)
			if tt.wantAnyErr {
				if err == nil {
					t.Errorf("init --db %q error = nil, want an error", db)
				}
				if entries, _ := os.ReadDir(dir); len(entries) != 0 {
					t.Errorf("init --db %q created %v, want nothing", db, entries)
				}
				return
			}
			if err != nil {
				t.Fatalf("init --db %q: %v", db, err)
			}
			if got := text(resp); got != expand(tt.wantText) {
				t.Errorf("init --db %q = %q, want %q", db, got, expand(tt.wantText))
			}
			if _, err := os.Stat(filepath.Join(dir, tt.wantDB)); err != nil {
				t.Errorf("after init --db %q, store file: %v", db, err)
			}
			if tt.wantGitignore {
				got := metaReadFile(t, filepath.Join(dir, ".llmd", ".gitignore"))
				var rules []string
				for _, line := range strings.Split(got, "\n") {
					line = strings.TrimSpace(line)
					if line != "" && !strings.HasPrefix(line, "#") {
						rules = append(rules, line)
					}
				}
				if strings.Join(rules, "\n") != strings.Join(whitelist, "\n") {
					t.Errorf("after init, .llmd/.gitignore rules = %v, want %v", rules, whitelist)
				}
			}

			// A second init must not overwrite the store.
			if _, err := metaInit(t, h, tt.author, db); err == nil || !strings.Contains(err.Error(), "already exists") {
				t.Errorf("second init --db %q error = %v, want already exists", db, err)
			}
		})
	}
}

// TestInitThenUse checks that the store init creates is usable by the
// same host: a write and a read succeed, and the gitignore is left
// alone by later commands.
func TestInitThenUse(t *testing.T) {
	h, dir := metaStoreless(t)
	if _, err := metaInit(t, h, "alice", ""); err != nil {
		t.Fatal(err)
	}
	c := h.Context(t.Context(), "alice")
	writeDoc(t, c, "notes/hi", "hello")
	if got := text(run(t, c, "cat", nil, "notes/hi")); got != "hello" {
		t.Errorf("cat notes/hi after init = %q, want %q", got, "hello")
	}
	before := metaReadFile(t, filepath.Join(dir, ".llmd", ".gitignore"))
	run(t, c, "config", nil, "git", "ls")
	if after := metaReadFile(t, filepath.Join(dir, ".llmd", ".gitignore")); after != before {
		t.Errorf(".gitignore changed by config git ls:\n%s", after)
	}
}
