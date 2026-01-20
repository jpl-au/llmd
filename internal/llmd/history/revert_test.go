package history_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/core"
	"github.com/jpl-au/llmd/internal/llmd/history"
)

func TestRevert(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "original content", testWriteOpts())
	s.Documents.Write(ctx, "docs/readme", "modified content", testWriteOpts())

	doc, err := s.History.Revert(ctx, "docs/readme", 1, testRevertOpts())
	if err != nil {
		t.Fatalf("Revert() error = %v", err)
	}

	if doc.Content != "original content" {
		t.Errorf("Content = %q, want %q", doc.Content, "original content")
	}
	if doc.Version != 3 {
		t.Errorf("Version = %d, want 3 (new version)", doc.Version)
	}
	if doc.Message != "Reverted to version 1" {
		t.Errorf("Message = %q, want %q", doc.Message, "Reverted to version 1")
	}

	latest, _ := s.Documents.Read(ctx, "docs/readme")
	if latest.Content != "original content" {
		t.Errorf("Latest content = %q, want %q", latest.Content, "original content")
	}
}

func TestRevert_CustomMessage(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "v1", testWriteOpts())
	s.Documents.Write(ctx, "docs/readme", "v2", testWriteOpts())

	revertOpts := testRevertOpts()
	revertOpts.Message = "Rolling back bad change"
	doc, err := s.History.Revert(ctx, "docs/readme", 1, revertOpts)
	if err != nil {
		t.Fatalf("Revert() error = %v", err)
	}

	if doc.Message != "Rolling back bad change" {
		t.Errorf("Message = %q, want %q", doc.Message, "Rolling back bad change")
	}
}

func TestRevert_SkipsUnchanged(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "same content", testWriteOpts())
	s.Documents.Write(ctx, "docs/readme", "different", testWriteOpts())
	s.Documents.Write(ctx, "docs/readme", "same content", testWriteOpts())

	doc, err := s.History.Revert(ctx, "docs/readme", 1, testRevertOpts())
	if err != nil {
		t.Fatalf("Revert() error = %v", err)
	}

	if doc.Version != 3 {
		t.Errorf("Version = %d, want 3 (no new version needed)", doc.Version)
	}
}

func TestRevert_RequiresAuthor(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())

	_, err := s.History.Revert(ctx, "docs/readme", 1, history.RevertOptions{
		WriteContext: core.WriteContext{Source: "cli"},
	})
	if err == nil {
		t.Error("Revert() without author should fail")
	}
}
