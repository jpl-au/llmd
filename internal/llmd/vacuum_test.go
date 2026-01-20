package llmd_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/core"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/internal/llmd/tags"
)

func testStore(t *testing.T) *llmd.Store {
	t.Helper()
	s, err := llmd.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func testWriteOpts() documents.WriteOptions {
	return documents.WriteOptions{WriteContext: core.WriteContext{Author: "test", Source: "cli"}}
}

func testDeleteOpts() documents.DeleteOptions {
	return documents.DeleteOptions{WriteContext: core.WriteContext{Author: "test", Source: "cli"}}
}

func testTagOpts() tags.Options {
	return tags.Options{WriteContext: core.WriteContext{Author: "test", Source: "cli"}}
}

func testLinkOpts() links.Options {
	return links.Options{WriteContext: core.WriteContext{Author: "test", Source: "cli"}}
}

func TestVacuum(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create documents
	s.Documents.Write(ctx, "docs/a", "content a", testWriteOpts())
	s.Documents.Write(ctx, "docs/b", "content b", testWriteOpts())

	// Add tags and links
	s.Tags.Add(ctx, "docs/a", "important", testTagOpts())
	s.Tags.Add(ctx, "docs/a", "review", testTagOpts())
	s.Links.Add(ctx, "docs/a", "docs/b", testLinkOpts())

	// Soft delete some items
	s.Documents.Delete(ctx, "docs/a", testDeleteOpts())
	s.Tags.Remove(ctx, "docs/a", "important", testTagOpts())
	s.Links.Remove(ctx, "docs/a", "docs/b", testLinkOpts())

	// Vacuum
	result, err := s.Vacuum(ctx)
	if err != nil {
		t.Fatalf("Vacuum() error = %v", err)
	}

	if result.Documents != 1 {
		t.Errorf("Documents = %d, want 1", result.Documents)
	}
	// Tags associated with deleted doc are also soft-deleted
	if result.Tags < 1 {
		t.Errorf("Tags = %d, want >= 1", result.Tags)
	}
	if result.Links != 1 {
		t.Errorf("Links = %d, want 1", result.Links)
	}
	if result.Total() < 3 {
		t.Errorf("Total() = %d, want >= 3", result.Total())
	}
}

func TestVacuum_Empty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	result, err := s.Vacuum(ctx)
	if err != nil {
		t.Fatalf("Vacuum() error = %v", err)
	}

	if result.Total() != 0 {
		t.Errorf("Total() = %d, want 0", result.Total())
	}
}
