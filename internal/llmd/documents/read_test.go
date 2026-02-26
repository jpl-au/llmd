package documents_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestRead(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	opts := testWriteOpts()
	opts.Message = "initial"
	if _, err := s.Documents.Write(ctx, "docs/readme", "# Hello", opts); err != nil {
		t.Fatalf("Write: %v", err)
	}

	doc, err := s.Documents.Read(ctx, "docs/readme")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if doc.Content != "# Hello" {
		t.Errorf("Content = %q, want %q", doc.Content, "# Hello")
	}
	if doc.Message != "initial" {
		t.Errorf("Message = %q, want %q", doc.Message, "initial")
	}
}

func TestRead_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Documents.Read(ctx, "nonexistent")
	if !errors.Is(err, documents.ErrNotFound) {
		t.Errorf("Read() error = %v, want ErrNotFound", err)
	}
}

func TestRead_SpecificVersion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	if _, err := s.Documents.Write(ctx, "docs/readme", "version 1", opts); err != nil {
		t.Fatalf("Write v1: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/readme", "version 2", opts); err != nil {
		t.Fatalf("Write v2: %v", err)
	}
	if _, err := s.Documents.Write(ctx, "docs/readme", "version 3", opts); err != nil {
		t.Fatalf("Write v3: %v", err)
	}

	v := 2
	doc, err := s.Documents.Read(ctx, "docs/readme", documents.ReadOptions{Version: &v})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if doc.Content != "version 2" {
		t.Errorf("Content = %q, want %q", doc.Content, "version 2")
	}
	if doc.Version != 2 {
		t.Errorf("Version = %d, want 2", doc.Version)
	}
}

func TestReadByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	written, err := s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	doc, err := s.Documents.ReadByKey(ctx, written.Key)
	if err != nil {
		t.Fatalf("ReadByKey() error = %v", err)
	}

	if doc.Content != "content" {
		t.Errorf("Content = %q, want %q", doc.Content, "content")
	}
}
