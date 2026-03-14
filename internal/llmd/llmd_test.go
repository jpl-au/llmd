package llmd

import (
	"context"
	"testing"
)

func TestOpenMemory(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	defer s.Close()

	if s.Path() != ":memory:" {
		t.Errorf("Path() = %q, want %q", s.Path(), ":memory:")
	}
}

func TestOpenMemory_TablesExist(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Check content table exists
	var name string
	row, err := s.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='content'").WithContext(ctx).ReadRow()
	if err != nil {
		t.Errorf("content table query failed: %v", err)
	} else if err := row.Scan(&name); err != nil {
		t.Errorf("content table not found: %v", err)
	}

	// Check entities table exists
	row, err = s.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='entities'").WithContext(ctx).ReadRow()
	if err != nil {
		t.Errorf("entities table query failed: %v", err)
	} else if err := row.Scan(&name); err != nil {
		t.Errorf("entities table not found: %v", err)
	}

	// Check FTS table exists
	row, err = s.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='content_fts'").WithContext(ctx).ReadRow()
	if err != nil {
		t.Errorf("content_fts table query failed: %v", err)
	} else if err := row.Scan(&name); err != nil {
		t.Errorf("content_fts table not found: %v", err)
	}
}
