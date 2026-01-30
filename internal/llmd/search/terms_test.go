package search

import (
	"reflect"
	"testing"
)

func TestExtractSearchTerms(t *testing.T) {
	tests := []struct {
		pattern string
		want    []string
	}{
		// Simple literal
		{"hello", []string{"hello"}},

		// Case preserved (lowercased)
		{"Hello", []string{"hello"}},

		// Multiple words in concat
		{"hello world", []string{"hello world"}},

		// Word boundary
		{`\bhello\b`, []string{"hello"}},

		// Prefix/suffix wildcards - still extracts the literal
		{"hello.*", []string{"hello"}},
		{".*world", []string{"world"}},

		// Alternation
		{"foo|bar", []string{"foo", "bar"}},

		// Short terms filtered out
		{"ab", nil},
		{"a", nil},

		// Complex pattern with extractable parts
		{`func\s+(\w+)`, []string{"func"}},

		// Capture group
		{"(hello)", []string{"hello"}},

		// Invalid pattern
		{"[invalid", nil},

		// Only metacharacters - no useful terms
		{`\d+`, nil},
		{`.*`, nil},
		{`.+`, nil},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := ExtractSearchTerms(tt.pattern)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractSearchTerms(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestBuildFTSQuery(t *testing.T) {
	tests := []struct {
		terms []string
		want  string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"hello"}, `"hello"`},
		{[]string{"hello", "world"}, `"hello" OR "world"`},
		{[]string{"foo", "bar", "baz"}, `"foo" OR "bar" OR "baz"`},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := BuildFTSQuery(tt.terms)
			if got != tt.want {
				t.Errorf("BuildFTSQuery(%v) = %q, want %q", tt.terms, got, tt.want)
			}
		})
	}
}

func TestEscapeFTS(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{`say "hello"`, `say ""hello""`},
		{"normal text", "normal text"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeFTS(tt.input)
			if got != tt.want {
				t.Errorf("escapeFTS(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
