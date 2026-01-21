package entities_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/entities"
)

func TestDelete(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Write an entity
	written, _ := s.Entities.Write(ctx, "test:item", `{"name":"foo"}`, testWriteOpts())

	// Delete it
	err := s.Entities.Delete(ctx, written.Key, testDeleteOpts())
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Should not be readable now
	_, err = s.Entities.Read(ctx, written.Key)
	if !errors.Is(err, entities.ErrNotFound) {
		t.Errorf("Read() after delete should return ErrNotFound, got %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	err := s.Entities.Delete(ctx, "nonexistent", testDeleteOpts())
	if !errors.Is(err, entities.ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestDelete_AlreadyDeleted(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Write and delete
	written, _ := s.Entities.Write(ctx, "test:item", `{"name":"foo"}`, testWriteOpts())
	s.Entities.Delete(ctx, written.Key, testDeleteOpts())

	// Delete again should return not found
	err := s.Entities.Delete(ctx, written.Key, testDeleteOpts())
	if !errors.Is(err, entities.ErrNotFound) {
		t.Errorf("Delete() again error = %v, want ErrNotFound", err)
	}
}
