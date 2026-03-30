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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jpl-au/llmd/internal/llmd/agents"
	"github.com/jpl-au/llmd/internal/llmd/audit"
	"github.com/jpl-au/llmd/internal/llmd/audits"
	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/internal/llmd/messages"
	"github.com/jpl-au/llmd/internal/llmd/search"
	"github.com/jpl-au/llmd/internal/llmd/tags"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	docpath "github.com/jpl-au/llmd/internal/path"
	"github.com/jpl-au/qwr"
	"github.com/jpl-au/qwr/checkpoint"
	"github.com/jpl-au/qwr/profile"
)

// Store is the top-level handle for all document and work operations.
// Public fields expose the sub-package managers; private fields hold
// the underlying database connections and event bus.
//
// Two databases back the store: llmd.db holds documents, tags, links,
// and search indices (the publishable artifact). work.db holds tasks,
// audits, messages, and agent activity (operational state). work.db is
// created on demand when the first work operation runs, so pure
// document stores never touch it.
type Store struct {
	// Document-side domains (llmd.db - always open).
	Documents *documents.Documents
	History   *history.History
	Search    *search.Search
	Bulk      *bulk.Bulk
	Tags      *tags.Tags
	Links     *links.Links
	Entities  *entities.Entities

	// Work-side domains (work.db - opened on demand).
	Tasks    *tasks.Tasks
	Audits   *audits.Audits
	Messages *messages.Messages
	Agents   *agents.Agents
	Audit    *audit.Log

	db       *qwr.Manager // llmd.db - always open
	work     *qwr.Manager // work.db - nil until first work operation
	bus      *events.Bus
	path     string // llmd.db path
	workPath string // work.db path
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

// open is the shared implementation for Init and Open. qwr manages
// SQLite pragmas via profiles (WAL, busy_timeout, foreign_keys, etc.),
// separate reader/writer connection pools, and serialised writes.
func open(path string) (*Store, error) {
	// Foreign keys are not part of qwr's default profiles, so we
	// clone the balanced profiles and enable them explicitly.
	rp := profile.ReadBalanced().WithForeignKeys(true)
	wp := profile.WriteBalanced().WithForeignKeys(true)

	db, err := qwr.New(path).
		Reader(rp).
		Writer(wp).
		Checkpoint(checkpoint.Truncate).
		Open()
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	s := &Store{
		db:       db,
		path:     path,
		workPath: filepath.Join(filepath.Dir(path), "work.db"),
	}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}

	s.wire()
	return s, nil
}

// OpenMemory opens an in-memory store for testing. Both the document
// database and work database are created in memory and fully migrated
// so all domains are available immediately.
func OpenMemory() (*Store, error) {
	rp := profile.ReadBalanced().WithForeignKeys(true)
	wp := profile.WriteBalanced().WithForeignKeys(true)

	db, err := qwr.New("file::memory:?cache=shared").
		Reader(rp).
		Writer(wp).
		Open()
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	work, err := qwr.New("file:work?mode=memory&cache=shared").
		Reader(rp).
		Writer(wp).
		Open()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("opening work database: %w", err)
	}

	s := &Store{
		db:       db,
		work:     work,
		path:     ":memory:",
		workPath: ":memory:",
	}

	if err := s.migrate(); err != nil {
		work.Close()
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}
	if err := s.migrateWork(); err != nil {
		work.Close()
		db.Close()
		return nil, fmt.Errorf("migrating work schema: %w", err)
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

	// Document-side domains (llmd.db).
	s.Documents = documents.New(s.db, s.bus)
	s.History = history.New(s.db, s.Documents)
	s.Search = search.New(s.db)
	s.Bulk = bulk.New(s.Documents)
	s.Tags = tags.New(s.db, s.Documents, s.bus)
	s.Links = links.New(s.db, s.Documents, s.bus)
	s.Entities = entities.New(s.db)

	// Work-side domains (work.db, opened on demand).
	s.Audit = audit.New(s.WorkDB)
	s.Tasks = tasks.New(s.WorkDB, s.Documents, s.Entities, s.Audit, s.bus)
	s.Audits = audits.New(s.WorkDB, s.bus)
	s.Messages = messages.New(s.WorkDB, s.bus)
	s.Agents = agents.New(s.WorkDB, filepath.Join(s.Dir(), "agents"), s.bus)

	// Bridge domain events into the message queue so consumers
	// can poll for changes across all domains. The handler only
	// queues when work.db is open - document-only stores skip
	// queue writes silently.
	s.bus.Subscribe(messages.NewHandler(s.Messages, s.workOpen))
}

