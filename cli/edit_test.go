package cli

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestEdit covers literal search-and-replace: the unique-match
// contract, --all, --message, deletion by replacing with an empty
// string, key addressing, the edit sentinels (no match, not unique,
// no-op) and the argument errors. Each case runs against a fresh store
// holding "docs/a" with the content below.
func TestEdit(t *testing.T) {
	const seed = "# Plan\n\nbuy milk\nbuy bread\nwalk the dog\n"

	tests := []struct {
		name         string
		args         []string // {key} is replaced with the key of docs/a
		author       string   // "-" dispatches with an empty author
		wantText     string
		wantContent  string // content of docs/a afterwards (also checked on error)
		wantVersions int    // history length afterwards (0 = don't check)
		wantMessage  string // message of the newest version
		wantErr      error
	}{
		{
			name:         "unique match is replaced",
			args:         []string{"docs/a", "walk the dog", "feed the cat"},
			wantText:     "Edited docs/a",
			wantContent:  "# Plan\n\nbuy milk\nbuy bread\nfeed the cat\n",
			wantVersions: 2,
		},
		{
			name:         "ambiguous match fails and leaves the document alone",
			args:         []string{"docs/a", "buy", "get"},
			wantErr:      sdk.ErrNotUnique,
			wantContent:  seed,
			wantVersions: 1,
		},
		{
			name:         "all replaces every occurrence",
			args:         []string{"--all", "docs/a", "buy", "get"},
			wantText:     "Edited docs/a",
			wantContent:  "# Plan\n\nget milk\nget bread\nwalk the dog\n",
			wantVersions: 2,
		},
		{
			name:         "all after the positionals",
			args:         []string{"docs/a", "buy", "get", "--all"},
			wantText:     "Edited docs/a",
			wantContent:  "# Plan\n\nget milk\nget bread\nwalk the dog\n",
			wantVersions: 2,
		},
		{
			name:         "expanded context disambiguates",
			args:         []string{"docs/a", "buy milk", "buy oat milk"},
			wantText:     "Edited docs/a",
			wantContent:  "# Plan\n\nbuy oat milk\nbuy bread\nwalk the dog\n",
			wantVersions: 2,
		},
		{
			name:         "empty replacement deletes the match",
			args:         []string{"docs/a", "walk the dog\n", ""},
			wantText:     "Edited docs/a",
			wantContent:  "# Plan\n\nbuy milk\nbuy bread\n",
			wantVersions: 2,
		},
		{
			name:         "message is recorded",
			args:         []string{"--message", "cat not dog", "docs/a", "dog", "cat"},
			wantText:     "Edited docs/a",
			wantContent:  "# Plan\n\nbuy milk\nbuy bread\nwalk the cat\n",
			wantVersions: 2,
			wantMessage:  "cat not dog",
		},
		{
			name:         "by key",
			args:         []string{"{key}", "dog", "cat"},
			wantText:     "Edited ",
			wantContent:  "# Plan\n\nbuy milk\nbuy bread\nwalk the cat\n",
			wantVersions: 2,
		},
		{
			name:         "no match",
			args:         []string{"docs/a", "unicorn", "horse"},
			wantErr:      sdk.ErrNoMatch,
			wantContent:  seed,
			wantVersions: 1,
		},
		{
			name:         "identical old and new",
			args:         []string{"docs/a", "dog", "dog"},
			wantErr:      sdk.ErrNoOp,
			wantContent:  seed,
			wantVersions: 1,
		},
		{
			name:    "missing document",
			args:    []string{"docs/missing", "a", "b"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "two positionals",
			args:    []string{"docs/a", "dog"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "no arguments",
			args:    nil,
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "docs/a", "dog", "cat"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "requires an author",
			args:    []string{"docs/a", "dog", "cat"},
			author:  "-",
			wantErr: sdk.ErrMissingArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/a", seed)
			args := docArgs(t, c, "docs/a", tt.args)

			author := c.Author
			if tt.author == "-" {
				author = ""
			}
			resp, err := docRunAs(t, c, author, "edit", nil, args...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("edit %v error = %v, want %v", args, err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("edit %v: %v", args, err)
			} else {
				if got := text(resp); len(got) < len(tt.wantText) || got[:len(tt.wantText)] != tt.wantText {
					t.Errorf("edit %v = %q, want prefix %q", args, got, tt.wantText)
				}
				if _, ok := resp.(sdk.Text); !ok {
					t.Errorf("edit %v response type = %T, want sdk.Text", args, resp)
				}
			}

			if tt.wantContent != "" {
				if got := docRead(t, c, "docs/a"); got != tt.wantContent {
					t.Errorf("cat docs/a after edit = %q, want %q", got, tt.wantContent)
				}
			}
			if tt.wantVersions != 0 {
				versions := docVersions(t, c, "docs/a")
				if len(versions) != tt.wantVersions {
					t.Errorf("history docs/a has %d versions, want %d", len(versions), tt.wantVersions)
				}
				if tt.wantMessage != "" && versions[0].Message != tt.wantMessage {
					t.Errorf("newest version message = %q, want %q", versions[0].Message, tt.wantMessage)
				}
			}
		})
	}
}
