package tags_test

import (
	"context"
	"testing"
)

func TestExists(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/test", "content", testWriteOpts())
	s.Tags.Add(ctx, "docs/test", "important", testOpts())

	if ok, err := s.Tags.Exists(ctx, "docs/test", "important"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if !ok {
		t.Error("Exists() = false, want true")
	}
}

func TestExists_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/test", "content", testWriteOpts())

	if ok, err := s.Tags.Exists(ctx, "docs/test", "nonexistent"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if ok {
		t.Error("Exists() = true, want false")
	}
}

func TestExists_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	written, _ := s.Documents.Write(ctx, "docs/test", "content", testWriteOpts())
	s.Tags.Add(ctx, "docs/test", "important", testOpts())

	if ok, err := s.Tags.Exists(ctx, written.Key, "important"); err != nil {
		t.Fatalf("Exists(key) error = %v", err)
	} else if !ok {
		t.Error("Exists(key) = false, want true")
	}
}

func TestExists_RemovedTag(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/test", "content", testWriteOpts())
	s.Tags.Add(ctx, "docs/test", "important", testOpts())
	s.Tags.Remove(ctx, "docs/test", "important", testOpts())

	if ok, err := s.Tags.Exists(ctx, "docs/test", "important"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if ok {
		t.Error("Exists() = true for removed tag, want false")
	}
}
