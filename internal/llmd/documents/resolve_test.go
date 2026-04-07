package documents_test

import (
	"context"
	"testing"
)

func TestKeyToPath(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	written, err := s.Documents.Write(ctx, "docs/readme", "hello world", testWriteOpts())
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	path, err := s.Documents.KeyToPath(ctx, written.Key)
	if err != nil {
		t.Fatalf("KeyToPath() error = %v", err)
	}
	if path != "docs/readme" {
		t.Errorf("path = %q, want %q", path, "docs/readme")
	}
}

func TestKeyToPath_StableAcrossVersions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	v1, err := s.Documents.Write(ctx, "docs/readme", "v1 content", testWriteOpts())
	if err != nil {
		t.Fatalf("Write v1: %v", err)
	}
	v2, err := s.Documents.Write(ctx, "docs/readme", "v2 content", testWriteOpts())
	if err != nil {
		t.Fatalf("Write v2: %v", err)
	}

	if v1.Key != v2.Key {
		t.Fatalf("Key changed between versions: v1=%q v2=%q", v1.Key, v2.Key)
	}

	path, err := s.Documents.KeyToPath(ctx, v1.Key)
	if err != nil {
		t.Fatalf("KeyToPath() error = %v", err)
	}
	if path != "docs/readme" {
		t.Errorf("path = %q, want %q", path, "docs/readme")
	}
}

func TestKeyToPath_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Documents.KeyToPath(ctx, "zzzzzzzzz")
	if err == nil {
		t.Error("KeyToPath() expected error for unknown key")
	}
}
