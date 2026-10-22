package llmd

import (
	"os"
	"path/filepath"
	"testing"
)

// initStore creates a store under a temporary .llmd directory.
func initStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".llmd", "llmd.db")
	s, err := Init(path)
	if err != nil {
		t.Fatalf("Init(%q) error = %v", path, err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("closing store: %v", err)
		}
	})
	return s, path
}

func TestInit(t *testing.T) {
	s, path := initStore(t)

	if s.Path() != path {
		t.Errorf("Path() = %q, want %q", s.Path(), path)
	}
	if s.Dir() != filepath.Dir(path) {
		t.Errorf("Dir() = %q, want %q", s.Dir(), filepath.Dir(path))
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Init(%q) did not create the database: %v", path, err)
	}
	if _, err := Init(path); err == nil {
		t.Errorf("Init(%q) on an existing store error = nil, want error", path)
	}
}

func TestOpen(t *testing.T) {
	tests := []struct {
		name    string
		exists  bool
		wantErr bool
	}{
		{"existing store", true, false},
		{"missing store", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".llmd", "llmd.db")
			if tt.exists {
				s, err := Init(path)
				if err != nil {
					t.Fatalf("Init(%q) error = %v", path, err)
				}
				if err := s.Close(); err != nil {
					t.Fatalf("closing initial store: %v", err)
				}
			}

			s, err := Open(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Open(%q) error = %v, wantErr %v", path, err, tt.wantErr)
			}
			if err == nil {
				if cerr := s.Close(); cerr != nil {
					t.Errorf("closing store: %v", cerr)
				}
			}
		})
	}
}

func TestInit_TablesExist(t *testing.T) {
	s, _ := initStore(t)

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

// TestWorkDB_OnDemand verifies work.db is created lazily next to llmd.db
// on the first work-side operation and not before.
func TestWorkDB_OnDemand(t *testing.T) {
	s, path := initStore(t)
	workPath := filepath.Join(filepath.Dir(path), "work.db")

	if _, err := os.Stat(workPath); err == nil {
		t.Fatalf("work.db exists before any work operation")
	}
	if s.workOpen() {
		t.Errorf("workOpen() = true before any work operation")
	}

	if db := s.WorkDB(); db == nil {
		t.Fatalf("WorkDB() = nil")
	}
	if _, err := os.Stat(workPath); err != nil {
		t.Errorf("work.db not created by WorkDB(): %v", err)
	}
	if !s.workOpen() {
		t.Errorf("workOpen() = false after WorkDB()")
	}
}
