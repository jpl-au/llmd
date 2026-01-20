package sql

import (
	stdsql "database/sql"
	"testing"

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
	tables := []string{"content", "entities", "content_fts"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func openTestDB() (*stdsql.DB, error) {
	return stdsql.Open("sqlite", ":memory:")
}
