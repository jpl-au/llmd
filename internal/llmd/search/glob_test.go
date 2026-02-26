package search_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/search"
)

func TestGlob(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/readme.md", "readme", testWriteOpts()); err != nil {
		t.Fatalf("Write readme: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/api.md", "api", testWriteOpts()); err != nil {
		t.Fatalf("Write api: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/guide/intro.md", "intro", testWriteOpts()); err != nil {
		t.Fatalf("Write intro: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "notes/todo.txt", "todo", testWriteOpts()); err != nil {
		t.Fatalf("Write todo: %v", err)
	}

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

	if _, err := s.Documents.Write(ctx, "docs/readme.md", "readme", testWriteOpts()); err != nil {
		t.Fatalf("Write readme: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/guide/intro.md", "intro", testWriteOpts()); err != nil {
		t.Fatalf("Write intro: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/guide/advanced/tips.md", "tips", testWriteOpts()); err != nil {
		t.Fatalf("Write tips: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "notes/todo.txt", "todo", testWriteOpts()); err != nil {
		t.Fatalf("Write todo: %v", err)
	}

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

	if _, err := s.Documents.Write(ctx, "a.md", "a", testWriteOpts()); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/b.md", "b", testWriteOpts()); err != nil {
		t.Fatalf("Write b: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/sub/c.md", "c", testWriteOpts()); err != nil {
		t.Fatalf("Write c: %v", err)
	}

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

	if _, err := s.Documents.Write(ctx, "v1", "v1", testWriteOpts()); err != nil {
		t.Fatalf("Write v1: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "v2", "v2", testWriteOpts()); err != nil {
		t.Fatalf("Write v2: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "v10", "v10", testWriteOpts()); err != nil {
		t.Fatalf("Write v10: %v", err)
	}

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
		if _, err := s.Documents.Write(ctx, "docs/"+string(rune('a'+i))+".md", "content", testWriteOpts()); err != nil {
			t.Fatalf("Write docs/%c.md: %v", rune('a'+i), err)
		}
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

	if _, err := s.Documents.Write(ctx, "docs/readme.md", "readme", testWriteOpts()); err != nil {
		t.Fatalf("Write readme: %v", err)
	}

	matches, err := s.Search.Glob(ctx, "notes/*.txt")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("Glob(no match) returned %d matches, want 0", len(matches))
	}
}
