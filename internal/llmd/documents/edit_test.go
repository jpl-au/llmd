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

	if _, err := s.Documents.Write(ctx, "docs/readme", "hello world", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

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

	if _, err := s.Documents.Write(ctx, "docs/readme", "foo bar foo baz foo", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

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

	if _, err := s.Documents.Write(ctx, "docs/readme", "hello world", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	_, err := s.Documents.Edit(ctx, "docs/readme", "notfound", "replacement", testEditOpts())
	if !errors.Is(err, documents.ErrNoMatch) {
		t.Errorf("Edit() error = %v, want ErrNoMatch", err)
	}
}

func TestEdit_NotUnique(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/readme", "foo bar foo", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	_, err := s.Documents.Edit(ctx, "docs/readme", "foo", "qux", testEditOpts())
	if !errors.Is(err, documents.ErrNotUnique) {
		t.Errorf("Edit() error = %v, want ErrNotUnique", err)
	}

	// The document must not have been touched: the failed edit should
	// not produce a new version.
	doc, err := s.Documents.Read(ctx, "docs/readme")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if doc.Version != 1 {
		t.Errorf("Version = %d, want 1 (no version should be created on failed edit)", doc.Version)
	}
	if doc.Content != "foo bar foo" {
		t.Errorf("Content = %q, want unchanged", doc.Content)
	}
}

func TestEdit_NoOp(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/readme", "hello", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	_, err := s.Documents.Edit(ctx, "docs/readme", "hello", "hello", testEditOpts())
	if !errors.Is(err, documents.ErrNoOp) {
		t.Errorf("Edit() error = %v, want ErrNoOp", err)
	}
}

// TestEdit_DisambiguatedByContext shows the documented escape hatch:
// when the bare search string is ambiguous, the agent should expand it
// with surrounding context until exactly one match remains.
func TestEdit_DisambiguatedByContext(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.Documents.Write(ctx, "docs/readme", "foo bar foo baz", testWriteOpts()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	doc, err := s.Documents.Edit(ctx, "docs/readme", "foo baz", "qux baz", testEditOpts())
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if doc.Content != "foo bar qux baz" {
		t.Errorf("Content = %q, want %q", doc.Content, "foo bar qux baz")
	}
}