// WorkDB returns the work database manager, opening and migrating
// work.db on demand if it does not already exist. Returns nil if
// the database cannot be opened (error is logged).
func (s *Store) WorkDB() *qwr.Manager {
	if s.work != nil {
		return s.work
	}

	rp := profile.ReadBalanced().WithForeignKeys(true)
	wp := profile.WriteBalanced().WithForeignKeys(true)

	db, err := qwr.New(s.workPath).
		Reader(rp).
		Writer(wp).
		Checkpoint(checkpoint.Truncate).
		Open()
	if err != nil {
		slog.Error("opening work database", "path", s.workPath, "err", err)
		return nil
	}

	s.work = db
	if err := s.migrateWork(); err != nil {
		slog.Error("migrating work schema", "err", err)
		s.work.Close()
		s.work = nil
		return nil
	}
	return s.work
}

// workOpen reports whether work.db is currently open. Used by the
// message queue handler to skip writes when no work database exists.
func (s *Store) workOpen() bool {
	return s.work != nil
}

// migrateWork creates all work tables in a single pass. Called once
// when work.db is first opened. All statements are idempotent.
func (s *Store) migrateWork() error {
	schemas := []string{
		audit.Schema,
		tasks.Schema,
		audits.Schema,
		messages.Schema,
		agents.Schema,
	}
	for _, ddl := range schemas {
		if _, err := s.work.Query(ddl).Write(); err != nil {
			return fmt.Errorf("migrating work schema: %w", err)
		}
	}
	return nil
}

// Close closes both databases. Safe to call when work.db was never opened.
func (s *Store) Close() error {
	var errs []error
	if s.work != nil {
		errs = append(errs, s.work.Close())
	}
	errs = append(errs, s.db.Close())
	return errors.Join(errs...)
}

// Checkpoint flushes WAL data into the database files and truncates
// the WAL to zero bytes. This leaves self-contained .db files safe for
// git commits (the gitignored -wal and -shm files become empty/absent).
// Both llmd.db and work.db (if open) are checkpointed.
func (s *Store) Checkpoint() error {
	if s.work != nil {
		if err := s.work.RunCheckpoint(checkpoint.Truncate); err != nil {
			return fmt.Errorf("checkpointing work database: %w", err)
		}
	}
	return s.db.RunCheckpoint(checkpoint.Truncate)
}

// Path returns the path to the store database.
func (s *Store) Path() string {
	return s.path
}

// Dir returns the .llmd/ directory that contains the store. For
// in-memory stores used in tests, returns a temporary directory.
// All callers that need the .llmd/ path should use this instead of
// filepath.Dir(store.Path()) to avoid scattering the :memory:
// check everywhere.
func (s *Store) Dir() string {
	if s.path == ":memory:" {
		return filepath.Join(os.TempDir(), "llmd-memory-store")
	}
	return filepath.Dir(s.path)
}

// DB returns the qwr manager. Used by the host to create extension
// contexts for event handlers and initialisable extensions that need
// custom tables.
func (s *Store) DB() *qwr.Manager {
	return s.db
}

// Bus returns the event bus for subscribing to document events.
func (s *Store) Bus() *events.Bus {
	return s.bus
}
