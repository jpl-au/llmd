// Package link provides the Link model for document relationships.
//
// Links create explicit, directional relationships between documents. Unlike tags
// which label individual documents, links connect two documents together,
// enabling graph-like navigation and relationship queries.
//
// # Directionality
//
// Links are directional: they go FROM a source document TO a target document.
// This is reflected in the model:
//   - Relation: the source document path (where the link originates)
//   - Value.To: the target document path (where the link points)
//
// When querying links, you can specify direction:
//   - Outgoing: links FROM the document (Relation = document)
//   - Incoming: links TO the document (Value.To = document)
//   - Both: all links involving the document
//
// # Labels
//
// Links can optionally have labels that describe the relationship type.
// For example: "requires", "implements", "related-to", "supersedes".
// Multiple links between the same documents are allowed if they have
// different labels. A link without a label represents an untyped relationship.
//
// # Storage
//
// Links are stored in the entities table with namespace "core:link". Each link is
// an entity where:
//   - Relation: the source document path
//   - Value: JSON payload with target path and optional label
//
// Example JSON: {"to": "docs/auth", "label": "requires"}
//
// # Self-Links
//
// Self-links (document linking to itself) are not allowed and will return
// ErrSelfLink when attempted.
//
// # Soft Deletion
//
// When a link is removed, it is soft-deleted (deleted_at is set). The link
// can be purged later to permanently remove it. When removing links without
// specifying a label, ALL links between the two documents are removed.
//
// # Usage
//
// Links are accessed through the store's Links service:
//
//	store.Links.Add(ctx, "docs/api", "docs/auth", opts)      // create link
//	store.Links.List(ctx, "docs/api", opts)                   // list links
//	store.Links.Exists(ctx, "docs/api", "docs/auth", opts)   // check existence
//	store.Links.Remove(ctx, "docs/api", "docs/auth", opts)   // remove link
package link

import "github.com/jpl-au/llmd/pkg/model/core"

// Value is the JSON payload stored in entity.value for links.
//
// This struct is marshalled to JSON when storing a link and unmarshalled
// when reading. It contains the target document path and an optional label
// describing the relationship type.
//
// Example JSON without label: {"to": "docs/models"}
// Example JSON with label: {"to": "docs/auth", "label": "requires"}
type Value struct {
	// To is the target document path that this link points to.
	To string `json:"to"`

	// Label is an optional relationship type descriptor.
	// Examples: "requires", "implements", "related-to", "supersedes".
	// When empty, represents an untyped/generic relationship.
	Label string `json:"label,omitempty"`
}

// Link represents a directional relationship between two documents.
//
// A Link combines entity metadata (Key, Origin) with link-specific data
// (Relation for source path, Value for target and label).
// Links are immutable once created - to change a link, remove and re-add it.
type Link struct {
	// Key is the unique identifier for this link entity (9-character nanoid).
	Key string

	// Relation is the source document path (FROM side of the link).
	// This corresponds to the entity's relation field.
	Relation string

	// Value contains the target path and optional label,
	// decoded from the entity's JSON value.
	Value Value

	// Origin tracks who created this link and from where.
	core.Origin

	// CreatedAt is the Unix timestamp (milliseconds) when created.
	CreatedAt int64
}
