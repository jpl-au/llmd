package links_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/links"
)

func TestExists(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts())
	s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts())
	s.Links.Add(ctx, "docs/a", "docs/b", testOpts())

	if ok, err := s.Links.Exists(ctx, "docs/a", "docs/b"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if !ok {
		t.Error("Exists() = false, want true")
	}
}

func TestExists_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts())
	s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts())

	if ok, err := s.Links.Exists(ctx, "docs/a", "docs/b"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if ok {
		t.Error("Exists() = true, want false")
	}
}

func TestExists_WithLabel(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts())
	s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts())

	opts := testOpts()
	opts.Label = "related"
	s.Links.Add(ctx, "docs/a", "docs/b", opts)

	// Should find with matching label
	if ok, err := s.Links.Exists(ctx, "docs/a", "docs/b", links.Options{Label: "related"}); err != nil {
		t.Fatalf("Exists(label=related) error = %v", err)
	} else if !ok {
		t.Error("Exists(label=related) = false, want true")
	}

	// Should not find with different label
	if ok, err := s.Links.Exists(ctx, "docs/a", "docs/b", links.Options{Label: "other"}); err != nil {
		t.Fatalf("Exists(label=other) error = %v", err)
	} else if ok {
		t.Error("Exists(label=other) = true, want false")
	}

	// Should find with no label filter
	if ok, err := s.Links.Exists(ctx, "docs/a", "docs/b"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if !ok {
		t.Error("Exists() = false, want true")
	}
}

func TestExists_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	docA, _ := s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts())
	docB, _ := s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts())
	s.Links.Add(ctx, "docs/a", "docs/b", testOpts())

	if ok, err := s.Links.Exists(ctx, docA.Key, docB.Key); err != nil {
		t.Fatalf("Exists(keys) error = %v", err)
	} else if !ok {
		t.Error("Exists(keys) = false, want true")
	}
}

func TestExists_RemovedLink(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts())
	s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts())
	s.Links.Add(ctx, "docs/a", "docs/b", testOpts())
	s.Links.Remove(ctx, "docs/a", "docs/b", testOpts())

	if ok, err := s.Links.Exists(ctx, "docs/a", "docs/b"); err != nil {
		t.Fatalf("Exists() error = %v", err)
	} else if ok {
		t.Error("Exists() = true for removed link, want false")
	}
}
