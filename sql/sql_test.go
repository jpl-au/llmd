package sql

import (
	"testing"

	"github.com/jpl-au/llmd/sqlite/sqlitetest"
)

func TestExec(t *testing.T) {
	db := sqlitetest.Open(t, "llmd.db")

	if err := Exec(db); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	// Verify tables exist
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
			row, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tt.table).WithContext(t.Context()).ReadRow()
			if err != nil {
				t.Fatalf("table %s query failed: %v", tt.table, err)
			}
			if err := row.Scan(&name); err != nil {
				t.Errorf("table %s not found: %v", tt.table, err)
			}
		})
	}
}

// TestExecHelpOnce checks that every store open leaves exactly one help
// row: the table once gained a row each time a store was opened.
func TestExecHelpOnce(t *testing.T) {
	db := sqlitetest.Open(t, "llmd.db")

	for i := range 3 {
		if err := Exec(db); err != nil {
			t.Fatalf("Exec() call %d error = %v", i+1, err)
		}
	}

	var n int
	var content string
	row, err := db.Query("SELECT COUNT(*), MAX(content) FROM help").WithContext(t.Context()).ReadRow()
	if err != nil {
		t.Fatalf("counting help rows: %v", err)
	}
	if err := row.Scan(&n, &content); err != nil {
		t.Fatalf("counting help rows: %v", err)
	}
	if n != 1 {
		t.Errorf("help rows = %d, want 1", n)
	}
	if content != help {
		t.Errorf("help content does not match the embedded help.md")
	}
}
