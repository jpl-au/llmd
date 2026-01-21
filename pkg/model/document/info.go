package document

// Info contains document metadata without content, used for listings.
//
// Info is a lightweight representation of a document that excludes the
// potentially large Content field. Used by List operations to efficiently
// return many documents without loading all their content into memory.
type Info struct {
	// Key is the stable document identifier (9-character nanoid).
	Key string

	// Path is the human-readable document identifier.
	Path string

	// Version is the version number of this document.
	Version int

	// Author identifies who created this version.
	Author string

	// Message is the optional commit-style message for this version.
	Message string

	// Source identifies the origin of this version.
	Source string

	// MIME is the content type.
	MIME string

	// Meta contains computed metadata (size, line count).
	Meta *Meta

	// CreatedAt is the Unix timestamp when this version was created.
	CreatedAt int64

	// DeletedAt is the Unix timestamp when soft-deleted, or nil if active.
	DeletedAt *int64
}
