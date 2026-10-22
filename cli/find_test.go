package cli

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestFind covers path-only full-text search: bare words, the path
// prefix, FTS5 operators (OR, phrases, prefix wildcards), literal
// hyphenated terms, the empty result, --limit/--all and the error
// cases. Each case runs against a fresh store holding four documents.
func TestFind(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []string // matching paths, order-insensitive
		wantErr error
	}{
		{
			name: "word matches every document containing it",
			args: []string{"postgres"},
			want: []string{"docs/db", "notes/db"},
		},
		{
			name: "path prefix restricts the search",
			args: []string{"postgres", "notes/"},
			want: []string{"notes/db"},
		},
		{
			name: "prefix with no matches",
			args: []string{"postgres", "nothing/"},
			want: []string{},
		},
		{
			name: "boolean OR",
			args: []string{"postgres OR mysql"},
			want: []string{"docs/db", "docs/web", "notes/db"},
		},
		{
			name: "implicit AND",
			args: []string{"postgres tuning"},
			want: []string{"notes/db"},
		},
		{
			name: "exact phrase",
			args: []string{`"error handling"`},
			want: []string{"docs/db"},
		},
		{
			name: "prefix wildcard",
			args: []string{"deploy*"},
			want: []string{"docs/web"},
		},
		{
			name: "hyphenated literal term",
			args: []string{"foo-bar"},
			want: []string{"notes/hyphen"},
		},
		{
			name: "no match",
			args: []string{"unicorn"},
			want: []string{},
		},
		{
			name: "limit caps the paths",
			args: []string{"--limit", "1", "postgres OR mysql"},
			want: []string{"?"},
		},
		{
			name: "all beats limit",
			args: []string{"--limit", "1", "--all", "postgres"},
			want: []string{"docs/db", "notes/db"},
		},
		{
			name:    "no query",
			args:    []string{"--all"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "postgres"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "limit must be an integer",
			args:    []string{"--limit", "some", "postgres"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/db", "# Database\n\nPostgres handles error handling well.\n")
			writeDoc(t, c, "docs/web", "# Web\n\nDeploying the frontend uses mysql.\n")
			writeDoc(t, c, "notes/db", "# Notes\n\nPostgres tuning notes.\n")
			writeDoc(t, c, "notes/hyphen", "# Notes\n\nfoo-bar baz\n")

			if tt.wantErr != nil {
				if err := runErr(t, c, "find", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("find %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "find", nil, tt.args...)
			if _, ok := resp.(sdk.Result); !ok {
				t.Errorf("find %v response type = %T, want sdk.Result", tt.args, resp)
			}
			paths, ok := data(resp).([]string)
			if !ok {
				t.Fatalf("find %v Data = %T, want []string", tt.args, data(resp))
			}
			if got := text(resp); got != strings.Join(paths, "\n") {
				t.Errorf("find %v = %q, want one Data path per line %v", tt.args, got, paths)
			}
			if len(tt.want) == 1 && tt.want[0] == "?" {
				// A capped result: only the count is deterministic.
				if len(paths) != 1 {
					t.Errorf("find %v returned %v, want exactly one path", tt.args, paths)
				}
				return
			}
			got := slices.Clone(paths)
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("find %v = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
