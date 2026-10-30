// Package engine provides the core document storage engine.
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
// direct dependencies: the message queue, extensions and the server's
// SSE and webhook subscribers. The FTS index is not a subscriber; it is
// updated inside each document transaction (see package fts).
//
// Storage uses SQLite with WAL mode for concurrent read access. Schema
// schema is created on Open/Init via the sql package.
package engine

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jpl-au/llmd/errs"

	"github.com/jpl-au/llmd/sqlite"

	"github.com/jpl-au/qwr"
	"github.com/jpl-au/qwr/checkpoint"

	"github.com/jpl-au/llmd/engine/agents"
	"github.com/jpl-au/llmd/engine/audit"
	"github.com/jpl-au/llmd/engine/audits"
	"github.com/jpl-au/llmd/engine/bulk"
	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/entities"
	"github.com/jpl-au/llmd/engine/events"
	"github.com/jpl-au/llmd/engine/history"
	"github.com/jpl-au/llmd/engine/links"
	"github.com/jpl-au/llmd/engine/messages"
	"github.com/jpl-au/llmd/engine/search"
	"github.com/jpl-au/llmd/engine/settings"
	"github.com/jpl-au/llmd/engine/tags"
	"github.com/jpl-au/llmd/engine/tasks"
	docpath "github.com/jpl-au/llmd/path"
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
	Settings *settings.Settings
	Audit    *audit.Log

	db       *qwr.Manager // llmd.db - always open
	workMu   sync.Mutex   // guards work: the server opens it from concurrent requests
	work     *qwr.Manager // work.db - nil until first work operation
	bus      *events.Bus
	path     string // llmd.db path
	workPath string // work.db path

	queueRetention time.Duration   // Vacuum removes queue messages older than this; 0 keeps them
	dbSettings     sqlite.Settings // how llmd.db and work.db are opened
}

// Option configures how a store is opened.
type Option func(*Store)

// WithDatabase sets the SQLite settings for llmd.db and work.db. The
// zero value keeps the defaults.
func WithDatabase(s sqlite.Settings) Option {
	return func(st *Store) { st.dbSettings = s }
}

// SetQueueRetention sets how long queue messages are kept: Vacuum
// removes messages older than d. Zero keeps every message.
func (s *Store) SetQueueRetention(d time.Duration) {
	s.queueRetention = d
}

// Errors from Init and Open.
var (
	ErrStoreExists = errors.New("a store already exists.")
	ErrNoStore     = errors.New("there is no store.")
)

// Init creates a new store. Fails with ErrStoreExists if it already
// exists.
func Init(path string, opts ...Option) (*Store, error) {
	var err error
	path, err = docpath.ResolveDB(path)
	if err != nil {
		return nil, fmt.Errorf("resolving database path: %w", err)
	}

	// Check if already exists
	if _, err := os.Stat(path); err == nil {
		return nil, errs.New(ErrStoreExists, "a store already exists at %s.", path)
	}

	// Create directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory: %w", err)
	}

	return open(path, opts)
}

// Open opens an existing store. Fails if it doesn't exist.
func Open(path string, opts ...Option) (*Store, error) {
	var err error
	path, err = docpath.ResolveDB(path)
	if err != nil {
		return nil, fmt.Errorf("resolving database path: %w", err)
	}

	// Check exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, errs.New(ErrNoStore, "there is no store at %s. Run llmd init to create one.", path)
	}

	return open(path, opts)
}

// open is the shared implementation for Init and Open. Database
// settings live in sqlite so llmd.db and work.db are opened
// identically.
func open(path string, opts []Option) (*Store, error) {
	s := &Store{
		path:     path,
		workPath: filepath.Join(filepath.Dir(path), "work.db"),
	}
	for _, o := range opts {
		o(s)
	}

	db, err := sqlite.Open(path, s.dbSettings)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	s.db = db

	if err := s.createSchema(); err != nil {
		return nil, errors.Join(fmt.Errorf("creating schema: %w", err), db.Close())
	}

	s.wire()
	return s, nil
}

