package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestRm covers soft deletion: the document disappears from cat and
// ls but stays visible to ls -a, keys work as identifiers, and the
// error cases. Each case runs against a fresh store holding "docs/a"
// and "docs/b".
func TestRm(t *testing.T) {
	tests := []struct {
		name     string
		args     []string // {key} is replaced with the key of docs/a
		author   string   // "-" dispatches with an empty author
		seed     func(t *testing.T, c sdk.Context)
		wantText string
		wantErr  error
		skip     string // t.Skip reason for cases that pin a known bug
	}{
		{
			name:     "deletes the document",
			args:     []string{"docs/a"},
			wantText: "Deleted docs/a",
		},
		{
			name:     "by key",
			args:     []string{"{key}"},
			wantText: "Deleted ",
		},
		{
			name:    "missing document",
			args:    []string{"docs/missing"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name: "already deleted",
			args: []string{"docs/a"},
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "rm", nil, "docs/a")
			},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "no path",
			args:    nil,
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "requires an author",
			args:    []string{"docs/a"},
			author:  "-",
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/a", "# A")
			writeDoc(t, c, "docs/b", "# B")
			if tt.seed != nil {
				tt.seed(t, c)
			}
			args := docArgs(t, c, "docs/a", tt.args)

			author := c.Author
			if tt.author == "-" {
				author = ""
			}
			resp, err := docRunAs(t, c, author, "rm", nil, args...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("rm %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("rm %v: %v", args, err)
			}
			if got := text(resp); !strings.HasPrefix(got, tt.wantText) {
				t.Errorf("rm %v = %q, want prefix %q", args, got, tt.wantText)
			}

			// The document is gone from cat and the default listing
			// but still listed, flagged deleted, with -a. Its sibling
			// is untouched. (cat reports "document is deleted", which
			// is not mapped to an SDK sentinel, so only failure is
			// asserted here.)
			if err := runErr(t, c, "cat", nil, "docs/a"); err == nil {
				t.Errorf("cat docs/a after rm error = nil, want an error")
			}
			if got := text(run(t, c, "ls", nil)); strings.Contains(got, "docs/a") || !strings.Contains(got, "docs/b") {
				t.Errorf("ls after rm = %q, want docs/b only", got)
			}
			if got := text(run(t, c, "ls", nil, "-la")); !strings.Contains(got, "docs/a (deleted)") {
				t.Errorf("ls -la after rm = %q, want to contain %q", got, "docs/a (deleted)")
			}
			if got := docRead(t, c, "docs/b"); got != "# B" {
				t.Errorf("cat docs/b after rm docs/a = %q, want %q", got, "# B")
			}
		})
	}
}
