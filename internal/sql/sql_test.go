package sql

import (
	"testing"

	"github.com/jpl-au/llmd/internal/sqlite/sqlitetest"
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
