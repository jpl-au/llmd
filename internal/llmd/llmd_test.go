package llmd

import (
	"testing"
)

func TestOpenMemory(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("closing store: %v", err)
		}
	})

	if s.Path() != ":memory:" {
		t.Errorf("Path() = %q, want %q", s.Path(), ":memory:")
	}
}

func TestOpenMemory_TablesExist(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("closing store: %v", err)
		}
	})

	tests := []struct {
		table string
	}{
		{"content"},
		{"entities"},
		{"content_fts"},
	}

	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			var name string
			row, err := s.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tt.table).WithContext(t.Context()).ReadRow()
			if err != nil {
				t.Fatalf("table %s query failed: %v", tt.table, err)
			}
			if err := row.Scan(&name); err != nil {
				t.Errorf("table %s not found: %v", tt.table, err)
			}
		})
	}
}
