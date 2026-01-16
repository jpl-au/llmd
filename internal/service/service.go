// Package service defines the shared interface for document operations.
// Commands and extensions depend on this interface rather than concrete
// implementations, enabling testing with mocks and future backend changes.
package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/jpl-au/llmd/internal/diff"
	"github.com/jpl-au/llmd/internal/edit"
	"github.com/jpl-au/llmd/internal/store"
)

// Service defines all document operations.
//
// Extensions should use document.New() to obtain a Service implementation.
// Always call Close() when done (use defer).
//
// Example:
//
//	svc, err := document.New()
//	if err != nil {
//	    return err
//	}
//	defer svc.Close()
//	doc, err := svc.Latest(ctx, "docs/readme", false)
type Service interface {
	// Close releases database resources. Always defer this after New().
	Close() error

	// Latest returns the most recent version of a document.
	// If includeDeleted is false, returns store.ErrNotFound for deleted docs.
	Latest(ctx context.Context, path string, includeDeleted bool) (*store.Document, error)

	// Version returns a specific version of a document.
	// Returns store.ErrNotFound if the version doesn't exist.
	Version(ctx context.Context, path string, version int) (*store.Document, error)

	// ByKey retrieves a document by its unique 8-char key.
	// Returns store.ErrNotFound if no document exists with that key.
	ByKey(ctx context.Context, key string) (*store.Document, error)

	// Resolve returns a document by path or key. Designed for user-facing entry
	// points where input could be either identifier type.
	//
	// For 8-character inputs, it checks both path and key concurrently since
	// SQLite WAL mode supports parallel reads. Non-8-character inputs are treated
	// as paths only since keys are always exactly 8 characters.
	//
	// Semantic difference from Latest: if input resolves as a key, you get that
	// specific version, which may not be the latest. If it resolves as a path,
	// you get the latest version. This matches user intent. Passing a key means
	// "I want this specific version". Passing a path means "I want the current
	// content".
	//
	// Use Resolve for user or LLM input that could be path or key. This includes
	// commands like cat, history, tag, and MCP tools. Use Latest for internal
	// code operating on known paths, such as iteration or post-resolution work.
	//
	// Some commands need different behaviour based on input type. For example,
	// rm deletes a specific version when given a key but soft-deletes the entire
	// document when given a path. These commands should implement custom resolution
	// logic rather than using Resolve.
	Resolve(ctx context.Context, pathOrKey string, includeDeleted bool) (*store.Document, error)

	// List returns documents matching a path prefix.
	// Use "" for all documents. Set deletedOnly to list only deleted docs.
	List(ctx context.Context, prefix string, includeDeleted, deletedOnly bool) ([]store.Document, error)

	// ListByTag returns documents matching a path prefix and having the specified tag.
	// More efficient than List followed by filtering when tag filtering is needed.
	ListByTag(ctx context.Context, prefix, tag string, includeDeleted, deletedOnly bool, opts store.TagOptions) ([]store.Document, error)

	// Write creates a new version of a document.
	// If the document doesn't exist, creates it at version 1.
	// If sync_files is enabled, also writes to .llmd/files/<path>.
	Write(ctx context.Context, path, content, author, message string) error

	// Delete soft-deletes a document (can be restored).
	// Returns store.ErrNotFound if the document doesn't exist.
	Delete(ctx context.Context, path string) error

	// DeleteVersion soft-deletes a specific version of a document.
	// Other versions remain accessible. Returns store.ErrNotFound if the version doesn't exist.
	DeleteVersion(ctx context.Context, path string, version int) error

	// Restore un-deletes a soft-deleted document.
	// Returns store.ErrNotFound if the document doesn't exist or isn't deleted.
	Restore(ctx context.Context, path string) error

	// Move renames a document from one path to another.
	// Returns store.ErrAlreadyExists if destination exists.
	Move(ctx context.Context, from, to string) error

	// Search performs full-text search across document content using FTS5.
	// Query supports standard FTS5 syntax: "word1 word2" (AND), "word1 OR word2",
	// "word*" (prefix), "\"exact phrase\"". Use prefix to limit to a path prefix.
	Search(ctx context.Context, query, prefix string, includeDeleted, deletedOnly bool) ([]store.Document, error)

	// History returns version history for a document, newest first.
	// Set limit to 0 for all versions.
	History(ctx context.Context, path string, limit int, includeDeleted bool) ([]store.Document, error)

	// Glob returns document paths matching a glob pattern.
	// Supports *, **, and ? wildcards.
	Glob(ctx context.Context, pattern string) ([]string, error)

	// Edit performs a search/replace edit on a document.
	// Creates a new version with the replacement applied.
	Edit(ctx context.Context, path string, opts edit.Options) error

	// EditLineRange replaces a range of lines in a document.
	// Line numbers are 1-indexed. Creates a new version.
	EditLineRange(ctx context.Context, path string, opts edit.LineRangeOptions, replacement string) error

	// Diff compares document versions or against filesystem.
	// See diff.Options for comparison modes.
	Diff(ctx context.Context, path string, opts diff.Options) (diff.Result, error)

	// Vacuum permanently deletes soft-deleted documents.
	// If olderThan is set, only deletes docs deleted before that duration.
	// Returns the count of documents permanently removed.
	Vacuum(ctx context.Context, olderThan *time.Duration, prefix string) (int64, error)

	// Tag adds a tag to a document.
	// Tags are case-sensitive strings. Duplicate tags are ignored.
	Tag(ctx context.Context, path, tag string, opts store.TagOptions) error

	// Untag removes a tag from a document.
	// Returns no error if the tag didn't exist.
	Untag(ctx context.Context, path, tag string, opts store.TagOptions) error

	// ListTags returns all tags for a document.
	// If path is empty, returns all tags in the store.
	ListTags(ctx context.Context, path string, opts store.TagOptions) ([]string, error)

	// PathsWithTag returns all document paths having a specific tag.
	PathsWithTag(ctx context.Context, tag string, opts store.TagOptions) ([]string, error)

	// FilesDir returns the path to the .llmd directory.
	// Used for filesystem sync operations.
	FilesDir() string

	// Exists checks if a document exists without fetching content.
	// More efficient than Latest() when you only need to check existence.
	Exists(ctx context.Context, path string) (bool, error)

	// Copy duplicates a document to a new path.
	// The copy starts at version 1 with message "Copied from <source>".
	// The copier parameter tracks who performed the copy (for audit purposes).
	// Returns store.ErrAlreadyExists if destination exists.
	Copy(ctx context.Context, from, to, copier string) error

	// Count returns the number of documents matching a path prefix.
	// Use "" to count all documents.
	Count(ctx context.Context, prefix string) (int64, error)

	// Meta returns document metadata without content.
	// More efficient than Latest() when content isn't needed.
	Meta(ctx context.Context, path string) (*store.DocumentMeta, error)

	// DB returns the underlying SQLite connection.
	// Extensions use this to create custom tables.
	// Do not close this connection directly; use Service.Close().
	DB() *sql.DB

	// Tx runs a function within a database transaction.
	// If fn returns nil, the transaction is committed.
	// If fn returns an error, the transaction is rolled back.
	//
	// Example:
	//
	//	err := svc.Tx(ctx, func(tx *sql.Tx) error {
	//	    _, err := tx.Exec("INSERT INTO tasks (title) VALUES (?)", "Task 1")
	//	    if err != nil {
	//	        return err // triggers rollback
	//	    }
	//	    _, err = tx.Exec("INSERT INTO tasks (title) VALUES (?)", "Task 2")
	//	    return err // nil commits, error rolls back
	//	})
	Tx(ctx context.Context, fn func(tx *sql.Tx) error) error

	// Link creates a bidirectional link between two documents.
	// Returns the unique link ID. Tag is optional (empty for untagged links).
	Link(ctx context.Context, from, to, tag string, opts store.LinkOptions) (string, error)

	// UnlinkByID removes a link by its unique ID (soft delete).
	UnlinkByID(ctx context.Context, id string) error

	// UnlinkByTag removes all links with a specific tag (soft delete).
	// Returns the count of links removed.
	UnlinkByTag(ctx context.Context, tag string, opts store.LinkOptions) (int64, error)

	// ListLinks returns all links for a document.
	// If tag is non-empty, filters to links with that tag.
	ListLinks(ctx context.Context, path, tag string, opts store.LinkOptions) ([]store.Link, error)

	// ListLinksByTag returns all links with a specific tag.
	ListLinksByTag(ctx context.Context, tag string, opts store.LinkOptions) ([]store.Link, error)

	// ListOrphanLinkPaths returns document paths with no links.
	ListOrphanLinkPaths(ctx context.Context, opts store.LinkOptions) ([]string, error)

	// ListPaths returns document paths without loading content, enabling
	// efficient path enumeration for glob matching and directory listings.
	ListPaths(ctx context.Context, prefix string) ([]string, error)

	// ListDeletedPaths returns paths of soft-deleted documents, enabling
	// trash management and vacuum preview without loading document content.
	ListDeletedPaths(ctx context.Context, prefix string) ([]string, error)

	// ListMeta returns metadata for multiple documents matching a prefix,
	// enabling efficient batch queries for listings that need size/version
	// info without loading full document content.
	ListMeta(ctx context.Context, prefix string, includeDeleted bool) ([]store.DocumentMeta, error)

	// CountDeleted returns the count of soft-deleted documents, enabling
	// vacuum preview and trash management without loading document data.
	CountDeleted(ctx context.Context, prefix string) (int64, error)

	// DeletedBefore returns paths of documents deleted before the given time,
	// enabling targeted vacuum operations that preserve recently deleted items.
	DeletedBefore(ctx context.Context, t time.Time, prefix string) ([]string, error)

	// VersionCount returns the number of versions for a document without
	// loading full history, enabling version management and display.
	VersionCount(ctx context.Context, path string) (int, error)

	// ListAuthors returns all distinct authors who have written documents,
	// enabling author-based filtering and audit reporting.
	ListAuthors(ctx context.Context) ([]string, error)

	// Stats returns aggregate database statistics for capacity planning
	// and operational visibility.
	Stats(ctx context.Context) (*store.Stats, error)

	// DeleteLinksForPath soft-deletes all links involving a document,
	// enabling cleanup when documents are removed or reorganised.
	DeleteLinksForPath(ctx context.Context, path string, opts store.LinkOptions) error

	// Checkpoint flushes the WAL to the main database file, removing
	// the -wal and -shm files. Useful before backup or distribution.
	Checkpoint(ctx context.Context) error
}
