package refs

import (
	"slices"
	"testing"
)

func TestTargets(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		content string
		want    []string
	}{
		{"sibling", "gdd/02", "See [Damage](04.md).", []string{"gdd/04"}},
		{"parent folder", "gdd/editor/01", "[Index](../the-pale-war.md)", []string{"gdd/the-pale-war"}},
		{"child folder", "gdd/the-pale-war", "[Editor](editor/editor.md)", []string{"gdd/editor/editor"}},
		{"at the root", "index", "[One](01.md)", []string{"01"}},
		{"a fragment names the document", "gdd/02", "[Rules](04.md#2-rules)", []string{"gdd/04"}},
		{"first appearance order, no repeats", "a", "[B](b.md), [C](c.md), [B again](b.md)", []string{"b", "c"}},
		{"in a table", "a", "| [B](b.md) | what |\n|---|---|\n| x | y |", []string{"b"}},
		{"code is not a reference", "a", "`[B](b.md)`\n\n```\n[C](c.md)\n```", nil},
		{"a web address is not a reference", "a", "[Site](https://example.com/b.md)", nil},
		{"an absolute path is not a reference", "a", "[B](/b.md)", nil},
		{"another file type is not a reference", "a", "[Art](art.png)", nil},
		{"leaving the root is not a reference", "a", "[Out](../b.md)", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Targets(tt.from, tt.content); !slices.Equal(got, tt.want) {
				t.Errorf("Targets(%q, %q) = %q, want %q", tt.from, tt.content, got, tt.want)
			}
		})
	}
}
