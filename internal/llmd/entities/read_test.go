package entities_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/entities"
)

func TestRead(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Write an entity first
	written, err := s.Entities.Write(ctx, "test:item", `{"name":"foo"}`, testWriteOpts())
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Read it back
	ent, err := s.Entities.Read(ctx, written.Key)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if ent.Key != written.Key {
		t.Errorf("Key = %q, want %q", ent.Key, written.Key)
	}
	if ent.Namespace != "test:item" {
		t.Errorf("Namespace = %q, want %q", ent.Namespace, "test:item")
	}
	if ent.Value != `{"name":"foo"}` {
		t.Errorf("Value = %q, want %q", ent.Value, `{"name":"foo"}`)
	}
}

func TestRead_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Entities.Read(ctx, "nonexistent")
	if !errors.Is(err, entities.ErrNotFound) {
		t.Errorf("Read() error = %v, want ErrNotFound", err)
	}
}

func TestRead_WithRelation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	opts := testWriteOpts()
	opts.Relation = "docs/readme"

	written, _ := s.Entities.Write(ctx, "test:item", `{"x":1}`, opts)

	ent, err := s.Entities.Read(ctx, written.Key)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if ent.Relation != "docs/readme" {
		t.Errorf("Relation = %q, want %q", ent.Relation, "docs/readme")
	}
}
