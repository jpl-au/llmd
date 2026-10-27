package cli

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/clock/clocktest"
	"github.com/jpl-au/llmd/sdk"
)

// TestLs covers listing: the "key path" default, prefix filtering,
// the -l table, -a for deleted documents, time and reverse ordering,
// combined short flags, --limit/--all, --since, the --tree fallback
// when not on a terminal, the empty store, the structured payload and
// the error cases.
//
// A deterministic clock orders the writes: notes/b is oldest, then
// docs/x, then notes/a, then gone/z (which is deleted afterwards).
func TestLs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string // {since:PATH} becomes the RFC 3339 creation time of PATH
		empty      bool     // run against an empty store
		wantOrder  []string // exact paths in order (plain output only)
		wantText   []string // substrings that must appear
		wantAbsent []string // substrings that must not appear
		wantLen    int      // number of entries in Data
		wantErr    error
		wantAnyErr bool // any error is acceptable (no sentinel is defined)
	}{
		{
			name:       "lists key and path sorted by path",
			args:       nil,
			wantOrder:  []string{"docs/x", "notes/a", "notes/b"},
			wantAbsent: []string{"gone/z"},
			wantLen:    3,
		},
		{
			name:      "prefix filter",
			args:      []string{"notes/"},
			wantOrder: []string{"notes/a", "notes/b"},
			wantLen:   2,
		},
		{
			name:      "prefix without trailing slash",
			args:      []string{"note"},
			wantOrder: []string{"notes/a", "notes/b"},
			wantLen:   2,
		},
		{
			name:      "no documents under prefix",
			args:      []string{"nothing/"},
			wantOrder: []string{},
			wantLen:   0,
		},
		{
			name:     "long format table",
			args:     []string{"-l"},
			wantText: []string{"KEY", "VER", "AUTHOR", "DATE", "PATH", "alice", "docs/x", "notes/a", "notes/b"},
			wantLen:  3,
		},
		{
			name:      "a includes deleted documents",
			args:      []string{"-a"},
			wantOrder: []string{"docs/x", "gone/z", "notes/a", "notes/b"},
			wantLen:   4,
		},
		{
			name:     "la marks deleted documents",
			args:     []string{"-la"},
			wantText: []string{"gone/z (deleted)"},
			wantLen:  4,
		},
		{
			name:      "t sorts newest first",
			args:      []string{"-t"},
			wantOrder: []string{"notes/a", "docs/x", "notes/b"},
			wantLen:   3,
		},
		{
			name:      "r reverses the path order",
			args:      []string{"-r"},
			wantOrder: []string{"notes/b", "notes/a", "docs/x"},
			wantLen:   3,
		},
		{
			name:      "tr sorts oldest first",
			args:      []string{"-tr"},
			wantOrder: []string{"notes/b", "docs/x", "notes/a"},
			wantLen:   3,
		},
		{
			name:     "lat combines long, all and time",
			args:     []string{"-lat"},
			wantText: []string{"KEY", "gone/z (deleted)"},
			wantLen:  4,
		},
		{
			name:      "limit caps the listing",
			args:      []string{"--limit", "2"},
			wantOrder: []string{"docs/x", "notes/a"},
			wantLen:   2,
		},
		{
			name:      "all beats limit",
			args:      []string{"--limit", "2", "--all"},
			wantOrder: []string{"docs/x", "notes/a", "notes/b"},
			wantLen:   3,
		},
		{
			name:      "since excludes documents up to the instant",
			args:      []string{"--since", "{since:docs/x}"},
			wantOrder: []string{"notes/a"},
			wantLen:   1,
		},
		{
			name:      "since in the future lists nothing",
			args:      []string{"--since", "2100-01-01T00:00:00Z"},
			wantOrder: []string{},
			wantLen:   0,
		},
		{
			name:      "tree falls back to flat paths when not a terminal",
			args:      []string{"--tree"},
			wantOrder: []string{"docs/x", "notes/a", "notes/b"},
			wantLen:   3,
		},
		{
			name:      "empty store",
			args:      nil,
			empty:     true,
			wantOrder: []string{},
			wantLen:   0,
		},
		{
			name:       "since must be a duration or timestamp",
			args:       []string{"--since", "yesterday"},
			wantAnyErr: true,
		},
		{
			name:       "prefix may not traverse",
			args:       []string{"../other"},
			wantAnyErr: true,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "unknown short flag",
			args:    []string{"-lz"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "limit must be an integer",
			args:    []string{"--limit", "few"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clocktest.Sequence(t, 1_000, 1)
			c := newCLI(t, "alice")
			if !tt.empty {
				writeDoc(t, c, "notes/b", "# B")
				writeDoc(t, c, "docs/x", "# X")
				writeDoc(t, c, "notes/a", "# A")
				writeDoc(t, c, "gone/z", "# Z")
				run(t, c, "rm", nil, "gone/z")
			}
			args := make([]string, len(tt.args))
			for i, a := range tt.args {
				if path, ok := strings.CutPrefix(a, "{since:"); ok {
					path = strings.TrimSuffix(path, "}")
					docs, err := c.Documents.List(path, sdk.ListOpts{})
					if err != nil || len(docs) != 1 {
						t.Fatalf("List(%s) = %v, %v", path, docs, err)
					}
					a = time.UnixMilli(docs[0].CreatedAt).UTC().Format(time.RFC3339Nano)
				}
				args[i] = a
			}

			if tt.wantAnyErr {
				if err := runErr(t, c, "ls", nil, args...); err == nil {
					t.Errorf("ls %v error = nil, want an error", args)
				}
				return
			}
			if tt.wantErr != nil {
				if err := runErr(t, c, "ls", nil, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("ls %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "ls", nil, args...)
			if _, ok := resp.(sdk.Result); !ok {
				t.Errorf("ls %v response type = %T, want sdk.Result", args, resp)
			}
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("ls %v = %q, want to contain %q", args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("ls %v = %q, must not contain %q", args, got, absent)
				}
			}

			if tt.wantOrder != nil {
				var paths []string
				if got != "" {
					for line := range strings.SplitSeq(got, "\n") {
						key, path, ok := strings.Cut(line, " ")
						if !ok || len(key) != 10 {
							t.Errorf("ls %v line %q, want \"<10-char key> <path>\"", args, line)
						}
						paths = append(paths, path)
					}
				}
				if !slices.Equal(paths, tt.wantOrder) {
					t.Errorf("ls %v paths = %v, want %v", args, paths, tt.wantOrder)
				}
			}

			// The structured payload: an empty listing is an empty
			// []sdk.Doc, otherwise one map per document with stable
			// field names.
			switch d := data(resp).(type) {
			case []sdk.Doc:
				if len(d) != 0 || tt.wantLen != 0 {
					t.Errorf("ls %v Data = %v, want %d entries", args, d, tt.wantLen)
				}
			case []map[string]any:
				if len(d) != tt.wantLen {
					t.Errorf("ls %v Data has %d entries, want %d", args, len(d), tt.wantLen)
				}
				for _, m := range d {
					for _, k := range []string{"key", "path", "version", "author", "message", "created_at", "deleted"} {
						if _, ok := m[k]; !ok {
							t.Errorf("ls %v Data entry %v lacks %q", args, m, k)
						}
					}
					if m["author"] != "alice" || m["version"] != 1 {
						t.Errorf("ls %v Data entry = %v, want author alice at version 1", args, m)
					}
					if deleted, _ := m["deleted"].(bool); deleted != (m["path"] == "gone/z") {
						t.Errorf("ls %v Data entry %v deleted = %v", args, m, deleted)
					}
				}
			default:
				t.Errorf("ls %v Data = %T, want []map[string]any or []sdk.Doc", args, data(resp))
			}
		})
	}
}

// TestLsTree exercises the terminal-only tree renderer directly, since
// ls falls back to flat output when stdout is not a terminal: nested
// directories, a deleted leaf, and a path that is both a document and
// a directory.
func TestLsTree(t *testing.T) {
	docs := []sdk.Doc{
		{Path: "notes/b", Deleted: true},
		{Path: "notes/a"},
		{Path: "notes"},
		{Path: "docs/deep/x"},
		{Path: "top"},
	}
	// Directory labels are styled, so strip escape sequences before
	// looking for the plain text.
	ansi := regexp.MustCompile("\x1b\\[[0-9;]*m")
	got := ansi.ReplaceAllString(buildTree(docs), "")
	for _, want := range []string{".", "notes", "a", "b (deleted)", "docs", "deep", "x", "top"} {
		if !strings.Contains(got, want) {
			t.Errorf("buildTree = %q, want to contain %q", got, want)
		}
	}
	if strings.Contains(got, "notes (deleted)") {
		t.Errorf("buildTree = %q, must not flag the live notes document deleted", got)
	}
	deleted := ansi.ReplaceAllString(buildTree([]sdk.Doc{{Path: "notes/a"}, {Path: "notes", Deleted: true}}), "")
	if !strings.Contains(deleted, "notes (deleted)") {
		t.Errorf("buildTree with deleted directory document = %q, want %q", deleted, "notes (deleted)")
	}
	if buildTree(nil) == "" {
		t.Errorf("buildTree(nil) = %q, want the root marker", buildTree(nil))
	}
}
