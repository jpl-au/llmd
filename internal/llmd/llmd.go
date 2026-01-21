// Package llmd provides the core business logic for document storage.
package llmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
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

	db   *sql.DB
	path string
}

// Open opens or creates a store at the given path.
// If path is empty, it looks for .llmd/llmd.db in the current directory.
func Open(path string) (*Store, error) {
	if path == "" {
		path = filepath.Join(".llmd", "llmd.db")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating store directory: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	s := &Store{
		db:   db,
		path: path,
	}

	// Initialize schema
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}

	// Initialize sub-components
	s.Documents = documents.New(db)
	s.History = history.New(db, s.Documents)
	s.Search = search.New(db)
	s.Bulk = bulk.New(s.Documents)
	s.Tags = tags.New(db, s.Documents)
	s.Links = links.New(db, s.Documents)

	return s, nil
}

// OpenMemory opens an in-memory store for testing.
func OpenMemory() (*Store, error) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&_foreign_keys=ON")
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

	// Initialize sub-components
	s.Documents = documents.New(db)
	s.History = history.New(db, s.Documents)
	s.Search = search.New(db)
	s.Bulk = bulk.New(s.Documents)
	s.Tags = tags.New(db, s.Documents)
	s.Links = links.New(db, s.Documents)

	return s, nil
}

// Close closes the store.
// Checkpoints WAL before closing to clean up -wal and -shm files.
func (s *Store) Close() error {
	if s.path != ":memory:" {
		_ = s.Checkpoint() // best effort, still close even if checkpoint fails
	}
	return s.db.Close()
}

// Checkpoint writes WAL data to the main database file and truncates the WAL.
// This removes the -wal and -shm files if no other connections exist.
func (s *Store) Checkpoint() error {
	_, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

// Path returns the path to the store database.
func (s *Store) Path() string {
	return s.path
}
