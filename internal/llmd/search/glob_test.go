package search_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/search"
)

func TestGlob(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme.md", "readme", testWriteOpts())
	s.Documents.Write(ctx, "docs/api.md", "api", testWriteOpts())
	s.Documents.Write(ctx, "docs/guide/intro.md", "intro", testWriteOpts())
	s.Documents.Write(ctx, "notes/todo.txt", "todo", testWriteOpts())

	matches, err := s.Search.Glob(ctx, "docs/*.md")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("Glob(docs/*.md) returned %d matches, want 2", len(matches))
	}
}

func TestGlob_DoublestarPrefix(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme.md", "readme", testWriteOpts())
	s.Documents.Write(ctx, "docs/guide/intro.md", "intro", testWriteOpts())
	s.Documents.Write(ctx, "docs/guide/advanced/tips.md", "tips", testWriteOpts())
	s.Documents.Write(ctx, "notes/todo.txt", "todo", testWriteOpts())

	matches, err := s.Search.Glob(ctx, "docs/**/*.md")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	if len(matches) != 3 {
		t.Errorf("Glob(docs/**/*.md) returned %d matches, want 3: %v", len(matches), matches)
	}
}

func TestGlob_DoublestarOnly(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "a.md", "a", testWriteOpts())
	s.Documents.Write(ctx, "docs/b.md", "b", testWriteOpts())
	s.Documents.Write(ctx, "docs/sub/c.md", "c", testWriteOpts())

	matches, err := s.Search.Glob(ctx, "**/*.md")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	if len(matches) != 3 {
		t.Errorf("Glob(**/*.md) returned %d matches, want 3: %v", len(matches), matches)
	}
}

func TestGlob_QuestionMark(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "v1", "v1", testWriteOpts())
	s.Documents.Write(ctx, "v2", "v2", testWriteOpts())
	s.Documents.Write(ctx, "v10", "v10", testWriteOpts())

	matches, err := s.Search.Glob(ctx, "v?")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("Glob(v?) returned %d matches, want 2: %v", len(matches), matches)
	}
}

func TestGlob_Limit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for i := range 5 {
		s.Documents.Write(ctx, "docs/"+string(rune('a'+i))+".md", "content", testWriteOpts())
	}

	matches, err := s.Search.Glob(ctx, "docs/*.md", search.Options{Limit: 2})
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("Glob(limit=2) returned %d matches, want 2", len(matches))
	}
}

func TestGlob_NoMatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme.md", "readme", testWriteOpts())

	matches, err := s.Search.Glob(ctx, "notes/*.txt")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("Glob(no match) returned %d matches, want 0", len(matches))
	}
}
