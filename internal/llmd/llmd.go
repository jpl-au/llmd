// Package llmd provides the core business logic for document storage.
package llmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/entities"
	"github.com/jpl-au/llmd/internal/llmd/events"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/internal/llmd/search"
	"github.com/jpl-au/llmd/internal/llmd/tags"
	_ "modernc.org/sqlite"
)

// Store provides access to llmd operations.
type Store struct {
	Documents *documents.Documents
	History   *history.History
	Search    *search.Search
	Bulk      *bulk.Bulk
	Tags      *tags.Tags
	Links     *links.Links
	Entities  *entities.Entities

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
	if path == "" {
		path = DefaultPath()
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
	if path == "" {
		path = DefaultPath()
	}

	// Check exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("store not found: %s (run 'llmd init' first)", path)
	}

	return open(path)
}

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

	s.bus = events.New()

	ftsHandler := search.NewFTSHandler(db)
	s.bus.Subscribe(ftsHandler)

	s.Documents = documents.New(db, s.bus)
	s.History = history.New(db, s.Documents)
	s.Search = search.New(db)
	s.Bulk = bulk.New(s.Documents)
	s.Tags = tags.New(db, s.Documents)
	s.Links = links.New(db, s.Documents)
	s.Entities = entities.New(db)

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

	// Create event bus
	s.bus = events.New()

	// Create FTS handler and subscribe
	ftsHandler := search.NewFTSHandler(db)
	s.bus.Subscribe(ftsHandler)

	// Initialize sub-components
	s.Documents = documents.New(db, s.bus)
	s.History = history.New(db, s.Documents)
	s.Search = search.New(db)
	s.Bulk = bulk.New(s.Documents)
	s.Tags = tags.New(db, s.Documents)
	s.Links = links.New(db, s.Documents)
	s.Entities = entities.New(db)

	return s, nil
}

// Close closes the store.
// Checkpoints WAL before closing to flush data to the main database file.
func (s *Store) Close() error {
	if s.path != ":memory:" {
		_ = s.Checkpoint() // best effort, still close even if checkpoint fails
	}
	return s.db.Close()
}

// Checkpoint writes WAL data to the main database file and truncates the WAL.
// This removes the -wal and -shm files if no other connections exist.
func (s *Store) Checkpoint() error {
	_, err := s.db.Exec("PRAGMA journal_mode=DELETE")
	return err
}

// Path returns the path to the store database.
func (s *Store) Path() string {
	return s.path
}

// Bus returns the event bus for subscribing to document events.
func (s *Store) Bus() *events.Bus {
	return s.bus
}
