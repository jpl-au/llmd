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
	s.Entities.Write(ctx, "test:item", `{"name":"a"}`, testWriteOpts())
	s.Entities.Write(ctx, "test:item", `{"name":"b"}`, testWriteOpts())
	s.Entities.Write(ctx, "other:item", `{"name":"c"}`, testWriteOpts())

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

	s.Entities.Write(ctx, "test:item", `{"x":1}`, opts1)
	s.Entities.Write(ctx, "test:item", `{"x":2}`, opts1) // same relation
	s.Entities.Write(ctx, "test:item", `{"x":3}`, opts2) // different relation

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
	for i := 0; i < 5; i++ {
		s.Entities.Write(ctx, "test:item", `{"i":1}`, testWriteOpts())
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
	s.Entities.Write(ctx, "test:item", `{"name":"b"}`, testWriteOpts())

	// Delete first one
	s.Entities.Delete(ctx, ent1.Key, testDeleteOpts())

	list, err := s.Entities.List(ctx, "test:item", entities.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 1 {
		t.Errorf("List() returned %d entities, want 1", len(list))
	}
}
