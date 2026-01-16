// interfaces.go defines the storage abstraction for document persistence.
//
// Separated from the SQLite implementation to enable testing and potential
// alternative backends. The interfaces are intentionally granular (Reader,
// Writer, Searcher, etc.) to support interface segregation - consumers only
// depend on the capabilities they need.
//
// Design: All mutating operations use soft-delete semantics. Documents are
// never immediately removed; they're marked deleted and can be recovered
// until Vacuum permanently purges them. This provides a safety net against
// accidental deletion and enables audit trails.

package store

import (
	"context"
	"database/sql"
	"time"
)

// Reader defines read-only operations for retrieving documents and metadata.
type Reader interface {
	// Latest retrieves the current version of a document. Use includeDeleted
	// to access soft-deleted documents for recovery operations.
	Latest(ctx context.Context, path string, includeDeleted bool) (*Document, error)

	// Version retrieves a specific historical version for audit or rollback.
	Version(ctx context.Context, path string, version int) (*Document, error)

	// ByKey retrieves a document by its unique 8-char key. Returns ErrNotFound
	// if no document exists with that key.
	ByKey(ctx context.Context, key string) (*Document, error)

	// List returns documents matching a path prefix. The deletedOnly flag
	// enables listing trash contents separately from active documents.
	List(ctx context.Context, prefix string, includeDeleted bool, deletedOnly bool) ([]Document, error)

	// ListPaths returns only paths without content, enabling efficient
	// glob matching without loading full documents into memory.
	ListPaths(ctx context.Context, prefix string) ([]string, error)

	// ListDeletedPaths returns paths of soft-deleted documents, enabling
	// trash listing and vacuum preview without loading document content.
	ListDeletedPaths(ctx context.Context, prefix string) ([]string, error)

	// ListMeta returns metadata for multiple documents matching a prefix,
	// enabling efficient batch queries for document listings, dashboards,
	// and admin tools that need size/version info without content.
	ListMeta(ctx context.Context, prefix string, includeDeleted bool) ([]DocumentMeta, error)

	// History returns version history for auditing changes over time.
	History(ctx context.Context, path string, limit int, includeDeleted bool) ([]Document, error)

	// Exists checks document presence without loading content, enabling
	// fast validation before operations that require the document to exist.
	Exists(ctx context.Context, path string) (bool, error)

	// Count returns document count for a prefix, useful for statistics
	// and pagination without loading full document data.
	Count(ctx context.Context, prefix string) (int64, error)

	// CountDeleted returns the count of soft-deleted documents, enabling
	// vacuum preview and trash management without loading document data.
	CountDeleted(ctx context.Context, prefix string) (int64, error)

	// Meta returns document metadata without content for efficient listings
	// where only size, version, and timestamps are needed.
	Meta(ctx context.Context, path string) (*DocumentMeta, error)

	// DeletedBefore returns paths of documents deleted before the given time,
	// enabling targeted vacuum operations and stale trash cleanup.
	DeletedBefore(ctx context.Context, t time.Time, prefix string) ([]string, error)

	// VersionCount returns the number of versions for a document without
	// loading full history, enabling version management decisions.
	VersionCount(ctx context.Context, path string) (int, error)

	// ListAuthors returns all distinct authors who have written documents,
	// enabling author-based filtering and audit reporting.
	ListAuthors(ctx context.Context) ([]string, error)

	// Stats returns aggregate database statistics for capacity planning
	// and operational dashboards.
	Stats(ctx context.Context) (*Stats, error)
}

// Writer defines operations that modify documents.
type Writer interface {
	// Write creates a new version, preserving the previous version for history.
	Write(ctx context.Context, path, content string, opts WriteOptions) error

	// Delete marks a document as deleted without removing data, allowing
	// recovery via Restore until Vacuum permanently removes it.
	Delete(ctx context.Context, path string, opts DeleteOptions) error

	// Restore recovers a soft-deleted document to active status.
	Restore(ctx context.Context, path string, opts RestoreOptions) error

	// Move renames a document, preserving all version history.
	Move(ctx context.Context, src, dst string, opts MoveOptions) error

	// Copy duplicates a document, creating version 1 at the destination
	// while preserving the source document unchanged. The copier parameter
	// tracks who performed the copy operation (distinct from the content author).
	Copy(ctx context.Context, from, to, copier string, opts CopyOptions) error
}

// Searcher defines search operations.
type Searcher interface {
	// Search performs full-text search across document paths and content.
	Search(ctx context.Context, query, prefix string, includeDeleted bool, deletedOnly bool) ([]Document, error)
}

// Tagger defines operations for managing tags on documents.
type Tagger interface {
	// Tag associates a label with a document for organisation and filtering.
	Tag(ctx context.Context, path, tag string, opts TagOptions) error

	// Untag removes a label from a document.
	Untag(ctx context.Context, path, tag string, opts TagOptions) error

	// ListTags returns tags for filtering. Pass empty path for all tags.
	ListTags(ctx context.Context, path string, opts TagOptions) ([]string, error)

	// PathsWithTag finds documents with a specific tag for batch operations.
	PathsWithTag(ctx context.Context, tag string, opts TagOptions) ([]string, error)

	// ListByTag returns documents matching a path prefix that have a specific tag.
	ListByTag(ctx context.Context, prefix, tag string, includeDeleted, deletedOnly bool, opts TagOptions) ([]Document, error)
}

// Linker defines operations for managing links between documents.
type Linker interface {
	// Link creates a relationship between documents. Returns the link ID
	// for later removal. Tags categorise relationship types.
	Link(ctx context.Context, from, to, tag string, opts LinkOptions) (string, error)

	// UnlinkByID soft-deletes a specific link for targeted removal.
	UnlinkByID(ctx context.Context, id string) error

	// UnlinkByTag soft-deletes all links with a tag for bulk cleanup.
	UnlinkByTag(ctx context.Context, tag string, opts LinkOptions) (int64, error)

	// ListLinks returns connections for a document to show relationships.
	ListLinks(ctx context.Context, path, tag string, opts LinkOptions) ([]Link, error)

	// ListLinksByTag returns all links with a tag for relationship analysis.
	ListLinksByTag(ctx context.Context, tag string, opts LinkOptions) ([]Link, error)

	// ListOrphanLinkPaths finds documents with no links, helping identify
	// disconnected content that may need attention.
	ListOrphanLinkPaths(ctx context.Context, opts LinkOptions) ([]string, error)

	// DeleteLinksForPath soft-deletes all links for a document when removed,
	// maintaining referential integrity.
	DeleteLinksForPath(ctx context.Context, path string, opts LinkOptions) error
}

// Maintainer defines operations for database maintenance and lifecycle.
type Maintainer interface {
	// Close releases the database connection.
	Close() error

	// DB exposes the underlying connection for extensions needing custom tables.
	DB() *sql.DB

	// Checkpoint flushes WAL to the main database file.
	Checkpoint(ctx context.Context) error

	// Vacuum permanently removes soft-deleted data.
	Vacuum(ctx context.Context, olderThan *time.Duration, path string) (int64, error)
}

// Store defines the persistence interface for documents. All operations are
// designed for soft-delete semantics, enabling recovery until vacuum.
type Store interface {
	Reader
	Writer
	Searcher
	Tagger
	Linker
	Maintainer
}
