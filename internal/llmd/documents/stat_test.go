package documents_test

import (
	"context"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/documents"
)

func TestStat(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	written, _ := s.Documents.Write(ctx, "docs/readme", "# Hello World\n\nThis is content.", testWriteOpts())

	stat, err := s.Documents.Stat(ctx, "docs/readme")
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	if stat.Path != "docs/readme" {
		t.Errorf("Path = %q, want %q", stat.Path, "docs/readme")
	}
	if stat.Key != written.Key {
		t.Errorf("Key = %q, want %q", stat.Key, written.Key)
	}
	if stat.Version != 1 {
		t.Errorf("Version = %d, want 1", stat.Version)
	}
	if stat.Meta == nil {
		t.Error("Meta is nil, want non-nil")
	} else {
		if stat.Meta.Lines != 3 {
			t.Errorf("Meta.Lines = %d, want 3", stat.Meta.Lines)
		}
	}
}

func TestStat_ByKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	written, _ := s.Documents.Write(ctx, "docs/readme", "content", testWriteOpts())

	stat, err := s.Documents.Stat(ctx, written.Key)
	if err != nil {
		t.Fatalf("Stat(key) error = %v", err)
	}

	if stat.Key != written.Key {
		t.Errorf("Key = %q, want %q", stat.Key, written.Key)
	}
	if stat.Path != "docs/readme" {
		t.Errorf("Path = %q, want %q", stat.Path, "docs/readme")
	}
}

func TestStat_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.Documents.Stat(ctx, "nonexistent")
	if err != documents.ErrNotFound {
		t.Errorf("Stat() error = %v, want ErrNotFound", err)
	}
}

func TestStat_Deleted(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	written, _ := s.Documents.Write(ctx, "docs/deleted", "content", testWriteOpts())
	s.Documents.Delete(ctx, "docs/deleted", testDeleteOpts())

	// Stat by key should return ErrDeleted
	stat, err := s.Documents.Stat(ctx, written.Key)
	if err != documents.ErrDeleted {
		t.Errorf("Stat(key) error = %v, want ErrDeleted", err)
	}
	if stat == nil {
		t.Error("Stat should return stat even when deleted")
	}
}
