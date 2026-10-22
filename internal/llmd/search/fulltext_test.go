package search_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/search"
)

// testDoc is a document written to a fresh store before searching.
type testDoc struct {
	path    string
	content string
}

// TestFullText covers result-level behaviour: which documents a query
// returns under each option. wantPaths is checked when set; want is the
// result count for cases where rank ordering makes the paths ambiguous.
func TestFullText(t *testing.T) {
	tests := []struct {
		name        string
		docs        []testDoc
		query       string
		opts        search.Options
		wantErr     error
		want        int      // number of results
		wantPaths   []string // sorted result paths, checked when set
		wantMatches int      // matches per result
	}{
		{
			name: "prefix match",
			docs: []testDoc{
				{"docs/auth", "authentication and authorization"},
				{"docs/api", "API endpoints for users"},
				{"docs/errors", "error handling and logging"},
			},
			query:       "auth*",
			want:        1,
			wantPaths:   []string{"docs/auth"},
			wantMatches: 1,
		},
		{
			name: "path prefix",
			docs: []testDoc{
				{"docs/readme", "hello world"},
				{"notes/readme", "hello world"},
			},
			query:       "hello",
			opts:        search.Options{Path: "docs/"},
			want:        1,
			wantPaths:   []string{"docs/readme"},
			wantMatches: 1,
		},
		{
			name: "limit",
			docs: []testDoc{
				{"docs/a", "common word here"},
				{"docs/b", "common word here"},
				{"docs/c", "common word here"},
				{"docs/d", "common word here"},
				{"docs/e", "common word here"},
			},
			query:       "common",
			opts:        search.Options{Limit: 2},
			want:        2,
			wantMatches: 1,
		},
		{
			name: "mode paths has no matches",
			docs: []testDoc{
				{"docs/readme", "hello world content"},
				{"docs/guide", "hello again content"},
			},
			query:       "hello",
			opts:        search.Options{Mode: search.ModePaths},
			want:        2,
			wantPaths:   []string{"docs/guide", "docs/readme"},
			wantMatches: 0,
		},
		{
			name:    "invalid query",
			docs:    []testDoc{{"docs/test", "some content"}},
			query:   "AND OR NOT",
			wantErr: search.ErrInvalidQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			for _, d := range tt.docs {
				if _, err := s.Documents.Write(t.Context(), d.path, d.content, testWriteOpts()); err != nil {
					t.Fatalf("Write(%q) error = %v", d.path, err)
				}
			}

			results, err := s.Search.FullText(t.Context(), tt.query, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("FullText(%q) error = %v, want %v", tt.query, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if len(results) != tt.want {
				t.Errorf("FullText(%q) returned %d results, want %d", tt.query, len(results), tt.want)
			}
			var paths []string
			for _, r := range results {
				paths = append(paths, r.Path)
				if len(r.Matches) != tt.wantMatches {
					t.Errorf("FullText(%q) %s has %d matches, want %d", tt.query, r.Path, len(r.Matches), tt.wantMatches)
				}
			}
			slices.Sort(paths)
			if tt.wantPaths != nil && !slices.Equal(paths, tt.wantPaths) {
				t.Errorf("FullText(%q) paths = %v, want %v", tt.query, paths, tt.wantPaths)
			}
		})
	}
}

// TestFullText_Modes covers match-level output: the Match values each
// mode produces for a single document.
func TestFullText_Modes(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		opts    search.Options
		want    []search.Match
	}{
		{
			name:    "lines with context",
			content: "line one\nline two with target word\nline three\nline four\nline five",
			query:   "target",
			opts:    search.Options{Mode: search.ModeLines, Context: 1},
			want: []search.Match{
				{Line: 2, Column: 14, Text: "line two with target word", Before: []string{"line one"}, After: []string{"line three"}},
			},
		},
		{
			name:    "lines multiple matches",
			content: "first match here\nsecond line\nthird match here\nfourth line",
			query:   "match",
			opts:    search.Options{Mode: search.ModeLines},
			want: []search.Match{
				{Line: 1, Column: 6, Text: "first match here"},
				{Line: 3, Column: 6, Text: "third match here"},
			},
		},
		{
			name: "sections",
			content: `# Introduction

This is the intro section.

# Features

This section has the target keyword.

# Conclusion

Final thoughts here.`,
			query: "target",
			opts:  search.Options{Mode: search.ModeSections},
			want: []search.Match{
				{Line: 5, Text: "# Features\n\nThis section has the target keyword.\n", Section: "Features"},
			},
		},
		{
			// No heading means empty section name.
			name:    "sections no headings",
			content: "Just plain text with target word and no markdown headings.",
			query:   "target",
			opts:    search.Options{Mode: search.ModeSections},
			want: []search.Match{
				{Line: 1, Text: "Just plain text with target word and no markdown headings.", Section: ""},
			},
		},
		{
			// Snippet contains the keyword with highlight markers.
			name:    "snippets",
			content: "prefix text here then the target keyword appears in the middle of this document followed by more text",
			query:   "target",
			opts:    search.Options{Mode: search.ModeSnippets},
			want: []search.Match{
				{Text: "prefix text here then the <b>target</b> keyword appears in the middle of this document followed by more text"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testStore(t)
			if _, err := s.Documents.Write(t.Context(), "docs/test", tt.content, testWriteOpts()); err != nil {
				t.Fatalf("Write() error = %v", err)
			}

			results, err := s.Search.FullText(t.Context(), tt.query, tt.opts)
			if err != nil {
				t.Fatalf("FullText(%q) error = %v", tt.query, err)
			}
			if len(results) != 1 {
				t.Fatalf("FullText(%q) returned %d results, want 1", tt.query, len(results))
			}
			if results[0].Path != "docs/test" {
				t.Errorf("FullText(%q) Path = %q, want %q", tt.query, results[0].Path, "docs/test")
			}
			if !reflect.DeepEqual(results[0].Matches, tt.want) {
				t.Errorf("FullText(%q) Matches = %#v, want %#v", tt.query, results[0].Matches, tt.want)
			}
		})
	}
}
