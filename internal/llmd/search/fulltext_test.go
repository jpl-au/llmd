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
