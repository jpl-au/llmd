package cli

import (
	"errors"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestSed covers s/old/new/ substitution: the unique-match default,
// the trailing g flag, alternate delimiters, the ignored -i flag,
// expression parse errors and the argument errors. Each case runs
// against a fresh store holding "docs/a" with the content below.
func TestSed(t *testing.T) {
	const seed = "# Notes\n\nfoo one\nfoo two\n/usr/local/bin\n"

	tests := []struct {
		name         string
		args         []string
		author       string // "-" dispatches with an empty author
		wantText     string
		wantContent  string // content of docs/a afterwards (also checked on error)
		wantVersions int    // history length afterwards (0 = don't check)
		wantErr      error
	}{
		{
			name:         "unique substitution",
			args:         []string{"s/foo one/bar one/", "docs/a"},
			wantText:     "Edited docs/a",
			wantContent:  "# Notes\n\nbar one\nfoo two\n/usr/local/bin\n",
			wantVersions: 2,
		},
		{
			name:         "-i is accepted and ignored",
			args:         []string{"-i", "s/foo one/bar one/", "docs/a"},
			wantText:     "Edited docs/a",
			wantContent:  "# Notes\n\nbar one\nfoo two\n/usr/local/bin\n",
			wantVersions: 2,
		},
		{
			name:         "-i between expression and path",
			args:         []string{"s/foo one/bar one/", "-i", "docs/a"},
			wantText:     "Edited docs/a",
			wantContent:  "# Notes\n\nbar one\nfoo two\n/usr/local/bin\n",
			wantVersions: 2,
		},
		{
			name:         "ambiguous match without g fails",
			args:         []string{"s/foo/bar/", "docs/a"},
			wantErr:      sdk.ErrNotUnique,
			wantContent:  seed,
			wantVersions: 1,
		},
		{
			name:         "g flag replaces every occurrence",
			args:         []string{"s/foo/bar/g", "docs/a"},
			wantText:     "Edited docs/a",
			wantContent:  "# Notes\n\nbar one\nbar two\n/usr/local/bin\n",
			wantVersions: 2,
		},
		{
			name:         "alternate delimiter",
			args:         []string{"s|/usr/local|/opt|", "docs/a"},
			wantText:     "Edited docs/a",
			wantContent:  "# Notes\n\nfoo one\nfoo two\n/opt/bin\n",
			wantVersions: 2,
		},
		{
			name:         "missing closing delimiter is tolerated",
			args:         []string{"s/foo two/bar two", "docs/a"},
			wantText:     "Edited docs/a",
			wantContent:  "# Notes\n\nfoo one\nbar two\n/usr/local/bin\n",
			wantVersions: 2,
		},
		{
			name:         "empty replacement deletes the match",
			args:         []string{"s/foo one\n//", "docs/a"},
			wantText:     "Edited docs/a",
			wantContent:  "# Notes\n\nfoo two\n/usr/local/bin\n",
			wantVersions: 2,
		},
		{
			name:         "no match",
			args:         []string{"s/unicorn/horse/", "docs/a"},
			wantErr:      sdk.ErrNoMatch,
			wantContent:  seed,
			wantVersions: 1,
		},
		{
			name:         "identical old and new",
			args:         []string{"s/foo one/foo one/", "docs/a"},
			wantErr:      sdk.ErrNoOp,
			wantContent:  seed,
			wantVersions: 1,
		},
		{
			name:    "expression must start with s",
			args:    []string{"y/foo/bar/", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "expression too short",
			args:    []string{"s/a", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "missing delimiter after pattern",
			args:    []string{"s/foo one", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "empty search pattern",
			args:    []string{"s//bar/", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "s/foo one/bar/", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "missing document",
			args:    []string{"s/foo one/bar/", "docs/missing"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "expression only",
			args:    []string{"s/foo one/bar/"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "-i only",
			args:    []string{"-i"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "no arguments",
			args:    nil,
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "requires an author",
			args:    []string{"s/foo one/bar/", "docs/a"},
			author:  "-",
			wantErr: sdk.ErrMissingArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/a", seed)

			author := c.Author
			if tt.author == "-" {
				author = ""
			}
			resp, err := docRunAs(t, c, author, "sed", nil, tt.args...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("sed %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("sed %v: %v", tt.args, err)
			} else if got := text(resp); got != tt.wantText {
				t.Errorf("sed %v = %q, want %q", tt.args, got, tt.wantText)
			}

			if tt.wantContent != "" {
				if got := docRead(t, c, "docs/a"); got != tt.wantContent {
					t.Errorf("cat docs/a after sed = %q, want %q", got, tt.wantContent)
				}
			}
			if tt.wantVersions != 0 {
				if versions := docVersions(t, c, "docs/a"); len(versions) != tt.wantVersions {
					t.Errorf("history docs/a has %d versions, want %d", len(versions), tt.wantVersions)
				}
			}
		})
	}
}
