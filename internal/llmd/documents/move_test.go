package documents_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestMove(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/old", "content", testWriteOpts())

	err := s.Documents.Move(ctx, "docs/old", "docs/new", testMoveOpts())
	if err != nil {
		t.Fatalf("Move() error = %v", err)
	}

	_, err = s.Documents.Read(ctx, "docs/old")
	if !errors.Is(err, documents.ErrNotFound) {
		t.Errorf("Read(old) error = %v, want ErrNotFound", err)
	}

	doc, err := s.Documents.Read(ctx, "docs/new")
	if err != nil {
		t.Fatalf("Read(new) error = %v", err)
	}
	if doc.Content != "content" {
		t.Errorf("Content = %q, want %q", doc.Content, "content")
	}
}

func TestMove_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	err := s.Documents.Move(ctx, "nonexistent", "docs/new", testMoveOpts())
	if !errors.Is(err, documents.ErrNotFound) {
		t.Errorf("Move() error = %v, want ErrNotFound", err)
	}
}
