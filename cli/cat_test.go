package cli

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestCat covers whole-document reads, the offset/limit window with
// stable line numbers, version and key addressing, and the error
// cases. Each case runs against a fresh store holding "docs/long"
// (ten numbered lines, two versions) and "docs/short".
func TestCat(t *testing.T) {
	long := func(n int) string {
		lines := make([]string, n)
		for i := range lines {
			lines[i] = "line " + strconv.Itoa(i+1)
		}
		return strings.Join(lines, "\n")
	}

	tests := []struct {
		name         string
		args         []string // use {key} / {key:1} for the key of docs/long
		wantText     []string // substrings that must appear
		wantAbsent   []string // substrings that must not appear
		wantMarkdown bool     // response is sdk.Markdown (rendered for humans)
		wantErr      error
	}{
		{
			name:         "whole document",
			args:         []string{"docs/short"},
			wantText:     []string{"# Short"},
			wantMarkdown: true,
		},
		{
			name:         "limit caps from the top",
			args:         []string{"--limit", "3", "docs/long"},
			wantText:     []string{"line 1", "line 3"},
			wantAbsent:   []string{"line 4"},
			wantMarkdown: true,
		},
		{
			name:         "offset and limit return a window",
			args:         []string{"--offset", "5", "--limit", "2", "docs/long"},
			wantText:     []string{"line 5", "line 6"},
			wantAbsent:   []string{"line 4", "line 7"},
			wantMarkdown: true,
		},
		{
			// -n numbers lines from the source, not from the slice,
			// and ships as Result because numbered text is not markdown.
			name:       "numbered with offset keeps source line numbers",
			args:       []string{"--offset", "5", "--limit", "2", "-n", "docs/long"},
			wantText:   []string{"5  line 5", "6  line 6"},
			wantAbsent: []string{"1  line"},
		},
		{
			name:         "specific version",
			args:         []string{"--version", "1", "docs/long"},
			wantText:     []string{"line 10"},
			wantAbsent:   []string{"line 11"},
			wantMarkdown: true,
		},
		{
			name:         "by key",
			args:         []string{"{key}"},
			wantText:     []string{"line 11"},
			wantMarkdown: true,
		},
		{
			name:         "by key and version",
			args:         []string{"{key:1}"},
			wantText:     []string{"line 10"},
			wantAbsent:   []string{"line 11"},
			wantMarkdown: true,
		},
		{
			name:         "multiple paths are concatenated",
			args:         []string{"docs/short", "docs/long"},
			wantText:     []string{"# Short", "line 1"},
			wantMarkdown: true,
		},
		{
			name:    "missing path",
			args:    []string{"docs/missing"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "no path",
			args:    nil,
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "docs/short"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/short", "# Short")
			writeDoc(t, c, "docs/long", long(10))
			writeDoc(t, c, "docs/long", long(11))
			docs, err := c.Documents.List("docs/long", sdk.ListOpts{})
			if err != nil || len(docs) == 0 {
				t.Fatalf("List(docs/long) = %v, %v", docs, err)
			}
			key := docs[0].Key

			args := make([]string, len(tt.args))
			for i, a := range tt.args {
				args[i] = strings.NewReplacer("{key:1}", key+":1", "{key}", key).Replace(a)
			}

			if tt.wantErr != nil {
				if err := runErr(t, c, "cat", nil, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("cat %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "cat", nil, args...)
			if _, ok := resp.(sdk.Markdown); ok != tt.wantMarkdown {
				t.Errorf("cat %v response type = %T, want markdown %v", args, resp, tt.wantMarkdown)
			}
			got := text(resp)
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("cat %v = %q, want to contain %q", args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("cat %v = %q, must not contain %q", args, got, absent)
				}
			}
			if d, ok := data(resp).(string); !ok || d != got {
				t.Errorf("cat %v Data = %v, want the same text as the response", args, data(resp))
			}
		})
	}
}
