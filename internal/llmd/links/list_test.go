package links_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/links"
)

func TestList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts()); err != nil {
		t.Fatalf("Write api: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts()); err != nil {
		t.Fatalf("Write models: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/auth", "auth content", testWriteOpts()); err != nil {
		t.Fatalf("Write auth: %v", err)
	}

	if _, err := s.Links.Add(ctx, "docs/api", "docs/models", testOpts()); err != nil {
		t.Fatalf("Add(docs/api -> docs/models): %v", err)
	}
	if _, err := s.Links.Add(ctx, "docs/api", "docs/auth", testOpts()); err != nil {
		t.Fatalf("Add(docs/api -> docs/auth): %v", err)
	}

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

	doc, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts())
	if err != nil {
		t.Fatalf("Write api: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts()); err != nil {
		t.Fatalf("Write models: %v", err)
	}
	if _, err := s.Links.Add(ctx, "docs/api", "docs/models", testOpts()); err != nil {
		t.Fatalf("Add(docs/api -> docs/models): %v", err)
	}

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

	if _, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts()); err != nil {
		t.Fatalf("Write api: %v", err)
	}

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

	if _, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts()); err != nil {
		t.Fatalf("Write api: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts()); err != nil {
		t.Fatalf("Write models: %v", err)
	}
	if _, err := s.Links.Add(ctx, "docs/api", "docs/models", testOpts()); err != nil {
		t.Fatalf("Add(docs/api -> docs/models): %v", err)
	}

	opts := testOpts()
	opts.Direction = links.Outgoing

	list, err := s.Links.List(ctx, "docs/api", opts)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 1 {
		t.Errorf("List() returned %d links, want 1", len(list))
	}
	if list[0].Value.To != "docs/models" {
		t.Errorf("Link Value.To = %q, want %q", list[0].Value.To, "docs/models")
	}
}

func TestList_Incoming(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts()); err != nil {
		t.Fatalf("Write api: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts()); err != nil {
		t.Fatalf("Write models: %v", err)
	}
	if _, err := s.Links.Add(ctx, "docs/api", "docs/models", testOpts()); err != nil {
		t.Fatalf("Add(docs/api -> docs/models): %v", err)
	}

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
	if list[0].Relation != "docs/api" {
		t.Errorf("Link Relation = %q, want %q", list[0].Relation, "docs/api")
	}
}

func TestList_Both(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/api", "api content", testWriteOpts()); err != nil {
		t.Fatalf("Write api: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/models", "models content", testWriteOpts()); err != nil {
		t.Fatalf("Write models: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/auth", "auth content", testWriteOpts()); err != nil {
		t.Fatalf("Write auth: %v", err)
	}

	// api -> models
	if _, err := s.Links.Add(ctx, "docs/api", "docs/models", testOpts()); err != nil {
		t.Fatalf("Add(docs/api -> docs/models): %v", err)
	}
	// auth -> api
	if _, err := s.Links.Add(ctx, "docs/auth", "docs/api", testOpts()); err != nil {
		t.Fatalf("Add(docs/auth -> docs/api): %v", err)
	}

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
