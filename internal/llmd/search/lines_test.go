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
	content := "first line\nsecond with keyword\nthird line\nfourth keyword again\nfifth line"

	matches := extractLines(content, "keyword", 1)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	// First match
	if matches[0].Line != 2 {
		t.Errorf("first match Line = %d, want 2", matches[0].Line)
	}
	if matches[0].Text != "second with keyword" {
		t.Errorf("first match Text = %q", matches[0].Text)
	}
	if len(matches[0].Before) != 1 || matches[0].Before[0] != "first line" {
		t.Errorf("first match Before = %v", matches[0].Before)
	}
	if len(matches[0].After) != 1 || matches[0].After[0] != "third line" {
		t.Errorf("first match After = %v", matches[0].After)
	}

	// Second match
	if matches[1].Line != 4 {
		t.Errorf("second match Line = %d, want 4", matches[1].Line)
	}
}

func TestExtractLines_CaseInsensitive(t *testing.T) {
	content := "UPPERCASE keyword here\nlowercase KEYWORD here"

	matches := extractLines(content, "keyword", 0)

	if len(matches) != 2 {
		t.Errorf("expected 2 case-insensitive matches, got %d", len(matches))
	}
}

func TestExtractLines_NoMatches(t *testing.T) {
	content := "nothing relevant here"

	matches := extractLines(content, "missing", 0)

	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}
