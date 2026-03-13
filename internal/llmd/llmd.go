// Package llmd provides the core document storage engine.
//
// Store is the top-level type that orchestrates sub-packages:
//
//   - documents: CRUD operations, versioning, soft-delete
//   - history: version listing, diffs, reverts
//   - search: FTS5 full-text search and path glob matching
//   - bulk: batch import/export operations
//   - tags: key-value metadata on documents
//   - links: relationships between documents
//   - entities: named entity extraction and storage
//   - events: in-process event bus for cross-cutting concerns
//
// The event bus connects packages that need to react to changes without
// direct dependencies. Currently, the FTS search index subscribes to
// document write/delete events to keep the full-text index in sync.
//
// Storage uses SQLite with WAL mode for concurrent read access. Schema
// migration runs automatically on Open/Init via the internal/sql package.
package llmd

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jpl-au/llmd/internal/llmd/audit"
	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/internal/llmd/search"
	"github.com/jpl-au/llmd/internal/llmd/tags"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	docpath "github.com/jpl-au/llmd/internal/path"
	_ "modernc.org/sqlite"
)

// Store is the top-level handle for all document operations. Public
// fields expose the sub-package managers; private fields hold the
// underlying database connection and event bus.
type Store struct {
	Documents *documents.Documents
	History   *history.History
	Search    *search.Search
	Bulk      *bulk.Bulk
	Tags      *tags.Tags
	Links     *links.Links
	Entities  *entities.Entities
	Tasks     *tasks.Tasks
	Audit     *audit.Log

	db   *sql.DB
	bus  *events.Bus
	path string
}

// DefaultPath returns the default store path.
func DefaultPath() string {
	return filepath.Join(".llmd", "llmd.db")
}

// Init creates a new store. Fails if it already exists.
func Init(path string) (*Store, error) {
	var err error
	path, err = docpath.ResolveDB(path)
	if err != nil {
		return nil, fmt.Errorf("resolving database path: %w", err)
	}

	// Check if already exists
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("store already exists: %s", path)
	}

	// Create directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory: %w", err)
	}

	return open(path)
}

// Open opens an existing store. Fails if it doesn't exist.
func Open(path string) (*Store, error) {
	var err error
	path, err = docpath.ResolveDB(path)
	if err != nil {
		return nil, fmt.Errorf("resolving database path: %w", err)
	}

	// Check exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("store not found: %s", path)
	}

	return open(path)
}

// open is the shared implementation for Init and Open. It configures
// SQLite pragmas, runs schema migration, wires up the event bus, and
// initialises all sub-package managers.
//
// Pragmas:
//   - journal_mode(WAL): allows concurrent readers during writes
//   - busy_timeout(5000): waits up to 5s instead of failing on lock contention
//   - foreign_keys(1): enforces referential integrity
func open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	s := &Store{
		db:   db,
		path: path,
	}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}

	s.wire()
	return s, nil
}

// OpenMemory opens an in-memory store for testing.
func OpenMemory() (*Store, error) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	s := &Store{
		db:   db,
		path: ":memory:",
	}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}

	s.wire()
	return s, nil
}

// wire initialises the event bus and all sub-package managers. Called
// once after migration by both open() and OpenMemory().
func (s *Store) wire() {
	s.bus = events.New()

	ftsHandler := search.NewFTSHandler(s.db)
	s.bus.Subscribe(ftsHandler)

	s.Documents = documents.New(s.db, s.bus)
	s.History = history.New(s.db, s.Documents)
	s.Search = search.New(s.db)
	s.Bulk = bulk.New(s.Documents)
	s.Tags = tags.New(s.db, s.Documents)
	s.Links = links.New(s.db, s.Documents)
	s.Entities = entities.New(s.db)
	s.Audit = audit.New(s.db)
	s.Tasks = tasks.New(s.db, s.Documents, s.Entities, s.Audit)
}

// Close closes the store. For on-disk stores, it first checkpoints the
// WAL to flush pending writes to the main database file, ensuring data
// durability if the process crashes after Close. The checkpoint is
// best-effort — a failure still proceeds with closing to avoid leaking
// the database connection.
func (s *Store) Close() error {
	if s.path != ":memory:" {
		if err := s.Checkpoint(); err != nil {
			slog.Warn("WAL checkpoint on close", "path", s.path, "error", err)
		}
	}
	return s.db.Close()
}

// Checkpoint flushes WAL data into the main database file and truncates
// the WAL to zero bytes. This leaves a self-contained .db file safe for
// git commits (the gitignored -wal and -shm files become empty/absent).
// The database stays in WAL mode for the next session.
func (s *Store) Checkpoint() error {
	_, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

// Path returns the path to the store database.
func (s *Store) Path() string {
	return s.path
}

// DB returns the underlying database connection. Used by the host to
// create extension contexts for event handlers and initialisable
// extensions that need custom tables.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Bus returns the event bus for subscribing to document events.
func (s *Store) Bus() *events.Bus {
	return s.bus
}
