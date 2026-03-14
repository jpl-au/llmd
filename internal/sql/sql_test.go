package sql

import (
	"context"
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
	defer db.Close()

	if err := Exec(db); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	// Verify tables exist
	ctx := context.Background()
	tables := []string{"content", "entities", "content_fts"}
	for _, table := range tables {
		var name string
		row, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).WithContext(ctx).ReadRow()
		if err != nil {
			t.Errorf("table %s query failed: %v", table, err)
			continue
		}
		if err := row.Scan(&name); err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func openTestDB() (*qwr.Manager, error) {
	rp := profile.ReadBalanced().WithForeignKeys(true)
	wp := profile.WriteBalanced().WithForeignKeys(true)
	return qwr.New("file::memory:?cache=shared").Reader(rp).Writer(wp).Open()
}
