package llmd

import (
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

	// Check content table exists
	var name string
	err = s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='content'").Scan(&name)
	if err != nil {
		t.Errorf("content table not found: %v", err)
	}

	// Check entities table exists
	err = s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='entities'").Scan(&name)
	if err != nil {
		t.Errorf("entities table not found: %v", err)
	}

	// Check FTS table exists
	err = s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='content_fts'").Scan(&name)
	if err != nil {
		t.Errorf("content_fts table not found: %v", err)
	}
}
