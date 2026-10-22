package search

import (
	"reflect"
	"testing"
)

func TestParseTerms(t *testing.T) {
	tests := []struct {
		query string
		want  []string
	}{
		{"hello", []string{"hello"}},
		{"hello world", []string{"hello", "world"}},
		{"hello*", []string{"hello"}},
		{`"exact phrase"`, []string{"exact phrase"}},
		{"foo AND bar", []string{"foo", "bar"}},
		{"foo OR bar", []string{"foo", "bar"}},
		{"NOT excluded", []string{"excluded"}},
		{"-negated term", []string{"negated", "term"}},
		{"prefix* suffix*", []string{"prefix", "suffix"}},
		{`mixed "quoted phrase" unquoted`, []string{"mixed", "quoted phrase", "unquoted"}},
		{"NEAR/3 proximity", []string{"near/3", "proximity"}}, // NEAR not filtered (rare usage)
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := parseTerms(tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTerms(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestIndexFold(t *testing.T) {
	tests := []struct {
		s, substr string
		want      int
	}{
		{"Hello World", "hello", 0},
		{"Hello World", "WORLD", 6},
		{"Hello World", "missing", -1},
		{"CamelCase", "camel", 0},
		{"CamelCase", "case", 5},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := indexFold(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("indexFold(%q, %q) = %d, want %d", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestGetContext(t *testing.T) {
	lines := []string{"zero", "one", "two", "three", "four", "five"}

	tests := []struct {
		name    string
		lineNum int
		n       int
		want    []string
	}{
		{"before 2", 3, -2, []string{"one", "two"}},
		{"after 2", 2, 2, []string{"three", "four"}},
		{"before at start", 1, -2, []string{"zero"}},
		{"after at end", 4, 3, []string{"five"}},
		{"no context", 2, 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getContext(lines, tt.lineNum, tt.n)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getContext(lines, %d, %d) = %v, want %v", tt.lineNum, tt.n, got, tt.want)
			}
		})
	}
}

func TestExtractLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		ctx     int
		want    []Match
	}{
		{
			name:    "with context",
			content: "first line\nsecond with keyword\nthird line\nfourth keyword again\nfifth line",
			query:   "keyword",
			ctx:     1,
			want: []Match{
				{Line: 2, Column: 12, Text: "second with keyword", Before: []string{"first line"}, After: []string{"third line"}},
				{Line: 4, Column: 7, Text: "fourth keyword again", Before: []string{"third line"}, After: []string{"fifth line"}},
			},
		},
		{
			name:    "case insensitive",
			content: "UPPERCASE keyword here\nlowercase KEYWORD here",
			query:   "keyword",
			want: []Match{
				{Line: 1, Column: 10, Text: "UPPERCASE keyword here"},
				{Line: 2, Column: 10, Text: "lowercase KEYWORD here"},
			},
		},
		{
			name:    "no matches",
			content: "nothing relevant here",
			query:   "missing",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLines(tt.content, tt.query, tt.ctx)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractLines(%q, %q, %d) = %#v, want %#v", tt.content, tt.query, tt.ctx, got, tt.want)
			}
		})
	}
}
