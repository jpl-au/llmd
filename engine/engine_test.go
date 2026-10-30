package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/messages"
	"github.com/jpl-au/llmd/engine/tasks"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/sqlite"
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

// TestWorkDB_Failure covers the ways work.db can fail to open or
// create its schema. WorkDB must return nil, leave the store without a work
// connection so a later call retries, and work-side domains must
// report the database as unavailable rather than panic.
func TestWorkDB_Failure(t *testing.T) {
	tests := []struct {
		name   string
		breach func(t *testing.T, dir, workPath string)
	}{
		{
			name: "work.db path is a directory",
			breach: func(t *testing.T, _, workPath string) {
				if err := os.Mkdir(workPath, 0o755); err != nil {
					t.Fatalf("Mkdir(%q) error = %v", workPath, err)
				}
			},
		},
		{
			name: ".llmd directory is read-only",
			breach: func(t *testing.T, dir, _ string) {
				if runtime.GOOS == "windows" {
					t.Skip("directory permissions are not enforced on Windows")
				}
				if os.Geteuid() == 0 {
					t.Skip("root bypasses directory permissions")
				}
				if err := os.Chmod(dir, 0o555); err != nil {
					t.Fatalf("Chmod(%q) error = %v", dir, err)
				}
				// Registered after initStore's Close, so it runs first
				// and the store and temp directory can be cleaned up.
				t.Cleanup(func() {
					if err := os.Chmod(dir, 0o755); err != nil {
						t.Errorf("restoring permissions on %q: %v", dir, err)
					}
				})
			},
		},
		{
			// A history table without the audited columns makes the
			// index creation in the work schema fail, so the database
			// opens but its schema cannot be created.
			name: "existing work.db with incompatible schema",
			breach: func(t *testing.T, _, workPath string) {
				db, err := sqlite.Open(workPath, sqlite.Settings{})
				if err != nil {
					t.Fatalf("sqlite.Open(%q) error = %v", workPath, err)
				}
				if _, err := db.Query(`CREATE TABLE history (id INTEGER PRIMARY KEY)`).Exec(); err != nil {
					t.Fatalf("creating conflicting table: %v", err)
				}
				if err := db.Close(); err != nil {
					t.Fatalf("closing seeded work.db: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, path := initStore(t)
			dir := filepath.Dir(path)
			tt.breach(t, dir, filepath.Join(dir, "work.db"))

			if db := s.WorkDB(); db != nil {
				t.Errorf("WorkDB() = %v, want nil", db)
			}
			if s.work != nil {
				t.Errorf("work = %v after failed WorkDB(), want nil so the next call retries", s.work)
			}
			_, err := s.Messages.Send(t.Context(), messages.SendOptions{Type: "direct", Author: "test", Source: "cli"})
			if err == nil {
				t.Errorf("Messages.Send() error = nil after failed WorkDB(), want error")
			}
		})
	}
}

// walSize returns the size of path, or 0 when it does not exist. The
// WAL is either truncated to zero bytes or removed by a checkpoint.
func walSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	return info.Size()
}

// TestCheckpoint dirties the WAL of llmd.db and, when the case opens
// it, work.db, then checks Checkpoint leaves both WALs empty or gone
// and the store still readable.
func TestCheckpoint(t *testing.T) {
	tests := []struct {
		name string
		work bool
	}{
		{"documents only", false},
		{"with work database", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, path := initStore(t)
			ctx := t.Context()
			workPath := filepath.Join(filepath.Dir(path), "work.db")

			if _, err := s.Documents.Write(ctx, "docs/a", "hello", documents.WriteOptions{Author: "test", Source: "cli"}); err != nil {
				t.Fatalf("Write(docs/a) error = %v", err)
			}
			if n := walSize(t, path+"-wal"); n == 0 {
				t.Fatalf("llmd.db-wal size before Checkpoint() = 0, want > 0 (nothing to flush)")
			}
			if tt.work {
				if _, err := s.Messages.Send(ctx, messages.SendOptions{Type: "direct", Author: "test", Source: "cli"}); err != nil {
					t.Fatalf("Messages.Send() error = %v", err)
				}
				if n := walSize(t, workPath+"-wal"); n == 0 {
					t.Fatalf("work.db-wal size before Checkpoint() = 0, want > 0 (nothing to flush)")
				}
			}

			if err := s.Checkpoint(); err != nil {
				t.Fatalf("Checkpoint() error = %v", err)
			}

			if n := walSize(t, path+"-wal"); n != 0 {
				t.Errorf("llmd.db-wal size after Checkpoint() = %d, want 0", n)
			}
			if n := walSize(t, workPath+"-wal"); n != 0 {
				t.Errorf("work.db-wal size after Checkpoint() = %d, want 0", n)
			}
			if _, err := os.Stat(workPath); !tt.work && err == nil {
				t.Errorf("work.db exists after Checkpoint() on a documents-only store")
			}

			doc, err := s.Documents.Read(ctx, "docs/a")
			if err != nil {
				t.Fatalf("Read(docs/a) after Checkpoint() error = %v", err)
			}
			if doc.Content != "hello" {
				t.Errorf("Read(docs/a) after Checkpoint() Content = %q, want %q", doc.Content, "hello")
			}
		})
	}
}

