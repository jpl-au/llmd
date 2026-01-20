package links_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/links"
)

func TestList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts())
	s.Documents.Write(ctx, "docs/auth", "auth content", testWriteOpts())

	s.Links.Add(ctx, "docs/api", "docs/models", testOpts())
	s.Links.Add(ctx, "docs/api", "docs/auth", testOpts())

	list, err := s.Links.List(ctx, "docs/api", testOpts())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 2 {
		t.Errorf("List() returned %d links, want 2", len(list))
	}
}

func TestList_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	doc, _ := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts())
	s.Links.Add(ctx, "docs/api", "docs/models", testOpts())

	list, err := s.Links.List(ctx, doc.Key, testOpts())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 1 {
		t.Errorf("List() returned %d links, want 1", len(list))
	}
}

func TestList_Empty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())

	list, err := s.Links.List(ctx, "docs/api", testOpts())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 0 {
		t.Errorf("List() returned %d links, want 0", len(list))
	}
}

func TestList_Outgoing(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts())
	s.Links.Add(ctx, "docs/api", "docs/models", testOpts())

	opts := testOpts()
	opts.Direction = links.Outgoing

	list, err := s.Links.List(ctx, "docs/api", opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 1 {
		t.Errorf("List() returned %d links, want 1", len(list))
	}
	if list[0].To != "docs/models" {
		t.Errorf("Link To = %q, want %q", list[0].To, "docs/models")
	}
}

func TestList_Incoming(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts())
	s.Links.Add(ctx, "docs/api", "docs/models", testOpts())

	opts := testOpts()
	opts.Direction = links.Incoming

	// Query incoming links TO docs/models
	list, err := s.Links.List(ctx, "docs/models", opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 1 {
		t.Errorf("List() returned %d links, want 1", len(list))
	}
	if list[0].From != "docs/api" {
		t.Errorf("Link From = %q, want %q", list[0].From, "docs/api")
	}
}

func TestList_Both(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts())
	s.Documents.Write(ctx, "docs/auth", "auth content", testWriteOpts())

	// api -> models
	s.Links.Add(ctx, "docs/api", "docs/models", testOpts())
	// auth -> api
	s.Links.Add(ctx, "docs/auth", "docs/api", testOpts())

	opts := testOpts()
	opts.Direction = links.Both

	// Query all links involving docs/api
	list, err := s.Links.List(ctx, "docs/api", opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 2 {
		t.Errorf("List() returned %d links, want 2", len(list))
	}
}