// wire initialises the event bus and all sub-package managers. Called
// once after the schema is created by open().
func (s *Store) wire() {
	s.bus = events.New()

	// Document-side domains (llmd.db).
	s.Documents = documents.New(s.db, s.bus)
	s.History = history.New(s.db, s.Documents)
	s.Search = search.New(s.db)
	s.Tags = tags.New(s.db, s.Documents, s.bus)
	s.Links = links.New(s.db, s.Documents, s.bus)
	s.Bulk = bulk.New(s.Documents, s.Tags, s.Links)
	s.Entities = entities.New(s.db)

	// Work-side domains (work.db, opened on demand).
	s.Audit = audit.New(s.WorkDB)
	s.Tasks = tasks.New(s.WorkDB, s.Documents, s.Audit, s.bus)
	s.Audits = audits.New(s.WorkDB, s.bus)
	s.Messages = messages.New(s.WorkDB, s.bus)
	s.Agents = agents.New(s.WorkDB, filepath.Join(s.Dir(), "agents"), s.bus)
	s.Settings = settings.New(s.WorkDB, s.bus)

	// Bridge domain events into the message queue so consumers
	// can poll for changes across all domains. The handler only
	// queues when work.db is open - document-only stores skip
	// queue writes silently.
	s.bus.Subscribe(messages.NewHandler(s.Messages, s.workOpen))
}

// WorkDB returns the work database manager, opening work.db and
// creating its schema on demand if it does not already exist. Returns nil if
// the database cannot be opened (error is logged); the next call tries
// again. Safe for concurrent use.
func (s *Store) WorkDB() *qwr.Manager {
	s.workMu.Lock()
	defer s.workMu.Unlock()
	if s.work != nil {
		return s.work
	}

	db, err := sqlite.Open(s.workPath, s.dbSettings)
	if err != nil {
		slog.Error("opening work database", "path", s.workPath, "err", err)
		return nil
	}

	if err := createWorkSchema(db); err != nil {
		slog.Error("creating work schema", "err", err)
		if cerr := db.Close(); cerr != nil {
			slog.Warn("closing work database after failed schema creation", "err", cerr)
		}
		return nil
	}
	s.work = db
	return s.work
}

// openWork returns the work database if this process has opened it,
// or nil. Unlike WorkDB it never opens the file.
func (s *Store) openWork() *qwr.Manager {
	s.workMu.Lock()
	defer s.workMu.Unlock()
	return s.work
}

// workOpen reports whether work.db exists. Used by the message queue
// handler to skip writes when no work database has ever been created.
// Checks the file on disk rather than the in-memory connection so
// that events are queued when work.db was created by a previous process.
func (s *Store) workOpen() bool {
	if s.openWork() != nil {
		return true
	}
	_, err := os.Stat(s.workPath)
	return err == nil
}

// createWorkSchema creates all work tables in a single pass. Called
// once when work.db is first opened. All statements are idempotent.
func createWorkSchema(db *qwr.Manager) error {
	schemas := []string{
		audit.Schema,
		tasks.Schema,
		audits.Schema,
		messages.Schema,
		agents.Schema,
		settings.Schema,
	}
	for _, ddl := range schemas {
		if _, err := db.Query(ddl).Exec(); err != nil {
			return fmt.Errorf("creating work schema: %w", err)
		}
	}
	return nil
}

// Close closes both databases. Safe to call when work.db was never opened.
func (s *Store) Close() error {
	var errs []error
	if work := s.openWork(); work != nil {
		errs = append(errs, work.Close())
	}
	errs = append(errs, s.db.Close())
	return errors.Join(errs...)
}

// Checkpoint flushes WAL data into the database files and truncates
// the WAL to zero bytes. This leaves self-contained .db files safe for
// git commits (the gitignored -wal and -shm files become empty/absent).
// Both llmd.db and work.db (if open) are checkpointed.
func (s *Store) Checkpoint() error {
	if work := s.openWork(); work != nil {
		if err := work.Checkpoint(checkpoint.Truncate).Exec(); err != nil {
			return fmt.Errorf("checkpointing work database: %w", err)
		}
	}
	return s.db.Checkpoint(checkpoint.Truncate).Exec()
}

// Path returns the path to the store database.
func (s *Store) Path() string {
	return s.path
}

// Dir returns the .llmd/ directory that contains the store.
func (s *Store) Dir() string {
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
