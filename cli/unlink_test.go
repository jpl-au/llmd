package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestUnlink covers removing a link and the error cases. Each case
// runs against a fresh store holding "docs/a" linked to "docs/b".
func TestUnlink(t *testing.T) {
	tests := []struct {
		name     string
		author   string // "" dispatches without an author
		args     []string
		wantText []string
		wantErr  error
	}{
		{
			name:     "remove link",
			author:   "alice",
			args:     []string{"docs/a", "docs/b"},
			wantText: []string{"Unlinked docs/a -> docs/b"},
		},
		{
			name:    "reverse direction is not linked",
			author:  "alice",
			args:    []string{"docs/b", "docs/a"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "missing document",
			author:  "alice",
			args:    []string{"docs/a", "docs/missing"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "one argument",
			author:  "alice",
			args:    []string{"docs/a"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "no arguments",
			author:  "alice",
			args:    nil,
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "without author",
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
			run(t, seed, "link", nil, "docs/a", "docs/b")

			c := seed
			c.Author = tt.author

			if tt.wantErr != nil {
				if err := runErr(t, c, "unlink", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("unlink %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				// A failed unlink leaves the link in place.
				if out := text(run(t, seed, "link", nil, "docs/a")); out != "docs/a -> docs/b" {
					t.Errorf("link docs/a after failed unlink = %q, want %q", out, "docs/a -> docs/b")
				}
				return
			}
			resp := run(t, c, "unlink", nil, tt.args...)
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("unlink %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			// The link is gone from both ends.
			if out := text(run(t, seed, "link", nil, "docs/a")); out != "" {
				t.Errorf("link docs/a after unlink = %q, want empty", out)
			}
			if in := text(run(t, seed, "link", nil, "--in", "docs/b")); in != "" {
				t.Errorf("link --in docs/b after unlink = %q, want empty", in)
			}
			// Unlinking twice is an error.
			if err := runErr(t, c, "unlink", nil, tt.args...); !errors.Is(err, sdk.ErrNotFound) {
				t.Errorf("second unlink %v error = %v, want %v", tt.args, err, sdk.ErrNotFound)
			}
		})
	}
}

// TestUnlinkByKey checks that unlink resolves document keys on both
// ends like link does.
func TestUnlinkByKey(t *testing.T) {
	c := newCLI(t, "alice")
	writeDoc(t, c, "docs/a", "# A")
	writeDoc(t, c, "docs/b", "# B")
	run(t, c, "link", nil, "docs/a", "docs/b")
	docs, err := c.Documents.List("docs/", sdk.ListOpts{})
	if err != nil || len(docs) != 2 {
		t.Fatalf("List(docs/) = %v, %v", docs, err)
	}
	keys := map[string]string{}
	for _, d := range docs {
		keys[d.Path] = d.Key
	}

	got := text(run(t, c, "unlink", nil, keys["docs/a"], keys["docs/b"]))
	if !strings.HasPrefix(got, "Unlinked ") {
		t.Errorf("unlink by key = %q, want Unlinked prefix", got)
	}
	if out := text(run(t, c, "link", nil, "docs/a")); out != "" {
		t.Errorf("link docs/a after unlink by key = %q, want empty", out)
	}
}
