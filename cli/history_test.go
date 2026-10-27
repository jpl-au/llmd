package cli

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestHistory covers the version log: the default cap of 10, -n and
// --all, newest-first ordering, messages, key addressing and the
// error cases. Each case runs against a fresh store holding
// "churn/doc" (15 versions, the first with a message) and "docs/one"
// (a single version).
func TestHistory(t *testing.T) {
	tests := []struct {
		name      string
		args      []string // {key} is replaced with the key of churn/doc
		seed      func(t *testing.T, c sdk.Context)
		wantLen   int      // number of versions in Data
		wantFirst int      // Number of the first (newest) version
		wantText  []string // substrings that must appear
		wantErr   error
	}{
		{
			name:      "defaults to 10 versions",
			args:      []string{"churn/doc"},
			wantLen:   10,
			wantFirst: 15,
			wantText:  []string{"VER", "AUTHOR", "DATE", "MESSAGE", "alice"},
		},
		{
			name:      "all returns every version",
			args:      []string{"--all", "churn/doc"},
			wantLen:   15,
			wantFirst: 15,
			wantText:  []string{"first revision"},
		},
		{
			name:      "n limits the count",
			args:      []string{"-n", "3", "churn/doc"},
			wantLen:   3,
			wantFirst: 15,
		},
		{
			name:      "n attached to the flag",
			args:      []string{"-n3", "churn/doc"},
			wantLen:   3,
			wantFirst: 15,
		},
		{
			name:      "n after the path",
			args:      []string{"churn/doc", "--limit", "12"},
			wantLen:   12,
			wantFirst: 15,
		},
		{
			name:      "all beats n",
			args:      []string{"-n", "3", "--all", "churn/doc"},
			wantLen:   15,
			wantFirst: 15,
		},
		{
			name:      "by key",
			args:      []string{"{key}"},
			wantLen:   10,
			wantFirst: 15,
			wantText:  []string{"VER"},
		},
		{
			name:      "single version with message",
			args:      []string{"docs/one"},
			wantLen:   1,
			wantFirst: 1,
			wantText:  []string{"only version"},
		},
		{
			name: "deleted document",
			args: []string{"docs/one"},
			seed: func(t *testing.T, c sdk.Context) {
				run(t, c, "rm", nil, "docs/one")
			},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "missing document",
			args:    []string{"docs/missing"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "no path",
			args:    []string{"--all"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "docs/one"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "n must be an integer",
			args:    []string{"-n", "many", "docs/one"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			run(t, c, "write", []byte("revision 1"), "--message", "first revision", "churn/doc")
			for i := 2; i <= 15; i++ {
				writeDoc(t, c, "churn/doc", "revision "+strconv.Itoa(i))
			}
			run(t, c, "write", []byte("# One"), "--message", "only version", "docs/one")
			if tt.seed != nil {
				tt.seed(t, c)
			}
			args := docArgs(t, c, "churn/doc", tt.args)

			if tt.wantErr != nil {
				if err := runErr(t, c, "history", nil, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("history %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "history", nil, args...)
			if _, ok := resp.(sdk.Result); !ok {
				t.Errorf("history %v response type = %T, want sdk.Result", args, resp)
			}
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("history %v = %q, want to contain %q", args, got, want)
				}
			}

			versions, ok := data(resp).([]sdk.Version)
			if !ok {
				t.Fatalf("history %v Data = %T, want []sdk.Version", args, data(resp))
			}
			if len(versions) != tt.wantLen {
				t.Errorf("history %v returned %d versions, want %d", args, len(versions), tt.wantLen)
			}
			if len(versions) > 0 && versions[0].Number != tt.wantFirst {
				t.Errorf("history %v first version = %d, want %d", args, versions[0].Number, tt.wantFirst)
			}
			for i := 1; i < len(versions); i++ {
				if versions[i].Number != versions[i-1].Number-1 {
					t.Errorf("history %v versions not newest-first and contiguous: %d after %d",
						args, versions[i].Number, versions[i-1].Number)
				}
			}
			for _, v := range versions {
				if v.Author != "alice" || v.CreatedAt == 0 {
					t.Errorf("history %v version %d = %+v, want author alice and a timestamp", args, v.Number, v)
				}
			}
		})
	}
}
