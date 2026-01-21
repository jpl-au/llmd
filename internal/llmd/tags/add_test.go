package tags_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/tags"
)

func TestAdd(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())

	tag, err := s.Tags.Add(ctx, "docs/readme", "important", testOpts())
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if tag.Value.Tag != "important" {
		t.Errorf("Value.Tag = %q, want %q", tag.Value.Tag, "important")
	}
	if tag.Relation != "docs/readme" {
		t.Errorf("Relation = %q, want %q", tag.Relation, "docs/readme")
	}
	if len(tag.Key) != 9 {
		t.Errorf("Key length = %d, want 9", len(tag.Key))
	}
}

func TestAdd_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	doc, _ := s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())

	// Add tag by document key
	tag, err := s.Tags.Add(ctx, doc.Key, "important", testOpts())
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if tag.Relation != "docs/readme" {
		t.Errorf("Relation = %q, want %q", tag.Relation, "docs/readme")
	}
}

func TestAdd_Duplicate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())

	tag1, _ := s.Tags.Add(ctx, "docs/readme", "important", testOpts())
	tag2, err := s.Tags.Add(ctx, "docs/readme", "important", testOpts())

	// Should return existing tag with ErrExists
	if err != tags.ErrExists {
		t.Fatalf("Add() error = %v, want ErrExists", err)
	}
	if tag2.Key != tag1.Key {
		t.Errorf("Duplicate add should return existing tag")
	}
}

func TestAdd_MultipleTags(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())

	s.Tags.Add(ctx, "docs/readme", "important", testOpts())
	s.Tags.Add(ctx, "docs/readme", "v1", testOpts())
	s.Tags.Add(ctx, "docs/readme", "needs-review", testOpts())

	tags, err := s.Tags.List(ctx, "docs/readme", testOpts())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(tags) != 3 {
		t.Errorf("List() returned %d tags, want 3", len(tags))
	}
}
