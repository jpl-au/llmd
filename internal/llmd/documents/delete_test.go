package documents_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestDelete(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	err := s.Documents.Delete(ctx, "docs/readme", testDeleteOpts())
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	list, _ := s.Documents.List(ctx)
	if len(list) != 0 {
		t.Errorf("List() after delete returned %d items, want 0", len(list))
	}

	_, err = s.Documents.Read(ctx, "docs/readme")
	if !errors.Is(err, documents.ErrDeleted) {
		t.Errorf("Read() after delete error = %v, want ErrDeleted", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	err := s.Documents.Delete(ctx, "nonexistent", testDeleteOpts())
	if !errors.Is(err, documents.ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}
