package tags_test

import (
	"context"
	"testing"
)

func TestList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())
	s.Tags.Add(ctx, "docs/readme", "important", testOpts())
	s.Tags.Add(ctx, "docs/readme", "v1", testOpts())

	tags, err := s.Tags.List(ctx, "docs/readme", testOpts())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(tags) != 2 {
		t.Errorf("List() returned %d tags, want 2", len(tags))
	}
}

func TestList_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	doc, _ := s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())
	s.Tags.Add(ctx, "docs/readme", "important", testOpts())

	tags, err := s.Tags.List(ctx, doc.Key, testOpts())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(tags) != 1 {
		t.Errorf("List() returned %d tags, want 1", len(tags))
	}
}

func TestList_Empty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())

	tags, err := s.Tags.List(ctx, "docs/readme", testOpts())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(tags) != 0 {
		t.Errorf("List() returned %d tags, want 0", len(tags))
	}
}

func TestListAll(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())
	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())

	s.Tags.Add(ctx, "docs/readme", "important", testOpts())
	s.Tags.Add(ctx, "docs/readme", "v1", testOpts())
	s.Tags.Add(ctx, "docs/api", "important", testOpts()) // duplicate tag name
	s.Tags.Add(ctx, "docs/api", "needs-review", testOpts())

	all, err := s.Tags.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	// Should return unique tag names: important, needs-review, v1
	if len(all) != 3 {
		t.Errorf("ListAll() returned %d tags, want 3: %v", len(all), all)
	}
}

func TestListAll_Empty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	all, err := s.Tags.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	if len(all) != 0 {
		t.Errorf("ListAll() returned %d tags, want 0", len(all))
	}
}
