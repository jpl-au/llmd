package history_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd/history"
)

func TestList(t *testing.T) {
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

	versions, err := s.History.List(ctx, "docs/readme")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(versions) != 3 {
		t.Errorf("List() returned %d versions, want 3", len(versions))
	}

	if versions[0].Version != 3 {
		t.Errorf("versions[0].Version = %d, want 3", versions[0].Version)
	}
	if versions[1].Version != 2 {
		t.Errorf("versions[1].Version = %d, want 2", versions[1].Version)
	}
	if versions[2].Version != 1 {
		t.Errorf("versions[2].Version = %d, want 1", versions[2].Version)
	}
}

func TestList_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	_, err := s.History.List(ctx, "nonexistent")
	if !errors.Is(err, history.ErrNotFound) {
		t.Errorf("List() error = %v, want ErrNotFound", err)
	}
}

func TestList_Limit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	opts := testWriteOpts()

	for i := 1; i <= 5; i++ {
		if _, err := s.Documents.Write(ctx, "docs/readme", fmt.Sprintf("version %d", i), opts); err != nil {
			t.Fatalf("Write v%d: %v", i, err)
		}
	}

	versions, err := s.History.List(ctx, "docs/readme", history.ListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(versions) != 2 {
		t.Errorf("List(Limit=2) returned %d versions, want 2", len(versions))
	}

	if versions[0].Version != 5 {
		t.Errorf("versions[0].Version = %d, want 5", versions[0].Version)
	}
}
