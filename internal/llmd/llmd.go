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
	"fmt"
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
	Audits    *audits.Audits
	Messages  *messages.Messages
	Agents    *agents.Agents
	Audit     *audit.Log

	db   *qwr.Manager
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
	rp := profile.ReadBalanced().WithForeignKeys(true)
	wp := profile.WriteBalanced().WithForeignKeys(true)

	db, err := qwr.New("file::memory:?cache=shared").
		Reader(rp).
		Writer(wp).
		Open()
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
	s.Tags = tags.New(s.db, s.Documents, s.bus)
	s.Links = links.New(s.db, s.Documents, s.bus)
	s.Entities = entities.New(s.db)
	s.Audit = audit.New(s.db)
	s.Tasks = tasks.New(s.db, s.Documents, s.Entities, s.Audit, s.bus)
	s.Audits = audits.New(s.db, s.bus)
	s.Messages = messages.New(s.db, s.bus)
	s.Agents = agents.New(s.db, filepath.Join(s.Dir(), "agents"), s.bus)

	// Bridge domain events into the message queue so consumers
	// can poll for changes across all domains.
	s.bus.Subscribe(messages.NewHandler(s.Messages))
}

// Close closes the store. qwr handles WAL checkpoint on close when
// configured with Checkpoint(checkpoint.Truncate) in open().
func (s *Store) Close() error {
	return s.db.Close()
}

// Checkpoint flushes WAL data into the main database file and truncates
// the WAL to zero bytes. This leaves a self-contained .db file safe for
// git commits (the gitignored -wal and -shm files become empty/absent).
// The database stays in WAL mode for the next session.
func (s *Store) Checkpoint() error {
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
