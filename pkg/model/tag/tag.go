// Package tag provides the Tag model for document tagging.
//
// Tags provide a way to categorize and organize documents with lightweight labels.
// Unlike links which connect documents to each other, tags are independent labels
// that can be attached to any document. Multiple tags can be attached to the same
// document, and the same tag can be applied across many documents.
//
// # Storage
//
// Tags are stored in the entities table with namespace "core:tag". Each tag is
// an entity where:
//   - Relation: the document path being tagged
//   - Value: JSON payload containing the tag name (e.g., {"tag": "important"})
//
// This design leverages the flexible entities system while providing type-safe
// access through the Tag and Value structs.
//
// # Tag Names
//
// Tag names must be lowercase alphanumeric with hyphens allowed (but not at
// start or end). Maximum length is 64 characters. Examples: "important",
// "needs-review", "v1", "draft".
//
// # Soft Deletion
//
// When a tag is removed, it is soft-deleted (deleted_at is set). The tag
// can be purged later to permanently remove it. This allows for audit trails
// and potential recovery.
//
// # Usage
//
// Tags are accessed through the store's Tags service:
//
//	store.Tags.Add(ctx, "docs/readme", "important", opts)
//	store.Tags.List(ctx, "docs/readme")
//	store.Tags.Find(ctx, "important")  // find all docs with this tag
//	store.Tags.Remove(ctx, "docs/readme", "important", opts)
package tag

import "github.com/jpl-au/llmd/pkg/model/core"

// Value is the JSON payload stored in entity.value for tags.
//
// This struct is marshalled to JSON when storing a tag and unmarshalled
// when reading. It contains only the tag name, keeping the payload minimal.
//
// Example JSON: {"tag": "important"}
type Value struct {
	// Tag is the tag name. Must be lowercase alphanumeric with hyphens,
	// 1-64 characters, not starting or ending with hyphen.
	Tag string `json:"tag"`
}

// Tag represents a tag attached to a document.
//
// A Tag combines entity metadata (Key, Provenance) with tag-specific data
// (Relation for the document path, Value for the tag name).
// Tags are immutable once created - to change a tag, remove and re-add it.
type Tag struct {
	// Key is the unique identifier for this tag entity (9-character nanoid).
	Key string

	// Relation is the document path this tag is attached to.
	// This corresponds to the entity's relation field.
	Relation string

	// Value contains the tag name, decoded from the entity's JSON value.
	Value Value

	// Provenance tracks who created this tag, from where, and when.
	core.Provenance
}

// Info represents tag metadata with usage count.
//
// Info is returned by listing all tags and provides aggregate information
// about tag usage across the store, useful for tag clouds, filters, or
// organizational views.
type Info struct {
	// Name is the tag name.
	Name string

	// Count is the number of documents currently tagged with this tag.
	// Only counts active tags (not soft-deleted).
	Count int
}
