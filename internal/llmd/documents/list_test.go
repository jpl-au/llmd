package documents_test

import (
	"context"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	s.Documents.Write(ctx, "docs/readme", "readme", opts)
	s.Documents.Write(ctx, "docs/api", "api", opts)
	s.Documents.Write(ctx, "notes/todo", "todo", opts)

	all, err := s.Documents.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 3 {
		t.Errorf("List() returned %d items, want 3", len(all))
	}

	docs, err := s.Documents.List(ctx, documents.ListOptions{Prefix: "docs/"})
	if err != nil {
		t.Fatalf("List(prefix) error = %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("List(prefix) returned %d items, want 2", len(docs))
	}
}

func TestList_OnlyLatestVersion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	s.Documents.Write(ctx, "docs/readme", "v1", opts)
	s.Documents.Write(ctx, "docs/readme", "v2", opts)
	s.Documents.Write(ctx, "docs/readme", "v3", opts)

	list, _ := s.Documents.List(ctx)
	if len(list) != 1 {
		t.Errorf("List() returned %d items, want 1 (latest only)", len(list))
	}
	if list[0].Version != 3 {
		t.Errorf("List() returned version %d, want 3", list[0].Version)
	}
}

func TestList_IncludeDeleted(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/active", "active", testWriteOpts())
	s.Documents.Write(ctx, "docs/deleted", "deleted", testWriteOpts())
	s.Documents.Delete(ctx, "docs/deleted", testDeleteOpts())

	list, _ := s.Documents.List(ctx)
	if len(list) != 1 {
		t.Errorf("List() returned %d items, want 1", len(list))
	}

	list, _ = s.Documents.List(ctx, documents.ListOptions{IncludeDeleted: true})
	if len(list) != 2 {
		t.Errorf("List(IncludeDeleted) returned %d items, want 2", len(list))
	}
}

func TestList_SortByTime(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	// Write in reverse alphabetical order with distinct timestamps
	// so time order (newest first) differs from path order.
	s.Documents.Write(ctx, "notes/c", "c", opts)
	time.Sleep(2 * time.Millisecond)
	s.Documents.Write(ctx, "notes/b", "b", opts)
	time.Sleep(2 * time.Millisecond)
	s.Documents.Write(ctx, "notes/a", "a", opts)

	list, err := s.Documents.List(ctx, documents.ListOptions{Sort: "time"})
	if err != nil {
		t.Fatalf("List(Sort:time) error = %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List(Sort:time) returned %d items, want 3", len(list))
	}

	// Newest first: a (written last), then b, then c
	if list[0].Path != "notes/a" || list[1].Path != "notes/b" || list[2].Path != "notes/c" {
		t.Errorf("List(Sort:time) = [%s, %s, %s], want [notes/a, notes/b, notes/c]",
			list[0].Path, list[1].Path, list[2].Path)
	}

	// Default sort should still be by path
	list, _ = s.Documents.List(ctx)
	if list[0].Path != "notes/a" || list[1].Path != "notes/b" || list[2].Path != "notes/c" {
		t.Errorf("List() default sort = [%s, %s, %s], want [notes/a, notes/b, notes/c]",
			list[0].Path, list[1].Path, list[2].Path)
	}
}
