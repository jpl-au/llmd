package entities_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/entities"
)

func TestPurge(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Write some entities
	ent1, _ := s.Entities.Write(ctx, "test:item", `{"name":"a"}`, testWriteOpts())
	ent2, _ := s.Entities.Write(ctx, "test:item", `{"name":"b"}`, testWriteOpts())
	if _, err := s.Entities.Write(ctx, "test:item", `{"name":"c"}`, testWriteOpts()); err != nil {
		t.Fatalf("Write c: %v", err)
	}

	// Delete some
	if err := s.Entities.Delete(ctx, ent1.Key, testDeleteOpts()); err != nil {
		t.Fatalf("Delete(%s): %v", ent1.Key, err)
	}
	if err := s.Entities.Delete(ctx, ent2.Key, testDeleteOpts()); err != nil {
		t.Fatalf("Delete(%s): %v", ent2.Key, err)
	}

	// Purge should remove 2 entities
	n, err := s.Entities.Purge(ctx)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if n != 2 {
		t.Errorf("Purge() = %d, want 2", n)
	}

	// Only one entity should remain
	list, _ := s.Entities.List(ctx, "test:item", entities.ListOptions{})
	if len(list) != 1 {
		t.Errorf("List() after purge returned %d entities, want 1", len(list))
	}
}

func TestPurge_Empty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Purge with nothing to purge
	n, err := s.Entities.Purge(ctx)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if n != 0 {
		t.Errorf("Purge() = %d, want 0", n)
	}
}

func TestPurge_OnlyDeleted(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Write entities but don't delete any
	if _, err := s.Entities.Write(ctx, "test:item", `{"name":"a"}`, testWriteOpts()); err != nil {
		t.Fatalf("Write a: %v", err)
	}
	if _, err := s.Entities.Write(ctx, "test:item", `{"name":"b"}`, testWriteOpts()); err != nil {
		t.Fatalf("Write b: %v", err)
	}

	// Purge should do nothing
	n, err := s.Entities.Purge(ctx)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}
	if n != 0 {
		t.Errorf("Purge() = %d, want 0", n)
	}
}
