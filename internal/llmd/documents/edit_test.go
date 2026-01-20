package documents_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestEdit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "hello world", testWriteOpts())

	doc, err := s.Documents.Edit(ctx, "docs/readme", "world", "universe", testEditOpts())
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}

	if doc.Content != "hello universe" {
		t.Errorf("Content = %q, want %q", doc.Content, "hello universe")
	}
	if doc.Version != 2 {
		t.Errorf("Version = %d, want 2", doc.Version)
	}
}

func TestEdit_ReplaceAll(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "foo bar foo baz foo", testWriteOpts())

	opts := testEditOpts()
	opts.ReplaceAll = true
	doc, err := s.Documents.Edit(ctx, "docs/readme", "foo", "qux", opts)
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}

	if doc.Content != "qux bar qux baz qux" {
		t.Errorf("Content = %q, want %q", doc.Content, "qux bar qux baz qux")
	}
}

func TestEdit_NoMatch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "hello world", testWriteOpts())

	_, err := s.Documents.Edit(ctx, "docs/readme", "notfound", "replacement", testEditOpts())
	if !errors.Is(err, documents.ErrNoMatch) {
		t.Errorf("Edit() error = %v, want ErrNoMatch", err)
	}
}
