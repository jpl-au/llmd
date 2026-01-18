// Package document provides higher-level document operations backed by a
// Store implementation. It exposes a `Service` which wraps a `store.Store`
// and offers convenience methods for reading, writing and manipulating
// documents and their filesystem mirrors.
package document

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/config"
	"github.com/jpl-au/llmd/internal/log"
	norm "github.com/jpl-au/llmd/internal/path"
	"github.com/jpl-au/llmd/internal/repo"
	"github.com/jpl-au/llmd/internal/store"
)

const DefaultAuthor = "unknown"

// Service provides higher-level document operations backed by a Store.
type Service struct {
	store         *store.SQLiteStore
	dbPath        string
	filesDir      string
	syncFiles     bool
	maxPath       int
	maxContent    int64
	maxLineLength int
	extCtx        extension.Context // for firing events to extensions
}

// New creates a new Service, discovering the DB by walking up the directory tree.
// The db parameter specifies which database to use (empty for default).
// Returns ErrNotInitialised if no matching database is found.
func New(db string) (*Service, error) {
	dbPath, err := repo.Discover(db)
	if err != nil {
		return nil, err
	}

	s, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}

	filesDir := filepath.Dir(dbPath)
	cfg, err := config.Load()
	if err != nil {
		return nil, err // config.Load provides detailed, actionable error messages
	}

	return &Service{
		store:         s,
		dbPath:        dbPath,
		filesDir:      filesDir,
		syncFiles:     cfg.SyncFiles(),
		maxPath:       cfg.MaxPath(),
		maxContent:    cfg.MaxContent(),
		maxLineLength: cfg.MaxLineLength(),
	}, nil
}

// Init initialises a new llmd store.
// If dir is empty, uses current directory; otherwise uses dir.
// The db parameter specifies which database to create (empty for default).
// If local is true, the database is added to .gitignore (not committed).
//
// Note: Init does not write config. Config is managed separately via "llmd config".
func Init(force bool, db string, local bool, dir string) error {
	return repo.Init(force, db, local, dir)
}

// Close checkpoints the WAL and closes the database connection.
func (s *Service) Close() error {
	if err := s.store.Checkpoint(context.Background()); err != nil {
		log.Event("service:close", "checkpoint").
			Detail("error", err.Error()).
			Write(err)
	}
	return s.store.Close()
}

// ReloadConfig reloads configuration from disk and updates cached values.
// Call this after modifying config to ensure the service uses new settings.
func (s *Service) ReloadConfig() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	s.syncFiles = cfg.SyncFiles()
	s.maxPath = cfg.MaxPath()
	s.maxContent = cfg.MaxContent()
	s.maxLineLength = cfg.MaxLineLength()
	return nil
}

// SetExtensionContext sets the extension context for firing events.
// Called from cmd/root.go after creating the context.
func (s *Service) SetExtensionContext(ctx extension.Context) {
	s.extCtx = ctx
}

// normalizePath normalises a document path for consistent storage and lookup.
// This is the service-layer entry point; store layer independently validates
// paths for defence-in-depth (protects against direct store access).
func (s *Service) normalizePath(path string) (string, error) {
	return norm.Normalise(path)
}

// normalizePrefix normalises an optional prefix path. Empty prefixes are
// passed through unchanged to enable "list all" operations.
func (s *Service) normalizePrefix(prefix string) (string, error) {
	if prefix == "" {
		return "", nil
	}
	return norm.Normalise(prefix)
}

// fireEvent notifies all registered extension event handlers.
//
// Design: Event handler errors are logged but not propagated. This is intentional:
// events are notifications, not veto points. Extensions observe operations but
// cannot block them. If critical operations need extension approval, use a
// different mechanism (e.g., pre-operation hooks that can return errors).
//
// Thread-safe: extension.All() returns a snapshot copy under read lock,
// and extensions are only registered during init() (never removed).
func (s *Service) fireEvent(e extension.Event) {
	if s.extCtx == nil {
		return
	}
	for _, ext := range extension.All() {
		if h, ok := ext.(extension.EventHandler); ok {
			if err := h.HandleEvent(s.extCtx, e); err != nil {
				log.Event("event:error", "error").
					Detail("ext", ext.Name()).
					Detail("event", string(e.EventType())).
					Write(err)
			}
		}
	}
}

// DB returns the underlying database connection for extensions.
func (s *Service) DB() *sql.DB {
	return s.store.DB()
}

// MaxLineLength returns the configured maximum line length for scanning.
func (s *Service) MaxLineLength() int {
	return s.maxLineLength
}

// DBPath returns the path to the database file.
func (s *Service) DBPath() string {
	return s.dbPath
}

// FilesDir returns the path to the files directory.
func (s *Service) FilesDir() string {
	return s.filesDir
}

// Tx runs a function within a database transaction.
//
// The defer Rollback pattern: We always defer Rollback(), then call Commit()
// at the end. This is safe because Rollback() on a committed transaction is
// a no-op. The pattern guarantees cleanup in all cases:
// - fn() returns error → Rollback() runs, undoing partial changes
// - fn() panics → Rollback() runs via defer
// - Commit() fails → Rollback() runs (already committed portions are safe)
// - Commit() succeeds → Rollback() is a no-op
//
// Why expose raw *sql.Tx: Extensions may need complex operations not covered
// by the Service API. Raw transactions let them do multi-step atomic operations
// while still benefiting from the service's connection management.
func (s *Service) Tx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after commit

	if err := fn(tx); err != nil {
		return fmt.Errorf("transaction rolled back: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
