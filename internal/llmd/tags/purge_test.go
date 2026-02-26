package tags_test

import (
	"context"
	"testing"
)

func TestPurge(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create a document and add tags
	if _, err := s.Documents.Write(ctx, "docs/test", "content", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/test", "important", testOpts()); err != nil {
		t.Fatalf("Add important: %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/test", "review", testOpts()); err != nil {
		t.Fatalf("Add review: %v", err)
	}
	if _, err := s.Tags.Add(ctx, "docs/test", "draft", testOpts()); err != nil {
		t.Fatalf("Add draft: %v", err)
	}

	// Remove some tags
	if err := s.Tags.Remove(ctx, "docs/test", "important", testOpts()); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if err := s.Tags.Remove(ctx, "docs/test", "review", testOpts()); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Purge should remove 2 tags
	n, err := s.Tags.Purge(ctx)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if n != 2 {
		t.Errorf("Purge() = %d, want 2", n)
	}

	// "draft" tag should still exist
	tags, err := s.Tags.List(ctx, "docs/test")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tags) != 1 || tags[0].Value.Tag != "draft" {
		t.Errorf("List() = %v, want [draft]", tags)
	}
}

func TestPurge_Empty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Purge with nothing to purge
	n, err := s.Tags.Purge(ctx)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if n != 0 {
		t.Errorf("Purge() = %d, want 0", n)
	}
}
