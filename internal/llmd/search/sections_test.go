package search

import (
	"testing"
)

func TestParseMarkdown(t *testing.T) {
	content := `# First Section

Content of first section.

# Second Section

Content of second section.

## Subsection

Nested content here.`

	sections := parseMarkdown(content)

	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}

	if sections[0].header != "First Section" {
		t.Errorf("sections[0].header = %q, want %q", sections[0].header, "First Section")
	}
	if sections[1].header != "Second Section" {
		t.Errorf("sections[1].header = %q, want %q", sections[1].header, "Second Section")
	}
	if sections[2].header != "Subsection" {
		t.Errorf("sections[2].header = %q, want %q", sections[2].header, "Subsection")
	}
}

func TestParseMarkdown_NoHeadings(t *testing.T) {
	content := "Just plain text without any headings."

	sections := parseMarkdown(content)

	if len(sections) != 1 {
		t.Fatalf("expected 1 section for no headings, got %d", len(sections))
	}
	if sections[0].header != "" {
		t.Errorf("expected empty header, got %q", sections[0].header)
	}
	if sections[0].text != content {
		t.Errorf("expected full content in section")
	}
}

func TestParseMarkdown_Preamble(t *testing.T) {
	content := `Some preamble text before any heading.

# First Heading

Content after heading.`

	sections := parseMarkdown(content)

	if len(sections) != 2 {
		t.Fatalf("expected 2 sections (preamble + heading), got %d", len(sections))
	}

	// Preamble should be first
	if sections[0].header != "" {
		t.Errorf("preamble should have empty header, got %q", sections[0].header)
	}
	if sections[0].startLine != 1 {
		t.Errorf("preamble startLine = %d, want 1", sections[0].startLine)
	}
}

func TestContainsTerms(t *testing.T) {
	tests := []struct {
		text  string
		terms []string
		want  bool
	}{
		{"hello world", []string{"hello"}, true},
		{"hello world", []string{"HELLO"}, true}, // case insensitive
		{"hello world", []string{"missing"}, false},
		{"hello world", []string{"missing", "hello"}, true}, // any term matches
		{"hello world", []string{}, false},
	}

	for _, tt := range tests {
		got := containsTerms(tt.text, tt.terms)
		if got != tt.want {
			t.Errorf("containsTerms(%q, %v) = %v, want %v", tt.text, tt.terms, got, tt.want)
		}
	}
}

func TestExtractSections(t *testing.T) {
	content := `# Intro

No matches here.

# Target Section

This section has the unicorn we want.

# Conclusion

Also no matches.`

	matches := extractSections(content, "unicorn")

	if len(matches) != 1 {
		t.Fatalf("expected 1 matching section, got %d", len(matches))
	}

	if matches[0].Section != "Target Section" {
		t.Errorf("Section = %q, want %q", matches[0].Section, "Target Section")
	}
}

func TestExtractSections_MultipleMatches(t *testing.T) {
	content := `# First Match

keyword appears here.

# No Match

nothing interesting.

# Second Match

keyword again here.`

	matches := extractSections(content, "keyword")

	if len(matches) != 2 {
		t.Fatalf("expected 2 matching sections, got %d", len(matches))
	}

	if matches[0].Section != "First Match" {
		t.Errorf("first match Section = %q", matches[0].Section)
	}
	if matches[1].Section != "Second Match" {
		t.Errorf("second match Section = %q", matches[1].Section)
	}
}
