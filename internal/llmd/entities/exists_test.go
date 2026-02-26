package entities_test

import (
	"context"
	"testing"
)

func TestExists(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	ent, _ := s.Entities.Write(ctx, "test:item", `{"name":"foo"}`, testWriteOpts())

	exists, err := s.Entities.Exists(ctx, ent.Key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}
}

func TestExists_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	exists, err := s.Entities.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true, want false")
	}
}

func TestExists_Deleted(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	ent, _ := s.Entities.Write(ctx, "test:item", `{"name":"foo"}`, testWriteOpts())
	if err := s.Entities.Delete(ctx, ent.Key, testDeleteOpts()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	exists, err := s.Entities.Exists(ctx, ent.Key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true for deleted entity, want false")
	}
}

func TestExistsInNamespace(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	opts := testWriteOpts()
	opts.Relation = "docs/readme"
	if _, err := s.Entities.Write(ctx, "test:item", `{"x":1}`, opts); err != nil {
		t.Fatalf("Write: %v", err)
	}

	exists, err := s.Entities.ExistsInNamespace(ctx, "test:item", "docs/readme")
	if err != nil {
		t.Fatalf("ExistsInNamespace() error = %v", err)
	}
	if !exists {
		t.Error("ExistsInNamespace() = false, want true")
	}
}

func TestExistsInNamespace_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	opts := testWriteOpts()
	opts.Relation = "docs/readme"
	if _, err := s.Entities.Write(ctx, "test:item", `{"x":1}`, opts); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Different relation
	exists, err := s.Entities.ExistsInNamespace(ctx, "test:item", "docs/other")
	if err != nil {
		t.Fatalf("ExistsInNamespace() error = %v", err)
	}
	if exists {
		t.Error("ExistsInNamespace() = true, want false")
	}
}

func TestFindByValue(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	opts := testWriteOpts()
	opts.Relation = "docs/readme"
	if _, err := s.Entities.Write(ctx, "test:item", `{"tag":"important"}`, opts); err != nil {
		t.Fatalf("Write important: %v", err)
	}
	if _, err := s.Entities.Write(ctx, "test:item", `{"tag":"draft"}`, opts); err != nil {
		t.Fatalf("Write draft: %v", err)
	}

	// Find by JSON path
	ent, err := s.Entities.FindByValue(ctx, "test:item", "docs/readme", "$.tag", "important")
	if err != nil {
		t.Fatalf("FindByValue() error = %v", err)
	}

	if ent.Value != `{"tag":"important"}` {
		t.Errorf("FindByValue() Value = %q, want %q", ent.Value, `{"tag":"important"}`)
	}
}

func TestFindByValue_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	opts := testWriteOpts()
	opts.Relation = "docs/readme"
	if _, err := s.Entities.Write(ctx, "test:item", `{"tag":"important"}`, opts); err != nil {
		t.Fatalf("Write: %v", err)
	}

	_, err := s.Entities.FindByValue(ctx, "test:item", "docs/readme", "$.tag", "nonexistent")
	if err == nil {
		t.Error("FindByValue() should return error for nonexistent value")
	}
}