// recordingHandler captures the event types delivered on the bus.
type recordingHandler struct {
	types []string
}

func (h *recordingHandler) HandleEvent(_ context.Context, e pkgevents.Event) error {
	h.types = append(h.types, e.Type)
	return nil
}

// TestAccessors checks DB and Bus hand out the live llmd.db manager
// and the bus the domains publish on: a query through DB sees a
// document written through the store, and a handler subscribed
// through Bus receives its event.
func TestAccessors(t *testing.T) {
	s, _ := initStore(t)
	ctx := t.Context()

	if s.DB() != s.db {
		t.Errorf("DB() = %p, want %p", s.DB(), s.db)
	}
	if s.Bus() != s.bus {
		t.Errorf("Bus() = %p, want %p", s.Bus(), s.bus)
	}

	h := &recordingHandler{}
	s.Bus().Subscribe(h)

	if _, err := s.Documents.Write(ctx, "docs/a", "hello", documents.WriteOptions{Author: "test", Source: "cli"}); err != nil {
		t.Fatalf("Write(docs/a) error = %v", err)
	}

	want := []string{pkgevents.DocumentWritten}
	if !slices.Equal(h.types, want) {
		t.Errorf("Bus() handler received %v, want %v", h.types, want)
	}

	row, err := s.DB().Query(`SELECT COUNT(*) FROM content WHERE path = ?`, "docs/a").WithContext(ctx).ReadRow()
	if err != nil {
		t.Fatalf("DB().Query() error = %v", err)
	}
	var n int
	if err := row.Scan(&n); err != nil {
		t.Fatalf("DB().Query() Scan error = %v", err)
	}
	if n != 1 {
		t.Errorf("DB() content rows for docs/a = %d, want 1", n)
	}
}

// TestWorkDBPath checks which work database each store keeps.
func TestWorkDBPath(t *testing.T) {
	dir := filepath.Join("games", ".llmd")
	tests := []struct{ store, want string }{
		{"llmd.db", "work.db"},
		{"llmd-adrift.db", "work-adrift.db"},
		{"notes.db", "notes-work.db"},
	}
	for _, tt := range tests {
		if got := workDBPath(filepath.Join(dir, tt.store)); got != filepath.Join(dir, tt.want) {
			t.Errorf("workDBPath(%s) = %s, want %s", tt.store, got, tt.want)
		}
	}
}

// TestStoresKeepTheirOwnWork checks that two stores in one folder keep
// separate boards: a task added to one is not on the other. They once
// shared work.db, so every named store showed every other store's tasks.
func TestStoresKeepTheirOwnWork(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".llmd")
	open := func(name string) *Store {
		s, err := Init(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("Init(%s) error = %v", name, err)
		}
		t.Cleanup(func() {
			if err := s.Close(); err != nil {
				t.Errorf("closing %s: %v", name, err)
			}
		})
		return s
	}
	main, adrift := open("llmd.db"), open("llmd-adrift.db")

	if _, err := adrift.Tasks.Add(t.Context(), "Grow the raft", nil, tasks.AddOptions{Author: "test", Source: "cli"}); err != nil {
		t.Fatalf("Tasks.Add error = %v", err)
	}
	for _, tt := range []struct {
		name  string
		store *Store
		want  int
	}{{"llmd.db", main, 0}, {"llmd-adrift.db", adrift, 1}} {
		got, err := tt.store.Tasks.List(t.Context(), tasks.ListOptions{})
		if err != nil {
			t.Fatalf("%s Tasks.List error = %v", tt.name, err)
		}
		if len(got) != tt.want {
			t.Errorf("%s has %d tasks, want %d", tt.name, len(got), tt.want)
		}
	}
}
