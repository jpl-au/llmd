package search_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/search"
)

func TestFullText(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/auth", "authentication and authorization", testWriteOpts())
	s.Documents.Write(ctx, "docs/api", "API endpoints for users", testWriteOpts())
	s.Documents.Write(ctx, "docs/errors", "error handling and logging", testWriteOpts())

	results, err := s.Search.FullText(ctx, "auth*")
	if err != nil {
		t.Fatalf("FullText() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("FullText() returned %d results, want 1", len(results))
	}
	if len(results) > 0 && results[0].Path != "docs/auth" {
		t.Errorf("Path = %q, want %q", results[0].Path, "docs/auth")
	}
}

func TestFullText_PathPrefix(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "hello world", testWriteOpts())
	s.Documents.Write(ctx, "notes/readme", "hello world", testWriteOpts())

	results, err := s.Search.FullText(ctx, "hello", search.Options{Path: "docs/"})
	if err != nil {
		t.Fatalf("FullText() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("FullText(path=docs/) returned %d results, want 1", len(results))
	}
}

func TestFullText_Limit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for i := range 5 {
		s.Documents.Write(ctx, "docs/"+string(rune('a'+i)), "common word here", testWriteOpts())
	}

	results, err := s.Search.FullText(ctx, "common", search.Options{Limit: 2})
	if err != nil {
		t.Fatalf("FullText() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("FullText(limit=2) returned %d results, want 2", len(results))
	}
}

func TestFullText_ModePaths(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "hello world content", testWriteOpts())
	s.Documents.Write(ctx, "docs/guide", "hello again content", testWriteOpts())

	results, err := s.Search.FullText(ctx, "hello", search.Options{Mode: search.ModePaths})
	if err != nil {
		t.Fatalf("FullText(ModePaths) error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("FullText(ModePaths) returned %d results, want 2", len(results))
	}
	for _, r := range results {
		if len(r.Matches) != 0 {
			t.Errorf("ModePaths should have no matches, got %d", len(r.Matches))
		}
	}
}

func TestFullText_ModeLines(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	content := "line one\nline two with target word\nline three\nline four\nline five"
	s.Documents.Write(ctx, "docs/test", content, testWriteOpts())

	results, err := s.Search.FullText(ctx, "target", search.Options{
		Mode:    search.ModeLines,
		Context: 1,
	})
	if err != nil {
		t.Fatalf("FullText(ModeLines) error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if len(results[0].Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results[0].Matches))
	}

	m := results[0].Matches[0]
	if m.Line != 2 {
		t.Errorf("Line = %d, want 2", m.Line)
	}
	if m.Text != "line two with target word" {
		t.Errorf("Text = %q, want %q", m.Text, "line two with target word")
	}
	if len(m.Before) != 1 || m.Before[0] != "line one" {
		t.Errorf("Before = %v, want [line one]", m.Before)
	}
	if len(m.After) != 1 || m.After[0] != "line three" {
		t.Errorf("After = %v, want [line three]", m.After)
	}
}

func TestFullText_ModeLines_MultipleMatches(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	content := "first match here\nsecond line\nthird match here\nfourth line"
	s.Documents.Write(ctx, "docs/test", content, testWriteOpts())

	results, err := s.Search.FullText(ctx, "match", search.Options{Mode: search.ModeLines})
	if err != nil {
		t.Fatalf("FullText(ModeLines) error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if len(results[0].Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(results[0].Matches))
	}
}

func TestFullText_ModeSections(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	content := `# Introduction

This is the intro section.

# Features

This section has the target keyword.

# Conclusion

Final thoughts here.`

	s.Documents.Write(ctx, "docs/readme", content, testWriteOpts())

	results, err := s.Search.FullText(ctx, "target", search.Options{Mode: search.ModeSections})
	if err != nil {
		t.Fatalf("FullText(ModeSections) error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if len(results[0].Matches) != 1 {
		t.Fatalf("expected 1 section match, got %d", len(results[0].Matches))
	}

	m := results[0].Matches[0]
	if m.Section != "Features" {
		t.Errorf("Section = %q, want %q", m.Section, "Features")
	}
}

func TestFullText_ModeSections_NoHeadings(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	content := "Just plain text with target word and no markdown headings."
	s.Documents.Write(ctx, "docs/plain", content, testWriteOpts())

	results, err := s.Search.FullText(ctx, "target", search.Options{Mode: search.ModeSections})
	if err != nil {
		t.Fatalf("FullText(ModeSections) error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if len(results[0].Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results[0].Matches))
	}

	// No heading means empty section name
	if results[0].Matches[0].Section != "" {
		t.Errorf("Section = %q, want empty", results[0].Matches[0].Section)
	}
}

func TestFullText_ModeSnippets(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	content := "prefix text here then the target keyword appears in the middle of this document followed by more text"
	s.Documents.Write(ctx, "docs/test", content, testWriteOpts())

	results, err := s.Search.FullText(ctx, "target", search.Options{Mode: search.ModeSnippets})
	if err != nil {
		t.Fatalf("FullText(ModeSnippets) error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if len(results[0].Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results[0].Matches))
	}

	// Snippet should contain the keyword with highlight markers
	m := results[0].Matches[0]
	if m.Text == "" {
		t.Error("snippet text should not be empty")
	}
}

func TestFullText_InvalidQuery(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/test", "some content", testWriteOpts())

	// Invalid FTS5 syntax
	_, err := s.Search.FullText(ctx, "AND OR NOT")
	if err == nil {
		t.Error("expected error for invalid query")
	}
}
