package cli

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/clock/clocktest"
	"github.com/jpl-au/llmd/sdk"
)

// TestGraph covers the store timeline and single-document lineage as
// they appear when stdout is not a terminal (plain lines), the
// structured payload, the empty store and the error cases. A
// deterministic clock fixes the event order: docs/a v1, docs/b v1,
// docs/a v2, docs/gone v1 (deleted afterwards).
func TestGraph(t *testing.T) {
	tests := []struct {
		name      string
		args      []string // {key} is replaced with the key of docs/a
		empty     bool     // run against an empty store
		wantText  string   // exact response text
		wantPaths []string // Data "path" fields in order
		wantVers  []int    // Data "version" fields in order
		wantErr   error
		skip      string // t.Skip reason for cases that pin a known bug
	}{
		{
			name:      "timeline lists every version chronologically",
			args:      nil,
			wantText:  "docs/a (v1)\ndocs/b (v1)\ndocs/a (v2)",
			wantPaths: []string{"docs/a", "docs/b", "docs/a"},
			wantVers:  []int{1, 1, 2},
		},
		{
			name:      "lineage lists a document's versions oldest first",
			args:      []string{"docs/a"},
			wantText:  "(v1) {ts} alice\n(v2) {ts} alice",
			wantPaths: []string{"docs/a", "docs/a"},
			wantVers:  []int{1, 2},
		},
		{
			name:      "lineage by key",
			args:      []string{"{key}"},
			wantText:  "(v1) {ts} alice\n(v2) {ts} alice",
			wantPaths: []string{"docs/a", "docs/a"},
			wantVers:  []int{1, 2},
		},
		{
			name:      "empty store",
			args:      nil,
			empty:     true,
			wantText:  "No events",
			wantPaths: []string{},
			wantVers:  []int{},
		},
		{
			name:    "missing document",
			args:    []string{"docs/missing"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "deleted document",
			args:    []string{"docs/gone"},
			wantErr: sdk.ErrNotFound,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			clocktest.Sequence(t, 1_000, 1)
			c := newCLI(t, "alice")
			if !tt.empty {
				writeDoc(t, c, "docs/a", "a1")
				writeDoc(t, c, "docs/b", "b1")
				writeDoc(t, c, "docs/a", "a2")
				writeDoc(t, c, "docs/gone", "gone")
				run(t, c, "rm", nil, "docs/gone")
			}
			var args []string
			if !tt.empty {
				args = docArgs(t, c, "docs/a", tt.args)
			} else {
				args = tt.args
			}

			if tt.wantErr != nil {
				if err := runErr(t, c, "graph", nil, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("graph %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "graph", nil, args...)
			if _, ok := resp.(sdk.Result); !ok {
				t.Errorf("graph %v response type = %T, want sdk.Result", args, resp)
			}

			// Lineage lines carry a local-time timestamp; compare the
			// text with the timestamp field blanked out.
			got := text(resp)
			if strings.Contains(tt.wantText, "{ts}") {
				var lines []string
				for line := range strings.SplitSeq(got, "\n") {
					f := strings.Fields(line)
					if len(f) != 6 {
						t.Fatalf("graph %v line %q, want \"(vN) DD Mon YYYY HH:MM author\"", args, line)
					}
					lines = append(lines, f[0]+" {ts} "+f[5])
				}
				got = strings.Join(lines, "\n")
			}
			if got != tt.wantText {
				t.Errorf("graph %v = %q, want %q", args, got, tt.wantText)
			}

			var paths []string
			var vers []int
			var last int64
			switch d := data(resp).(type) {
			case []map[string]any:
				for _, m := range d {
					path, _ := m["path"].(string)
					ver, _ := m["version"].(int)
					at, _ := m["created_at"].(int64)
					paths = append(paths, path)
					vers = append(vers, ver)
					if m["author"] != "alice" || at < last {
						t.Errorf("graph %v Data entry %v, want author alice and non-decreasing created_at", args, m)
					}
					last = at
				}
			case []graphEvent:
				if len(d) != 0 {
					t.Errorf("graph %v Data = %v, want empty", args, d)
				}
			default:
				t.Fatalf("graph %v Data = %T, want []map[string]any", args, data(resp))
			}
			if !slices.Equal(paths, tt.wantPaths) || !slices.Equal(vers, tt.wantVers) {
				t.Errorf("graph %v Data paths/versions = %v/%v, want %v/%v", args, paths, vers, tt.wantPaths, tt.wantVers)
			}
		})
	}
}

// TestGraphBursts exercises the terminal-only clustering and tree
// rendering directly, since graph prints flat lines when stdout is
// not a terminal.
func TestGraphBursts(t *testing.T) {
	ev := func(path string, v int, at int64) graphEvent {
		return graphEvent{Path: path, Version: v, Author: "alice", CreatedAt: at}
	}
	tests := []struct {
		name   string
		events []graphEvent
		want   []int
	}{
		{
			name:   "fewer than three events form one burst",
			events: []graphEvent{ev("a", 1, 0), ev("b", 1, 1000)},
			want:   []int{0},
		},
		{
			name:   "evenly spaced events form one burst",
			events: []graphEvent{ev("a", 1, 0), ev("a", 2, 10), ev("a", 3, 20), ev("a", 4, 30)},
			want:   []int{0},
		},
		{
			name: "a long gap starts a new burst",
			events: []graphEvent{
				ev("a", 1, 0), ev("b", 1, 1), ev("a", 2, 2),
				ev("c", 1, 1000), ev("c", 2, 1001), ev("a", 3, 1002),
			},
			want: []int{0, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clusterBursts(tt.events)
			if !slices.Equal(got, tt.want) {
				t.Errorf("clusterBursts = %v, want %v", got, tt.want)
			}
			rendered := renderBurstTree(got, tt.events)
			for _, e := range tt.events {
				if !strings.Contains(rendered, e.Path) || !strings.Contains(rendered, "(v"+string(rune('0'+e.Version))+")") {
					t.Errorf("renderBurstTree = %q, want to contain %s (v%d)", rendered, e.Path, e.Version)
				}
			}
		})
	}
	if got := renderBurstTree(nil, nil); got != "" {
		t.Errorf("renderBurstTree(nil, nil) = %q, want empty", got)
	}
}
