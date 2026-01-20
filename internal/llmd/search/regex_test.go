package search_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/search"
)

func TestRegex(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/code", "func main() {\n\tfmt.Println(\"hello\")\n}", testWriteOpts())
	s.Documents.Write(ctx, "docs/other", "no functions here", testWriteOpts())

	result, err := s.Search.Regex(ctx, "func.*\\(\\)", search.Options{Mode: search.ModeFiles})
	if err != nil {
		t.Fatalf("Regex() error = %v", err)
	}

	if len(result.Files) != 1 {
		t.Errorf("Regex() found %d files, want 1", len(result.Files))
	}
	if len(result.Files) > 0 && result.Files[0] != "docs/code" {
		t.Errorf("Files[0] = %q, want %q", result.Files[0], "docs/code")
	}
}

func TestRegex_IgnoreCase(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/test", "ERROR: something failed", testWriteOpts())

	result, err := s.Search.Regex(ctx, "error", search.Options{
		IgnoreCase: true,
		Mode:       search.ModeFiles,
	})
	if err != nil {
		t.Fatalf("Regex() error = %v", err)
	}

	if len(result.Files) != 1 {
		t.Errorf("Regex(ignoreCase) found %d files, want 1", len(result.Files))
	}
}

func TestRegex_Count(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/test", "TODO: first\nTODO: second\nTODO: third", testWriteOpts())

	result, err := s.Search.Regex(ctx, "TODO", search.Options{Mode: search.ModeCount})
	if err != nil {
		t.Fatalf("Regex() error = %v", err)
	}

	if result.Counts["docs/test"] != 3 {
		t.Errorf("Counts[docs/test] = %d, want 3", result.Counts["docs/test"])
	}
}

func TestRegex_Content(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/test", "line one\nTODO: fix this\nline three", testWriteOpts())

	result, err := s.Search.Regex(ctx, "TODO", search.Options{Mode: search.ModeContent})
	if err != nil {
		t.Fatalf("Regex() error = %v", err)
	}

	if len(result.Matches) != 1 {
		t.Fatalf("Matches = %d, want 1", len(result.Matches))
	}
	if result.Matches[0].Line != 2 {
		t.Errorf("Line = %d, want 2", result.Matches[0].Line)
	}
	if result.Matches[0].Content != "TODO: fix this" {
		t.Errorf("Content = %q, want %q", result.Matches[0].Content, "TODO: fix this")
	}
}

func TestRegex_InvalidPattern(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Search.Regex(ctx, "[invalid")
	if err != search.ErrInvalidPattern {
		t.Errorf("Regex() error = %v, want ErrInvalidPattern", err)
	}
}
