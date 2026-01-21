package history_test

import (
	"context"
	"testing"
)

func TestDiff(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	s.Documents.Write(ctx, "docs/readme", "first version", opts)
	s.Documents.Write(ctx, "docs/readme", "second version", opts)

	// Get keys for specific versions
	versions, _ := s.History.List(ctx, "docs/readme")
	v1Key := versions[1].Key // older version
	v2Key := versions[0].Key // newer version

	result, err := s.History.Diff(ctx, v1Key, v2Key)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if result.A.Content != "first version" {
		t.Errorf("A.Content = %q, want %q", result.A.Content, "first version")
	}
	if result.B.Content != "second version" {
		t.Errorf("B.Content = %q, want %q", result.B.Content, "second version")
	}
}

func TestDiff_ByPath(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	s.Documents.Write(ctx, "docs/readme", "readme content", opts)
	s.Documents.Write(ctx, "docs/api", "api content", opts)

	result, err := s.History.Diff(ctx, "docs/readme", "docs/api")
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if result.A.Content != "readme content" {
		t.Errorf("A.Content = %q, want %q", result.A.Content, "readme content")
	}
	if result.B.Content != "api content" {
		t.Errorf("B.Content = %q, want %q", result.B.Content, "api content")
	}
}

func TestDiff_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	s.Documents.Write(ctx, "docs/readme", "content", opts)

	_, err := s.History.Diff(ctx, "docs/readme", "docs/nonexistent")
	if err == nil {
		t.Error("Diff() expected error for nonexistent document")
	}
}
