package tags_test

import (
	"context"
	"testing"
)

func TestExists(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/test", "content", testWriteOpts()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/test", "important", testOpts()); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if ok, err := s.Tags.Exists(ctx, "docs/test", "important"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if !ok {
		t.Error("Exists() = false, want true")
	}
}

func TestExists_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/test", "content", testWriteOpts()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if ok, err := s.Tags.Exists(ctx, "docs/test", "nonexistent"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if ok {
		t.Error("Exists() = true, want false")
	}
}

func TestExists_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	written, err := s.Documents.Write(ctx, "docs/test", "content", testWriteOpts())
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/test", "important", testOpts()); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if ok, err := s.Tags.Exists(ctx, written.Key, "important"); err != nil {
		t.Fatalf("Exists(key) error = %v", err)
	} else if !ok {
		t.Error("Exists(key) = false, want true")
	}
}

func TestExists_RemovedTag(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/test", "content", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/test", "important", testOpts()); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Tags.Remove(ctx, "docs/test", "important", testOpts()); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if ok, err := s.Tags.Exists(ctx, "docs/test", "important"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if ok {
		t.Error("Exists() = true for removed tag, want false")
	}
}
