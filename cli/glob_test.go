package cli

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestGlob covers path matching: * within a segment, ** across
// segments, ?, a suffix pattern, the empty result, --limit/--all and
// the error cases. Each case runs against a fresh store holding the
// documents listed in seed; glob results are in path order.
func TestGlob(t *testing.T) {
	seed := []string{
		"docs/y-draft",
		"notes/a",
		"notes/b",
		"notes/meeting-1",
		"notes/meeting-2",
		"notes/sub/deep",
		"notes/sub/x-draft",
		"top",
	}

	tests := []struct {
		name       string
		args       []string
		want       []string // exact paths in order
		wantErr    error
		wantAnyErr bool // any error is acceptable (no sentinel is defined)
	}{
		{
			name: "star matches one level",
			args: []string{"notes/*"},
			want: []string{"notes/a", "notes/b", "notes/meeting-1", "notes/meeting-2"},
		},
		{
			name: "double star matches recursively",
			args: []string{"notes/**"},
			want: []string{"notes/a", "notes/b", "notes/meeting-1", "notes/meeting-2", "notes/sub/deep", "notes/sub/x-draft"},
		},
		{
			name: "question mark matches one character",
			args: []string{"notes/meeting-?"},
			want: []string{"notes/meeting-1", "notes/meeting-2"},
		},
		{
			name: "suffix across segments",
			args: []string{"**/*-draft"},
			want: []string{"docs/y-draft", "notes/sub/x-draft"},
		},
		{
			name: "bare star matches top-level documents only",
			args: []string{"*"},
			want: []string{"top"},
		},
		{
			name: "exact path",
			args: []string{"notes/a"},
			want: []string{"notes/a"},
		},
		{
			name: "no match",
			args: []string{"zzz/*"},
			want: []string{},
		},
		{
			name: "limit caps the matches",
			args: []string{"--limit", "2", "notes/**"},
			want: []string{"notes/a", "notes/b"},
		},
		{
			name: "all beats limit",
			args: []string{"--limit", "2", "--all", "notes/*"},
			want: []string{"notes/a", "notes/b", "notes/meeting-1", "notes/meeting-2"},
		},
		{
			name: "deleted documents are excluded",
			args: []string{"notes/sub/*"},
			want: []string{"notes/sub/x-draft"},
		},
		{
			name:       "invalid pattern",
			args:       []string{"notes/["},
			wantAnyErr: true,
		},
		{
			name:    "no pattern",
			args:    []string{"--all"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "notes/*"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "limit must be an integer",
			args:    []string{"--limit", "some", "notes/*"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			for _, p := range seed {
				writeDoc(t, c, p, "# "+p)
			}
			if tt.name == "deleted documents are excluded" {
				run(t, c, "rm", nil, "notes/sub/deep")
			}

			if tt.wantAnyErr {
				if err := runErr(t, c, "glob", nil, tt.args...); err == nil {
					t.Errorf("glob %v error = nil, want an error", tt.args)
				}
				return
			}
			if tt.wantErr != nil {
				if err := runErr(t, c, "glob", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("glob %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "glob", nil, tt.args...)
			if _, ok := resp.(sdk.Result); !ok {
				t.Errorf("glob %v response type = %T, want sdk.Result", tt.args, resp)
			}
			if got := text(resp); got != strings.Join(tt.want, "\n") {
				t.Errorf("glob %v = %q, want %q", tt.args, got, strings.Join(tt.want, "\n"))
			}
			paths, ok := data(resp).([]string)
			if !ok {
				t.Fatalf("glob %v Data = %T, want []string", tt.args, data(resp))
			}
			if !slices.Equal(paths, tt.want) {
				t.Errorf("glob %v Data = %v, want %v", tt.args, paths, tt.want)
			}
		})
	}
}
