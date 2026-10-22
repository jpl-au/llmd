package sql

import (
	"testing"

	"github.com/jpl-au/qwr"
	"github.com/jpl-au/qwr/profile"

	_ "modernc.org/sqlite"
)

func TestExec(t *testing.T) {
	db, err := openTestDB()
	if err != nil {
		t.Fatalf("openTestDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing database: %v", err)
		}
	})

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

func openTestDB() (*qwr.Manager, error) {
	rp := profile.ReadBalanced().WithForeignKeys(true)
	wp := profile.WriteBalanced().WithForeignKeys(true)
	return qwr.New("file::memory:?cache=shared").Reader(rp).Writer(wp).Open()
}
