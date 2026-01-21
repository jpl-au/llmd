// Package document provides the Document model for versioned content.
//
// Documents are the primary content storage in llmd. Each document is identified
// by a path (like a filesystem path) and supports full version history. Every
// write creates a new version; content is never modified in place.
//
// # Architecture
//
// llmd uses a two-table architecture:
//   - content: versioned document storage (this package's domain)
//   - entities: metadata, state, and relationships (tags, links, etc.)
//
// Documents are stored in the content table. Each version is a separate row,
// identified by both a stable Key (same across versions) and a unique ID
// (per version). The latest non-deleted version is the "current" document.
//
// # Paths and Keys
//
// Documents have both a Path and a Key:
//   - Path: human-readable identifier like "docs/readme" or "notes/2024/jan"
//   - Key: stable 9-character nanoid, same across all versions of a document
//
// Paths can be renamed; Keys never change. Use Keys for stable references,
// Paths for human interaction. Both can be used to look up documents.
//
// # Versioning
//
// Every write creates a new version. Versions are numbered starting from 1.
// You can retrieve any version by number, or get the latest. Deleted documents
// have DeletedAt set on their latest version; earlier versions remain accessible.
//
// # Content and MIME Types
//
// Content is stored as text. The MIME field indicates the content type
// (e.g., "text/markdown", "text/plain", "application/json"). This helps
// clients render content appropriately and enables format-aware operations.
//
// # Hash
//
// Each version has an XXH3 hash of its content. This enables efficient
// change detection: if the hash matches, the content is identical. Used
// internally for deduplication and by import/export operations.
//
// # Document vs Entity
//
// Documents store content with full version history. Entities (tags, links)
// store metadata about documents. Documents don't embed Provenance because
// they have an additional Message field (commit-style message) that entities
// don't have.
//
// # Usage
//
// Documents are accessed through the store's Documents service:
//
//	store.Documents.Write(ctx, "docs/readme", "# README\n...", opts)
//	store.Documents.Read(ctx, "docs/readme")
//	store.Documents.List(ctx, "docs/")
//	store.Documents.Delete(ctx, "docs/readme", opts)
//	store.Documents.History(ctx, "docs/readme")
package document

// Document represents a versioned content item stored in the content table.
//
// A Document combines content storage with comprehensive metadata including
// version history, authorship, and content analysis. Documents are immutable
// once written; modifications create new versions.
type Document struct {
	// ID is the database row ID for this specific version.
	// Each version has a unique ID. Not typically used in application code.
	ID int64

	// Key is the stable document identifier (9-character nanoid).
	// The same Key is used across all versions of a document.
	// Use this for stable references that survive path renames.
	Key string

	// Namespace scopes the document (default: "default").
	// Allows logical separation of documents within the same store.
	Namespace string

	// Path is the human-readable document identifier (e.g., "docs/readme").
	// Paths are hierarchical and can be renamed. Use Key for stable references.
	Path string

	// Content is the document's text content.
	// Binary content is not supported; use base64 encoding if needed.
	Content string

	// Version is the version number, starting from 1.
	// Increments with each write to the same path.
	Version int

	// Hash is the XXH3 hash of the content.
	// Used for change detection and deduplication.
	Hash string

	// Author identifies who created this version (e.g., username, "system").
	Author string

	// Message is an optional commit-style message describing this version.
	// Similar to git commit messages; useful for version history.
	Message string

	// Source identifies the origin of this version (e.g., "cli", "api", "import").
	Source string

	// MIME is the content type (e.g., "text/markdown", "text/plain").
	// Helps clients render content appropriately.
	MIME string

	// Meta contains computed metadata (size, line count).
	// May be nil if not computed.
	Meta *Meta

	// CreatedAt is the Unix timestamp (seconds) when this version was created.
	CreatedAt int64

	// DeletedAt is the Unix timestamp when soft-deleted, or nil if active.
	// Only the latest version can be deleted; earlier versions remain.
	DeletedAt *int64

	// Resolved indicates how this document was found during retrieval.
	// Helps callers understand the lookup path used.
	Resolved Resolved
}
