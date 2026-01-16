// sqlite_ops.go provides SQLite connection management and low-level operations.
//
// Separated to isolate SQLite-specific concerns (pragmas, connection pooling,
// driver registration) from business logic. This is the only file that imports
// the SQLite driver, making it easier to swap implementations if needed.
//
// Design: WAL mode with busy timeout balances concurrency and durability.
// WAL allows concurrent readers during writes (critical for MCP scenarios).
// The 5-second busy timeout prevents "database is locked" errors without
// waiting forever on stuck connections.

package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	// Register sqlite driver
	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite with WAL mode for concurrent access.
// It provides versioned document storage with full-text search capabilities.
type SQLiteStore struct {
	db *sql.DB
}

// Compile-time interface compliance check. This ensures SQLiteStore implements
// the full Store interface. If a method is missing or has the wrong signature,
// the build fails immediately with a clear error, rather than failing at runtime
// when the method is called. This is especially valuable when interfaces change.
var _ Store = (*SQLiteStore)(nil)

// Open opens the SQLite database file at `path` and returns a configured
// SQLiteStore. The caller should call Close on the returned store.
//
// The pragma configuration balances durability, performance, and concurrency
// for llmd's usage pattern (frequent small writes, occasional bulk imports,
// read-heavy LLM workflows).
func Open(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}

	// WAL mode: Allows concurrent readers while writing. Without this, readers
	// block writers and vice versa. Critical for MCP server scenarios where
	// an LLM might read while the user writes. Trade-off: Creates -wal and
	// -shm files alongside the database.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	// Busy timeout: How long to wait when another connection holds a lock.
	// 5 seconds is generous - most operations complete in milliseconds. This
	// prevents "database is locked" errors during concurrent access without
	// waiting forever on a stuck connection.
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting busy timeout: %w", err)
	}

	// Synchronous NORMAL: With WAL mode, NORMAL is safe against corruption
	// (WAL provides the durability guarantee). FULL would fsync on every
	// commit, which is ~10x slower. The only risk with NORMAL is losing the
	// last transaction on OS crash - acceptable for a document store where
	// users can re-run the command.
	if _, err := db.Exec(`PRAGMA synchronous=NORMAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting synchronous mode: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Init creates tables and indexes if they don't exist. Safe to call multiple
// times; uses IF NOT EXISTS to avoid errors on existing databases.
func (s *SQLiteStore) Init() error {
	return execSchema(s.db)
}

// Close releases the database connection. Call before program exit to ensure
// all pending writes are flushed.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// DB exposes the underlying connection for extensions that need custom tables.
// Extensions should not modify core tables directly.
func (s *SQLiteStore) DB() *sql.DB {
	return s.db
}

// scanner abstracts sql.Row and sql.Rows, enabling a single scan function
// to handle both single-row and multi-row queries.
type scanner interface {
	Scan(dest ...any) error
}

// scanDoc extracts a Document from a database row, handling nullable fields.
func scanDoc(sc scanner) (Document, error) {
	var d Document
	var msg sql.NullString
	var del sql.NullInt64

	err := sc.Scan(&d.ID, &d.Key, &d.Path, &d.Content, &d.Version, &d.Author, &msg, &d.CreatedAt, &del)
	if err != nil {
		return d, err
	}

	if msg.Valid {
		d.Message = msg.String
	}
	if del.Valid {
		d.DeletedAt = &del.Int64
	}
	return d, nil
}

// scanDocument converts sql.ErrNoRows to ErrNotFound for consistent error handling.
func (s *SQLiteStore) scanDocument(row *sql.Row) (*Document, error) {
	d, err := scanDoc(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan document: %w", err)
	}
	return &d, nil
}

// scanDocuments iterates over query results, collecting documents into a slice.
func (s *SQLiteStore) scanDocuments(rows *sql.Rows) ([]Document, error) {
	var docs []Document
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// Tx executes fn within a database transaction, handling Begin/Commit/Rollback
// automatically. This eliminates a class of bugs where callers forget to commit,
// forget to rollback on error, or fail to check commit errors.
//
// The transaction lifecycle:
//  1. BeginTx is called to start the transaction with context
//  2. fn executes with the transaction
//  3. If fn returns an error, the transaction is rolled back
//  4. If fn succeeds, the transaction is committed
//  5. Rollback is deferred to handle panics and early returns
//
// Context cancellation will abort the transaction at the next database call.
//
// Callers focus on business logic; Tx handles the ceremony:
//
//	err := s.Tx(ctx, func(tx *sql.Tx) error {
//	    if _, err := tx.ExecContext(ctx, `UPDATE ...`); err != nil {
//	        return err  // triggers rollback
//	    }
//	    return nil  // triggers commit
//	})
//
// For functions that need to return values, use a closure variable:
//
//	var count int64
//	err := s.Tx(ctx, func(tx *sql.Tx) error {
//	    result, err := tx.ExecContext(ctx, `DELETE ...`)
//	    if err != nil {
//	        return err
//	    }
//	    count, _ = result.RowsAffected()
//	    return nil
//	})
//	return count, err
func (s *SQLiteStore) Tx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after commit

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// genID creates a unique 8-character identifier using crypto/rand for security.
// Used for document keys, tag IDs, and link IDs to enable direct lookups.
func genID() (string, error) {
	b := make([]byte, 5) // 5 bytes = 8 base32 chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return strings.ToLower(base32.StdEncoding.EncodeToString(b)), nil
}
