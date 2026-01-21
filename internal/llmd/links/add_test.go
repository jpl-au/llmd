package links_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/links"
)

func TestAdd(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts())

	link, err := s.Links.Add(ctx, "docs/api", "docs/models", testOpts())
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if link.From != "docs/api" {
		t.Errorf("From = %q, want %q", link.From, "docs/api")
	}
	if link.To != "docs/models" {
		t.Errorf("To = %q, want %q", link.To, "docs/models")
	}
	if len(link.Key) != 9 {
		t.Errorf("Key length = %d, want 9", len(link.Key))
	}
}

func TestAdd_WithLabel(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	s.Documents.Write(ctx, "docs/auth", "auth content", testWriteOpts())

	opts := testOpts()
	opts.Label = "requires"

	link, err := s.Links.Add(ctx, "docs/api", "docs/auth", opts)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if link.Label != "requires" {
		t.Errorf("Label = %q, want %q", link.Label, "requires")
	}
}

func TestAdd_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	fromDoc, _ := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	toDoc, _ := s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts())

	link, err := s.Links.Add(ctx, fromDoc.Key, toDoc.Key, testOpts())
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if link.From != "docs/api" {
		t.Errorf("From = %q, want %q", link.From, "docs/api")
	}
	if link.To != "docs/models" {
		t.Errorf("To = %q, want %q", link.To, "docs/models")
	}
}

func TestAdd_SelfLink(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())

	_, err := s.Links.Add(ctx, "docs/api", "docs/api", testOpts())
	if !errors.Is(err, links.ErrSelfLink) {
		t.Errorf("Add() error = %v, want ErrSelfLink", err)
	}
}

func TestAdd_Duplicate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts())

	link1, _ := s.Links.Add(ctx, "docs/api", "docs/models", testOpts())
	link2, err := s.Links.Add(ctx, "docs/api", "docs/models", testOpts())

	// Should return existing link with ErrExists
	if !errors.Is(err, links.ErrExists) {
		t.Fatalf("Add() error = %v, want ErrExists", err)
	}
	if link2.Key != link1.Key {
		t.Errorf("Duplicate add should return existing link")
	}
}

func TestAdd_MultipleLabels(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	s.Documents.Write(ctx, "docs/auth", "auth content", testWriteOpts())

	opts1 := testOpts()
	opts1.Label = "requires"

	opts2 := testOpts()
	opts2.Label = "related"

	link1, _ := s.Links.Add(ctx, "docs/api", "docs/auth", opts1)
	link2, err := s.Links.Add(ctx, "docs/api", "docs/auth", opts2)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Different labels should create different links
	if link2.Key == link1.Key {
		t.Errorf("Different labels should create different links")
	}
}
