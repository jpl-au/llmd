package search_test

import (
	"slices"
	"testing"

	"github.com/jpl-au/llmd/engine/search"
)

func TestGlob(t *testing.T) {
	tests := []struct {
		name    string
		docs    []string
		rewrite []string // docs written a second time so they reach version 2
		pattern string
		opts    search.Options
		want    []string
	}{
		{
			name:    "single star",
			docs:    []string{"docs/readme.md", "docs/api.md", "docs/guide/intro.md", "notes/todo.txt"},
			pattern: "docs/*.md",
			want:    []string{"docs/api.md", "docs/readme.md"},
		},
		{
			name:    "doublestar prefix",
			docs:    []string{"docs/readme.md", "docs/guide/intro.md", "docs/guide/advanced/tips.md", "notes/todo.txt"},
			pattern: "docs/**/*.md",
			want:    []string{"docs/guide/advanced/tips.md", "docs/guide/intro.md", "docs/readme.md"},
		},
		{
			name:    "doublestar only",
			docs:    []string{"a.md", "docs/b.md", "docs/sub/c.md"},
			pattern: "**/*.md",
			want:    []string{"a.md", "docs/b.md", "docs/sub/c.md"},
		},
		{
			name:    "question mark",
			docs:    []string{"v1", "v2", "v10"},
			pattern: "v?",
			want:    []string{"v1", "v2"},
		},
		{
			name:    "limit",
			docs:    []string{"docs/a.md", "docs/b.md", "docs/c.md", "docs/d.md", "docs/e.md"},
			pattern: "docs/*.md",
			opts:    search.Options{Limit: 2},
			want:    []string{"docs/a.md", "docs/b.md"},
		},
		{
			name:    "no match",
			docs:    []string{"docs/readme.md"},
			pattern: "notes/*.txt",
			want:    nil,
		},
		{
			// Regression: the latest-version subquery must correlate per
			// path. A single doc at version 2 used to hide every doc still
			// at version 1.
			name:    "mixed versions",
			docs:    []string{"docs/a.md", "docs/b.md", "docs/c.md"},
			rewrite: []string{"docs/b.md"},
			pattern: "docs/*.md",
			want:    []string{"docs/a.md", "docs/b.md", "docs/c.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			for _, p := range tt.docs {
				if _, err := s.Documents.Write(t.Context(), p, "content", testWriteOpts()); err != nil {
					t.Fatalf("Write(%q) error = %v", p, err)
				}
			}
			for _, p := range tt.rewrite {
				if _, err := s.Documents.Write(t.Context(), p, "content v2", testWriteOpts()); err != nil {
					t.Fatalf("Write(%q) v2 error = %v", p, err)
				}
			}

			got, err := s.Search.Glob(t.Context(), tt.pattern, tt.opts)
			if err != nil {
				t.Fatalf("Glob(%q) error = %v", tt.pattern, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Glob(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}
