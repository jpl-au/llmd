package links_test

import (
	"context"
	"testing"
)

func TestPurge(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create documents and add links
	s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts())
	s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts())
	s.Documents.Write(ctx, "docs/c", "content c", testWriteOpts())

	s.Links.Add(ctx, "docs/a", "docs/b", testOpts())
	s.Links.Add(ctx, "docs/a", "docs/c", testOpts())
	s.Links.Add(ctx, "docs/b", "docs/c", testOpts())

	// Remove some links
	s.Links.Remove(ctx, "docs/a", "docs/b", testOpts())
	s.Links.Remove(ctx, "docs/a", "docs/c", testOpts())

	// Purge should remove 2 links
	n, err := s.Links.Purge(ctx)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if n != 2 {
		t.Errorf("Purge() = %d, want 2", n)
	}

	// b -> c link should still exist
	links, err := s.Links.List(ctx, "docs/b")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(links) != 1 || links[0].Value.To != "docs/c" {
		t.Errorf("List() = %v, want [docs/c]", links)
	}
}

func TestPurge_Empty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Purge with nothing to purge
	n, err := s.Links.Purge(ctx)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if n != 0 {
		t.Errorf("Purge() = %d, want 0", n)
	}
}
