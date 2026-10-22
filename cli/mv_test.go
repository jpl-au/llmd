package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestMv covers renaming: content and the full version history follow
// the document, the old path stops resolving, keys work as the source,
// and the error cases. Each case runs against a fresh store holding
// "docs/a" (two versions) and "docs/other".
func TestMv(t *testing.T) {
	tests := []struct {
		name       string
		args       []string // {key} is replaced with the key of docs/a
		author     string   // "-" dispatches with an empty author
		wantText   string
		wantErr    error
		wantAnyErr bool   // any error is acceptable (no sentinel is defined)
		skip       string // t.Skip reason for cases that pin a known bug
	}{
		{
			name:     "moves the document with its history",
			args:     []string{"docs/a", "docs/b"},
			wantText: "Moved docs/a -> docs/b",
		},
		{
			name:     "moves into a deeper hierarchy",
			args:     []string{"docs/a", "projects/site/notes/b"},
			wantText: "Moved docs/a -> projects/site/notes/b",
		},
		{
			name:     "by key",
			args:     []string{"{key}", "docs/b"},
			wantText: "-> docs/b",
		},
		{
			name:    "missing source",
			args:    []string{"docs/missing", "docs/b"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:       "destination already exists",
			args:       []string{"docs/a", "docs/other"},
			wantAnyErr: true,
		},
		{
			name:    "destination already exists is ErrExists",
			args:    []string{"docs/a", "docs/other"},
			wantErr: sdk.ErrExists,
		},
		{
			name:       "invalid destination",
			args:       []string{"docs/a", "../escape"},
			wantAnyErr: true,
		},
		{
			name:    "one argument",
			args:    []string{"docs/a"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "no arguments",
			args:    nil,
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "requires an author",
			args:    []string{"docs/a", "docs/b"},
			author:  "-",
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "docs/a", "docs/b"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/a", "# A v1")
			writeDoc(t, c, "docs/a", "# A v2")
			writeDoc(t, c, "docs/other", "# Other")
			args := docArgs(t, c, "docs/a", tt.args)

			author := c.Author
			if tt.author == "-" {
				author = ""
			}
			resp, err := docRunAs(t, c, author, "mv", nil, args...)
			if tt.wantAnyErr {
				if err == nil {
					t.Errorf("mv %v error = nil, want an error", args)
				}
				return
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("mv %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("mv %v: %v", args, err)
			}
			if got := text(resp); !strings.HasSuffix(got, tt.wantText) {
				t.Errorf("mv %v = %q, want suffix %q", args, got, tt.wantText)
			}

			to := args[1]
			if got := docRead(t, c, to); got != "# A v2" {
				t.Errorf("cat %s after mv = %q, want %q", to, got, "# A v2")
			}
			if err := runErr(t, c, "cat", nil, "docs/a"); !errors.Is(err, sdk.ErrNotFound) {
				t.Errorf("cat docs/a after mv error = %v, want %v", err, sdk.ErrNotFound)
			}
			if versions := docVersions(t, c, to); len(versions) != 2 {
				t.Errorf("history %s after mv has %d versions, want 2", to, len(versions))
			}
			if got := text(run(t, c, "ls", nil)); !strings.Contains(got, to) || strings.Contains(got, "docs/a") {
				t.Errorf("ls after mv = %q, want %s and not docs/a", got, to)
			}
		})
	}
}
