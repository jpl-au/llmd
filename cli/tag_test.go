package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestTag covers the five shapes of the tag command (add, remove,
// list by document, list all, find) by path and by key, the data
// payload each returns, and the error cases. Each case runs against
// a fresh store holding "docs/a" and "docs/b", with "docs/b" already
// tagged "shared".
func TestTag(t *testing.T) {
	tests := []struct {
		name       string
		author     string   // "" dispatches without an author
		args       []string // {key} is replaced by the key of docs/a
		wantText   []string
		wantAbsent []string
		wantData   func(t *testing.T, d any)
		wantErr    error
	}{
		{
			name:     "add tag",
			author:   "alice",
			args:     []string{"docs/a", "important"},
			wantText: []string{"Tagged docs/a with important"},
		},
		{
			name:     "add tag by key",
			author:   "alice",
			args:     []string{"{key}", "important"},
			wantText: []string{"Tagged", "with important"},
		},
		{
			name:     "list tags on document",
			author:   "alice",
			args:     []string{"docs/b"},
			wantText: []string{"shared"},
			wantData: func(t *testing.T, d any) {
				tags, ok := d.([]sdk.Tag)
				if !ok || len(tags) != 1 || tags[0].Name != "shared" || tags[0].Path != "docs/b" {
					t.Errorf("tag docs/b Data = %#v, want one sdk.Tag shared on docs/b", d)
				}
			},
		},
		{
			name:     "list all tags with counts",
			author:   "alice",
			args:     nil,
			wantText: []string{"shared (1)"},
			wantData: func(t *testing.T, d any) {
				infos, ok := d.([]sdk.TagInfo)
				if !ok || len(infos) != 1 || infos[0].Name != "shared" || infos[0].Count != 1 {
					t.Errorf("tag Data = %#v, want one sdk.TagInfo shared (1)", d)
				}
			},
		},
		{
			name:       "find documents with tag",
			author:     "alice",
			args:       []string{"-f", "shared"},
			wantText:   []string{"docs/b"},
			wantAbsent: []string{"docs/a"},
			wantData: func(t *testing.T, d any) {
				paths, ok := d.([]string)
				if !ok || len(paths) != 1 || paths[0] != "docs/b" {
					t.Errorf("tag -f shared Data = %#v, want [docs/b]", d)
				}
			},
		},
		{
			name:     "find with long flag",
			author:   "alice",
			args:     []string{"--find", "shared"},
			wantText: []string{"docs/b"},
		},
		{
			name:   "find unknown tag is empty",
			author: "alice",
			args:   []string{"-f", "nope"},
			wantData: func(t *testing.T, d any) {
				if paths, _ := d.([]string); len(paths) != 0 {
					t.Errorf("tag -f nope Data = %#v, want empty", d)
				}
			},
		},
		{
			name:     "remove tag",
			author:   "alice",
			args:     []string{"-d", "docs/b", "shared"},
			wantText: []string{"Removed tag shared from docs/b"},
		},
		{
			name:     "remove tag with long flag",
			author:   "alice",
			args:     []string{"--delete", "docs/b", "shared"},
			wantText: []string{"Removed tag shared from docs/b"},
		},
		{
			name:    "remove tag not on document",
			author:  "alice",
			args:    []string{"-d", "docs/a", "nope"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "duplicate tag",
			author:  "alice",
			args:    []string{"docs/b", "shared"},
			wantErr: sdk.ErrExists,
		},
		{
			name:    "tag missing document",
			author:  "alice",
			args:    []string{"docs/missing", "important"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "find without name",
			author:  "alice",
			args:    []string{"-f"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			author:  "alice",
			args:    []string{"--bogus", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:     "list all without author",
			author:   "",
			args:     nil,
			wantText: []string{"shared (1)"},
		},
		{
			name:     "list document without author",
			author:   "",
			args:     []string{"docs/b"},
			wantText: []string{"shared"},
		},
		{
			name:     "find without author",
			author:   "",
			args:     []string{"-f", "shared"},
			wantText: []string{"docs/b"},
		},
		{
			name:    "add without author",
			author:  "",
			args:    []string{"docs/a", "important"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "remove without author",
			author:  "",
			args:    []string{"-d", "docs/b", "shared"},
			wantErr: sdk.ErrMissingArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed := newCLI(t, "alice")
			writeDoc(t, seed, "docs/a", "# A")
			writeDoc(t, seed, "docs/b", "# B")
			run(t, seed, "tag", nil, "docs/b", "shared")
			docs, err := seed.Documents.List("docs/a", sdk.ListOpts{})
			if err != nil || len(docs) == 0 {
				t.Fatalf("List(docs/a) = %v, %v", docs, err)
			}
			key := docs[0].Key

			c := seed
			c.Author = tt.author
			args := make([]string, len(tt.args))
			for i, a := range tt.args {
				args[i] = strings.ReplaceAll(a, "{key}", key)
			}

			if tt.wantErr != nil {
				err := runErr(t, c, "tag", nil, args...)
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("tag %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "tag", nil, args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("tag %v = %q, want to contain %q", args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("tag %v = %q, must not contain %q", args, got, absent)
				}
			}
			if tt.wantData != nil {
				tt.wantData(t, data(resp))
			}
		})
	}
}

// TestTagRoundTrip checks that add, list by document, list all, find
// and remove agree with each other across a sequence of calls on one
// store, including addressing by key.
func TestTagRoundTrip(t *testing.T) {
	c := newCLI(t, "alice")
	writeDoc(t, c, "docs/a", "# A")
	writeDoc(t, c, "docs/b", "# B")
	docs, err := c.Documents.List("docs/a", sdk.ListOpts{})
	if err != nil || len(docs) == 0 {
		t.Fatalf("List(docs/a) = %v, %v", docs, err)
	}
	key := docs[0].Key

	run(t, c, "tag", nil, "docs/a", "x")
	run(t, c, "tag", nil, "docs/b", "x")
	run(t, c, "tag", nil, key, "y")

	if got := text(run(t, c, "tag", nil)); !strings.Contains(got, "x (2)") || !strings.Contains(got, "y (1)") {
		t.Errorf("tag = %q, want x (2) and y (1)", got)
	}
	if got := text(run(t, c, "tag", nil, "docs/a")); !strings.Contains(got, "x") || !strings.Contains(got, "y") {
		t.Errorf("tag docs/a = %q, want x and y", got)
	}
	if got := text(run(t, c, "tag", nil, key)); !strings.Contains(got, "y") {
		t.Errorf("tag %s = %q, want y", key, got)
	}
	if got := text(run(t, c, "tag", nil, "-f", "x")); !strings.Contains(got, "docs/a") || !strings.Contains(got, "docs/b") {
		t.Errorf("tag -f x = %q, want docs/a and docs/b", got)
	}

	run(t, c, "tag", nil, "-d", "docs/a", "x")

	if got := text(run(t, c, "tag", nil, "-f", "x")); got != "docs/b" {
		t.Errorf("tag -f x after removal = %q, want docs/b", got)
	}
	if got := text(run(t, c, "tag", nil, "docs/a")); strings.Contains(got, "x") {
		t.Errorf("tag docs/a after removal = %q, must not contain x", got)
	}
	if got := text(run(t, c, "tag", nil)); !strings.Contains(got, "x (1)") {
		t.Errorf("tag after removal = %q, want x (1)", got)
	}
}
