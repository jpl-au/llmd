package search

import (
	"reflect"
	"testing"
)

func TestParseMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []section
	}{
		{
			name: "headings",
			content: `# First Section

Content of first section.

# Second Section

Content of second section.

## Subsection

Nested content here.`,
			want: []section{
				{header: "First Section", startLine: 1, endLine: 4, text: "# First Section\n\nContent of first section.\n"},
				{header: "Second Section", startLine: 5, endLine: 8, text: "# Second Section\n\nContent of second section.\n"},
				{header: "Subsection", startLine: 9, endLine: 11, text: "## Subsection\n\nNested content here."},
			},
		},
		{
			// No headings: the whole content is one section with an empty header.
			name:    "no headings",
			content: "Just plain text without any headings.",
			want: []section{
				{header: "", startLine: 1, endLine: 1, text: "Just plain text without any headings."},
			},
		},
		{
			// Preamble before the first heading comes first with an empty header.
			name: "preamble",
			content: `Some preamble text before any heading.

# First Heading

Content after heading.`,
			want: []section{
				{header: "", startLine: 1, endLine: 2, text: "Some preamble text before any heading.\n"},
				{header: "First Heading", startLine: 3, endLine: 5, text: "# First Heading\n\nContent after heading."},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMarkdown(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseMarkdown(%q) = %#v, want %#v", tt.content, got, tt.want)
			}
		})
	}
}

func TestContainsTerms(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		terms []string
		want  bool
	}{
		{"match", "hello world", []string{"hello"}, true},
		{"case insensitive", "hello world", []string{"HELLO"}, true},
		{"missing", "hello world", []string{"missing"}, false},
		{"any term matches", "hello world", []string{"missing", "hello"}, true},
		{"no terms", "hello world", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsTerms(tt.text, tt.terms)
			if got != tt.want {
				t.Errorf("containsTerms(%q, %v) = %v, want %v", tt.text, tt.terms, got, tt.want)
			}
		})
	}
}

func TestExtractSections(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		want    []Match
	}{
		{
			name: "single match",
			content: `# Intro

No matches here.

# Target Section

This section has the unicorn we want.

# Conclusion

Also no matches.`,
			query: "unicorn",
			want: []Match{
				{Line: 5, Text: "# Target Section\n\nThis section has the unicorn we want.\n", Section: "Target Section"},
			},
		},
		{
			name: "multiple matches",
			content: `# First Match

keyword appears here.

# No Match

nothing interesting.

# Second Match

keyword again here.`,
			query: "keyword",
			want: []Match{
				{Line: 1, Text: "# First Match\n\nkeyword appears here.\n", Section: "First Match"},
				{Line: 9, Text: "# Second Match\n\nkeyword again here.", Section: "Second Match"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSections(tt.content, tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractSections(%q, %q) = %#v, want %#v", tt.content, tt.query, got, tt.want)
			}
		})
	}
}
