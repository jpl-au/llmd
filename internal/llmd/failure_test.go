package llmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/messages"
)

// TestVacuumWithWorkDB checks that a vacuum also compacts the work
// database when one has been opened.
func TestVacuumWithWorkDB(t *testing.T) {
	s := testStore(t)
	if _, err := s.Messages.Send(t.Context(), messages.SendOptions{Type: "note", Payload: "hi", Author: "test", Source: "cli"}); err != nil {
		t.Fatalf("Messages.Send() error = %v", err)
	}
	if s.WorkDB() == nil {
		t.Fatal("WorkDB() = nil after a send")
	}
	result, err := s.Vacuum(t.Context(), llmd.VacuumOptions{})
	if err != nil {
		t.Fatalf("Vacuum() with work database error = %v", err)
	}
	if result.Total() != 0 {
		t.Errorf("Vacuum() purged %d rows from an undeleted store, want 0", result.Total())
	}
	msgs, err := s.Messages.Pending(t.Context(), "anyone", 0)
	if err != nil || len(msgs) != 1 {
		t.Errorf("Pending() after vacuum = %v, %v; want the one message kept", msgs, err)
	}
}

// TestOpenErrors covers the ways a store fails to open or initialise:
// a missing store, a parent path that is a file, and a database path
// that is a directory.
func TestOpenErrors(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, dir string) string // returns the db path
		init    bool
		wantMsg string
	}{
		{
			name:    "open missing store",
			prepare: func(_ *testing.T, dir string) string { return filepath.Join(dir, "llmd.db") },
			wantMsg: "there is no store at",
		},
		{
			name: "init under a file",
			prepare: func(t *testing.T, dir string) string {
				f := filepath.Join(dir, "file")
				if err := os.WriteFile(f, nil, 0o644); err != nil {
					t.Fatal(err)
				}
				return filepath.Join(f, "llmd.db")
			},
			init:    true,
			wantMsg: "creating directory",
		},
		{
			name: "init over an existing store",
			prepare: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "llmd.db")
				s, err := llmd.Init(p)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
				return p
			},
			init:    true,
			wantMsg: "already exists",
		},
		{
			name: "open a directory",
			prepare: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "llmd.db")
				if err := os.Mkdir(p, 0o755); err != nil {
					t.Fatal(err)
				}
				return p
			},
			wantMsg: "opening database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.prepare(t, t.TempDir())
			var err error
			var s *llmd.Store
			if tt.init {
				s, err = llmd.Init(path)
			} else {
				s, err = llmd.Open(path)
			}
			if s != nil {
				if cerr := s.Close(); cerr != nil {
					t.Errorf("Close() error = %v", cerr)
				}
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error = %v, want to contain %q", err, tt.wantMsg)
			}
		})
	}
}
