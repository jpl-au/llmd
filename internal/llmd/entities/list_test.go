package entities_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/entities"
)

func TestList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Write multiple entities
	if _, err := s.Entities.Write(ctx, "test:item", `{"name":"a"}`, testWriteOpts()); err != nil {
		t.Fatalf("Write(test:item a): %v", err)
	}
	if _, err := s.Entities.Write(ctx, "test:item", `{"name":"b"}`, testWriteOpts()); err != nil {
		t.Fatalf("Write(test:item b): %v", err)
	}
	if _, err := s.Entities.Write(ctx, "other:item", `{"name":"c"}`, testWriteOpts()); err != nil {
		t.Fatalf("Write(other:item c): %v", err)
	}

	// List by namespace
	list, err := s.Entities.List(ctx, "test:item", entities.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 2 {
		t.Errorf("List() returned %d entities, want 2", len(list))
	}
}

func TestList_WithRelation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	opts1 := testWriteOpts()
	opts1.Relation = "docs/a"

	opts2 := testWriteOpts()
	opts2.Relation = "docs/b"

	if _, err := s.Entities.Write(ctx, "test:item", `{"x":1}`, opts1); err != nil {
		t.Fatalf("Write(x:1): %v", err)
	}
	if _, err := s.Entities.Write(ctx, "test:item", `{"x":2}`, opts1); err != nil { // same relation
		t.Fatalf("Write(x:2): %v", err)
	}
	if _, err := s.Entities.Write(ctx, "test:item", `{"x":3}`, opts2); err != nil { // different relation
		t.Fatalf("Write(x:3): %v", err)
	}

	// Filter by relation
	list, err := s.Entities.List(ctx, "test:item", entities.ListOptions{Relation: "docs/a"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 2 {
		t.Errorf("List(relation=docs/a) returned %d entities, want 2", len(list))
	}
}

func TestList_Empty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	list, err := s.Entities.List(ctx, "nonexistent:ns", entities.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 0 {
		t.Errorf("List() returned %d entities, want 0", len(list))
	}
}

func TestList_Limit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Write 5 entities
	for range 5 {
		if _, err := s.Entities.Write(ctx, "test:item", `{"i":1}`, testWriteOpts()); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}

	// List with limit
	list, err := s.Entities.List(ctx, "test:item", entities.ListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 3 {
		t.Errorf("List(limit=3) returned %d entities, want 3", len(list))
	}
}

func TestList_ExcludesDeleted(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	ent1, _ := s.Entities.Write(ctx, "test:item", `{"name":"a"}`, testWriteOpts())
	if _, err := s.Entities.Write(ctx, "test:item", `{"name":"b"}`, testWriteOpts()); err != nil {
		t.Fatalf("Write b: %v", err)
	}

	// Delete first one
	if err := s.Entities.Delete(ctx, ent1.Key, testDeleteOpts()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list, err := s.Entities.List(ctx, "test:item", entities.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 1 {
		t.Errorf("List() returned %d entities, want 1", len(list))
	}
}
