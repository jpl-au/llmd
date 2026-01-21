package entities_test

import (
	"context"
	"testing"
)

func TestWrite(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	ent, err := s.Entities.Write(ctx, "test:item", `{"name":"foo"}`, testWriteOpts())
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if ent.Namespace != "test:item" {
		t.Errorf("Namespace = %q, want %q", ent.Namespace, "test:item")
	}
	if ent.Value != `{"name":"foo"}` {
		t.Errorf("Value = %q, want %q", ent.Value, `{"name":"foo"}`)
	}
	if len(ent.Key) != 9 {
		t.Errorf("Key length = %d, want 9", len(ent.Key))
	}
	if ent.Author != "test" {
		t.Errorf("Author = %q, want %q", ent.Author, "test")
	}
	if ent.Source != "cli" {
		t.Errorf("Source = %q, want %q", ent.Source, "cli")
	}
}

func TestWrite_WithRelation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	opts := testWriteOpts()
	opts.Relation = "docs/readme"

	ent, err := s.Entities.Write(ctx, "test:item", `{"name":"bar"}`, opts)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if ent.Relation != "docs/readme" {
		t.Errorf("Relation = %q, want %q", ent.Relation, "docs/readme")
	}
}

func TestWrite_NoRelation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	ent, err := s.Entities.Write(ctx, "config:setting", `{"theme":"dark"}`, testWriteOpts())
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if ent.Relation != "" {
		t.Errorf("Relation = %q, want empty", ent.Relation)
	}
}
