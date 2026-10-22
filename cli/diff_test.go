package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestDiff covers the unified diff: latest-vs-previous with one path,
// explicit versions, key:version, two documents, -C context, --stat,
// the 500-line cap with --all to lift it, and the error cases. Each
// case runs against a fresh store holding "docs/a" at three versions
// (ten numbered lines; v2 changes line 5, v3 changes it again and
// appends line 11), "docs/b" and "docs/single" (one version).
func TestDiff(t *testing.T) {
	numbered := func(n int) []string {
		lines := make([]string, n)
		for i := range lines {
			lines[i] = "line " + strconv.Itoa(i+1)
		}
		return lines
	}
	v1 := numbered(10)
	v2 := numbered(10)
	v2[4] = "line five"
	v3 := numbered(11)
	v3[4] = "line FIVE"

	// big writes a 600-line document and then rewrites every line, so
	// the diff runs to about 1200 lines and trips the default cap.
	big := func(t *testing.T, c sdk.Context) {
		var sb strings.Builder
		for i := range 600 {
			fmt.Fprintf(&sb, "row %d\n", i)
		}
		writeDoc(t, c, "docs/big", sb.String())
		writeDoc(t, c, "docs/big", strings.ReplaceAll(sb.String(), "row", "line"))
	}

	tests := []struct {
		name       string
		args       []string // {key} is replaced with the key of docs/a
		seed       func(t *testing.T, c sdk.Context)
		wantText   []string // substrings that must appear
		wantAbsent []string // substrings that must not appear
		wantExact  string   // full response text, when deterministic
		wantLines  int      // newline count of the response (0 = don't check)
		wantErr    error
		wantAnyErr bool   // any error is acceptable (no sentinel is defined)
		skip       string // t.Skip reason for cases that pin a known bug
	}{
		{
			name:       "latest against previous",
			args:       []string{"docs/a"},
			wantText:   []string{"--- docs/a:2", "+++ docs/a:3", "@@", "-line five", "+line FIVE", "+line 11"},
			wantAbsent: []string{"truncated"},
		},
		{
			name:     "explicit versions",
			args:     []string{"docs/a:1", "docs/a:3"},
			wantText: []string{"--- docs/a:1", "+++ docs/a:3", "-line 5", "+line FIVE"},
		},
		{
			name:     "by key and version",
			args:     []string{"{key}:1", "{key}:2"},
			wantText: []string{"@@", "-line 5", "+line five"},
		},
		{
			name:     "key against path",
			args:     []string{"{key}:1", "docs/a"},
			wantText: []string{"+++ docs/a:3", "+line FIVE"},
		},
		{
			name:     "two documents",
			args:     []string{"docs/a", "docs/b"},
			wantText: []string{"--- docs/a:3", "+++ docs/b:1", "+# B", "-line 1"},
		},
		{
			// Three lines of context by default: lines 2-4 and 6-8
			// surround the change on line 5.
			name:       "default context",
			args:       []string{"docs/a:1", "docs/a:2"},
			wantText:   []string{"line 2", "line 8"},
			wantAbsent: []string{"line 1\n", "line 9"},
		},
		{
			name:       "context attached to the flag",
			args:       []string{"-C1", "docs/a:1", "docs/a:2"},
			wantText:   []string{"line 4", "line 6"},
			wantAbsent: []string{"line 3", "line 7"},
		},
		{
			name:       "context as a separate value",
			args:       []string{"docs/a:1", "docs/a:2", "-C", "1"},
			wantText:   []string{"line 4", "line 6"},
			wantAbsent: []string{"line 3", "line 7"},
		},
		{
			name:      "stat only",
			args:      []string{"--stat", "docs/a:1", "docs/a:2"},
			wantExact: "+1 -1",
		},
		{
			name:      "stat with one path",
			args:      []string{"--stat", "docs/a"},
			wantExact: "+2 -1",
		},
		{
			name:      "identical versions",
			args:      []string{"docs/a:1", "docs/a:1"},
			wantExact: "No differences",
		},
		{
			name:       "large diff is capped at 500 lines",
			args:       []string{"docs/big"},
			seed:       big,
			wantText:   []string{"-row 0", "more lines truncated (use --all to show full diff, +600 -600 total)"},
			wantAbsent: []string{"-row 599", "+line 599"},
			wantLines:  500,
		},
		{
			name:       "all lifts the cap",
			args:       []string{"--all", "docs/big"},
			seed:       big,
			wantText:   []string{"-row 0", "-row 599", "+line 599"},
			wantAbsent: []string{"truncated"},
		},
		{
			name:       "single version has no previous",
			args:       []string{"docs/single"},
			wantAnyErr: true,
		},
		{
			name:       "missing document",
			args:       []string{"docs/missing"},
			wantAnyErr: true,
		},
		{
			name:    "missing document is ErrNotFound",
			args:    []string{"docs/missing"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "missing target",
			args:    []string{"docs/a", "docs/missing"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "missing version",
			args:    []string{"docs/a:1", "docs/a:9"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "no path",
			args:    []string{"--stat"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "three paths",
			args:    []string{"docs/a", "docs/b", "docs/single"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "context must be an integer",
			args:    []string{"-C", "lots", "docs/a"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/a", strings.Join(v1, "\n")+"\n")
			writeDoc(t, c, "docs/a", strings.Join(v2, "\n")+"\n")
			writeDoc(t, c, "docs/a", strings.Join(v3, "\n")+"\n")
			writeDoc(t, c, "docs/b", "# B")
			writeDoc(t, c, "docs/single", "# Single")
			if tt.seed != nil {
				tt.seed(t, c)
			}
			args := docArgs(t, c, "docs/a", tt.args)

			if tt.wantAnyErr {
				if err := runErr(t, c, "diff", nil, args...); err == nil {
					t.Errorf("diff %v error = nil, want an error", args)
				}
				return
			}
			if tt.wantErr != nil {
				if err := runErr(t, c, "diff", nil, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("diff %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "diff", nil, args...)
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("diff %v response type = %T, want sdk.Text", args, resp)
			}
			got := text(resp)
			if tt.wantExact != "" && got != tt.wantExact {
				t.Errorf("diff %v = %q, want %q", args, got, tt.wantExact)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("diff %v = %q, want to contain %q", args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("diff %v = %q, must not contain %q", args, got, absent)
				}
			}
			if tt.wantLines != 0 && strings.Count(got, "\n") != tt.wantLines {
				t.Errorf("diff %v has %d newlines, want %d", args, strings.Count(got, "\n"), tt.wantLines)
			}
		})
	}
}

// TestDiffTruncate pins the helper behind the 500-line cap: a diff at
// or under the cap is returned unchanged (same string, so the caller
// can detect the no-op by identity), a longer one is cut to max lines
// and reports the original count.
func TestDiffTruncate(t *testing.T) {
	tests := []struct {
		name      string
		lines     int
		max       int
		wantLines int
		wantTotal int
		wantSame  bool
	}{
		{name: "under the cap", lines: 3, max: 5, wantLines: 3, wantTotal: 3, wantSame: true},
		{name: "at the cap", lines: 5, max: 5, wantLines: 5, wantTotal: 5, wantSame: true},
		{name: "over the cap", lines: 8, max: 5, wantLines: 5, wantTotal: 8},
		// strings.Split("") is one empty element, so an empty diff
		// counts as a single line.
		{name: "empty", lines: 0, max: 5, wantLines: 1, wantTotal: 1, wantSame: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := make([]string, tt.lines)
			for i := range lines {
				lines[i] = "l" + strconv.Itoa(i)
			}
			in := strings.Join(lines, "\n")
			got, total := truncateDiff(in, tt.max)
			if total != tt.wantTotal {
				t.Errorf("truncateDiff(%d lines, %d) total = %d, want %d", tt.lines, tt.max, total, tt.wantTotal)
			}
			if (got == in) != tt.wantSame {
				t.Errorf("truncateDiff(%d lines, %d) unchanged = %v, want %v", tt.lines, tt.max, got == in, tt.wantSame)
			}
			if n := len(strings.Split(got, "\n")); n != tt.wantLines {
				t.Errorf("truncateDiff(%d lines, %d) returned %d lines, want %d", tt.lines, tt.max, n, tt.wantLines)
			}
		})
	}
}

// TestDiffColour checks that terminal colouring keeps every line of a
// unified diff intact and in order; styling may add escape codes but
// must never drop or reorder content.
func TestDiffColour(t *testing.T) {
	in := "--- a:1\n+++ a:2\n@@ -1,2 +1,2 @@\n context\n-removed\n+added"
	got := colourDiff(in)
	lines := strings.Split(in, "\n")
	if n := len(strings.Split(got, "\n")); n != len(lines) {
		t.Fatalf("colourDiff produced %d lines, want %d: %q", n, len(lines), got)
	}
	pos := 0
	for _, line := range lines {
		i := strings.Index(got[pos:], line)
		if i < 0 {
			t.Errorf("colourDiff output %q lost or reordered %q", got, line)
			continue
		}
		pos += i + len(line)
	}
	if colourDiff("") != "" {
		t.Errorf("colourDiff(\"\") = %q, want empty", colourDiff(""))
	}
}
