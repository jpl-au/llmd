package links_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/links"
)

func TestRemove(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts()); err != nil {
		t.Fatalf("Write api: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts()); err != nil {
		t.Fatalf("Write models: %v", err)
	}
	if _, err := s.Links.Add(ctx, "docs/api", "docs/models", testOpts()); err != nil {
		t.Fatalf("Add: %v", err)
	}

	err := s.Links.Remove(ctx, "docs/api", "docs/models", testOpts())
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Link should no longer appear in list
	list, _ := s.Links.List(ctx, "docs/api", testOpts())
	if len(list) != 0 {
		t.Errorf("List() after remove returned %d links, want 0", len(list))
	}
}

func TestRemove_WithLabel(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts()); err != nil {
		t.Fatalf("Write api: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/auth", "auth content", testWriteOpts()); err != nil {
		t.Fatalf("Write auth: %v", err)
	}

	opts1 := testOpts()
	opts1.Label = "requires"

	opts2 := testOpts()
	opts2.Label = "related"

	if _, err := s.Links.Add(ctx, "docs/api", "docs/auth", opts1); err != nil {
		t.Fatalf("Add requires: %v", err)
	}
	if _, err := s.Links.Add(ctx, "docs/api", "docs/auth", opts2); err != nil {
		t.Fatalf("Add related: %v", err)
	}

	// Remove only the "requires" link
	removeOpts := testOpts()
	removeOpts.Label = "requires"
	err := s.Links.Remove(ctx, "docs/api", "docs/auth", removeOpts)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Should still have the "related" link
	list, _ := s.Links.List(ctx, "docs/api", testOpts())
	if len(list) != 1 {
		t.Errorf("List() returned %d links, want 1", len(list))
	}
	if list[0].Value.Label != "related" {
		t.Errorf("Remaining link Value.Label = %q, want %q", list[0].Value.Label, "related")
	}
}

func TestRemove_AllLinks(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts()); err != nil {
		t.Fatalf("Write api: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/auth", "auth content", testWriteOpts()); err != nil {
		t.Fatalf("Write auth: %v", err)
	}

	opts1 := testOpts()
	opts1.Label = "requires"

	opts2 := testOpts()
	opts2.Label = "related"

	if _, err := s.Links.Add(ctx, "docs/api", "docs/auth", opts1); err != nil {
		t.Fatalf("Add requires: %v", err)
	}
	if _, err := s.Links.Add(ctx, "docs/api", "docs/auth", opts2); err != nil {
		t.Fatalf("Add related: %v", err)
	}

	// Remove all links (no label specified)
	err := s.Links.Remove(ctx, "docs/api", "docs/auth", testOpts())
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Should have no links
	list, _ := s.Links.List(ctx, "docs/api", testOpts())
	if len(list) != 0 {
		t.Errorf("List() returned %d links, want 0", len(list))
	}
}

func TestRemove_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts()); err != nil {
		t.Fatalf("Write api: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts()); err != nil {
		t.Fatalf("Write models: %v", err)
	}

	err := s.Links.Remove(ctx, "docs/api", "docs/models", testOpts())
	if !errors.Is(err, links.ErrNotFound) {
		t.Errorf("Remove() error = %v, want ErrNotFound", err)
	}
}

func TestRemove_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	fromDoc, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	if err != nil {
		t.Fatalf("Write api: %v", err)
	}
	toDoc, err := s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts())
	if err != nil {
		t.Fatalf("Write models: %v", err)
	}
	if _, err := s.Links.Add(ctx, "docs/api", "docs/models", testOpts()); err != nil {
		t.Fatalf("Add: %v", err)
	}

	err = s.Links.Remove(ctx, fromDoc.Key, toDoc.Key, testOpts())
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	list, _ := s.Links.List(ctx, "docs/api", testOpts())
	if len(list) != 0 {
		t.Errorf("List() after remove returned %d links, want 0", len(list))
	}
}
