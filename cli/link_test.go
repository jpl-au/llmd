package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestLink covers creating plain and labelled links, listing outgoing
// and incoming links, and the error cases. Each case runs against a
// fresh store holding "docs/a", "docs/b" and "docs/c" with an existing
// labelled link docs/b -> docs/c.
func TestLink(t *testing.T) {
	tests := []struct {
		name       string
		author     string // "" dispatches without an author
		args       []string
		wantText   []string
		wantAbsent []string
		wantData   func(t *testing.T, d any)
		wantErr    error
	}{
		{
			name:     "create link",
			author:   "alice",
			args:     []string{"docs/a", "docs/b"},
			wantText: []string{"Linked docs/a -> docs/b"},
		},
		{
			name:     "create labelled link",
			author:   "alice",
			args:     []string{"--label", "follow-up", "docs/a", "docs/b"},
			wantText: []string{"Linked docs/a -> docs/b"},
		},
		{
			name:     "list outgoing with label",
			author:   "alice",
			args:     []string{"docs/b"},
			wantText: []string{"docs/b -> docs/c (ref)"},
			wantData: func(t *testing.T, d any) {
				ll, ok := d.([]sdk.Link)
				if !ok || len(ll) != 1 || ll[0].From != "docs/b" || ll[0].To != "docs/c" || ll[0].Label != "ref" {
					t.Errorf("link docs/b Data = %#v, want one link docs/b -> docs/c (ref)", d)
				}
			},
		},
		{
			name:       "list outgoing of target is empty",
			author:     "alice",
			args:       []string{"docs/c"},
			wantAbsent: []string{"docs/b"},
			wantData: func(t *testing.T, d any) {
				if ll, _ := d.([]sdk.Link); len(ll) != 0 {
					t.Errorf("link docs/c Data = %#v, want no links", d)
				}
			},
		},
		{
			name:     "list incoming",
			author:   "alice",
			args:     []string{"--in", "docs/c"},
			wantText: []string{"docs/b -> docs/c (ref)"},
		},
		{
			name:       "list incoming of source is empty",
			author:     "alice",
			args:       []string{"--in", "docs/b"},
			wantAbsent: []string{"docs/c"},
		},
		{
			name:    "duplicate link",
			author:  "alice",
			args:    []string{"--label", "ref", "docs/b", "docs/c"},
			wantErr: sdk.ErrExists,
		},
		{
			// Links are keyed by (from, to, label): the same pair with
			// another label is a distinct link, not a duplicate.
			name:     "same pair with different label",
			author:   "alice",
			args:     []string{"docs/b", "docs/c"},
			wantText: []string{"Linked docs/b -> docs/c"},
		},
		{
			name:    "self link",
			author:  "alice",
			args:    []string{"docs/a", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "missing source",
			author:  "alice",
			args:    []string{"docs/missing", "docs/a"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "missing target",
			author:  "alice",
			args:    []string{"docs/a", "docs/missing"},
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
			args:    []string{"--bogus", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:     "both lists incoming and outgoing",
			author:   "alice",
			args:     []string{"--both", "docs/b"},
			wantText: []string{"docs/b -> docs/c (ref)"},
		},
		{
			name:    "label without value",
			author:  "alice",
			args:    []string{"docs/a", "docs/b", "--label"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:     "list without author",
			author:   "",
			args:     []string{"docs/b"},
			wantText: []string{"docs/b -> docs/c (ref)"},
		},
		{
			name:     "list incoming without author",
			author:   "",
			args:     []string{"--in", "docs/c"},
			wantText: []string{"docs/b -> docs/c (ref)"},
		},
		{
			name:    "create without author",
			author:  "",
			args:    []string{"docs/a", "docs/b"},
			wantErr: sdk.ErrMissingArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed := newCLI(t, "alice")
			writeDoc(t, seed, "docs/a", "# A")
			writeDoc(t, seed, "docs/b", "# B")
			writeDoc(t, seed, "docs/c", "# C")
			run(t, seed, "link", nil, "--label", "ref", "docs/b", "docs/c")

			c := seed
			c.Author = tt.author

			if tt.wantErr != nil {
				err := runErr(t, c, "link", nil, tt.args...)
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("link %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "link", nil, tt.args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("link %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("link %v = %q, must not contain %q", tt.args, got, absent)
				}
			}
			if tt.wantData != nil {
				tt.wantData(t, data(resp))
			}
		})
	}
}

// TestLinkRoundTrip checks that a link created by key lists by path
// from both ends, that a plain link has no label suffix, and that a
// label set at creation is reported on listing.
func TestLinkRoundTrip(t *testing.T) {
	c := newCLI(t, "alice")
	writeDoc(t, c, "docs/a", "# A")
	writeDoc(t, c, "docs/b", "# B")
	writeDoc(t, c, "docs/c", "# C")
	docs, err := c.Documents.List("docs/", sdk.ListOpts{})
	if err != nil || len(docs) != 3 {
		t.Fatalf("List(docs/) = %v, %v", docs, err)
	}
	keys := map[string]string{}
	for _, d := range docs {
		keys[d.Path] = d.Key
	}

	run(t, c, "link", nil, keys["docs/a"], keys["docs/b"])
	run(t, c, "link", nil, "--label", "see", "docs/a", "docs/c")

	got := text(run(t, c, "link", nil, keys["docs/a"]))
	for _, want := range []string{"docs/a -> docs/b\n", "docs/a -> docs/c (see)"} {
		if !strings.Contains(got+"\n", want) {
			t.Errorf("link %s = %q, want to contain %q", keys["docs/a"], got, want)
		}
	}
	if in := text(run(t, c, "link", nil, "--in", "docs/b")); in != "docs/a -> docs/b" {
		t.Errorf("link --in docs/b = %q, want %q", in, "docs/a -> docs/b")
	}
	ll, ok := data(run(t, c, "link", nil, "docs/a")).([]sdk.Link)
	if !ok || len(ll) != 2 {
		t.Fatalf("link docs/a Data = %#v, want two links", ll)
	}
}
