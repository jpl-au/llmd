// Package entity provides the Entity model for metadata and state.
//
// The entities table is a general-purpose store for metadata, state, and relationships.
// It provides a flexible foundation upon which higher-level concepts like tags and
// links are built. By using JSON in the Value field, entities can store any structured
// data without schema changes.
//
// # Architecture
//
// llmd uses a two-table architecture:
//   - content: versioned document storage (immutable versions)
//   - entities: metadata, state, and relationships (flexible JSON payloads)
//
// This separation keeps documents focused on content while entities handle
// everything else: tagging, linking, configuration, application state, etc.
//
// # Namespaces
//
// Entities are organized by namespace, which acts as a type discriminator.
// Core namespaces used by llmd:
//   - "core:tag" - document tags
//   - "core:link" - document relationships
//
// Applications can define custom namespaces for their own use:
//   - "todo:item" - todo list items
//   - "kanban:card" - kanban board cards
//   - "config:setting" - application configuration
//
// # Relation Field
//
// The Relation field is optional and provides a way to associate an entity
// with something else. Its meaning depends on the namespace:
//   - For tags: the document path being tagged
//   - For links: the source document path
//   - For config: might be empty (global) or a path (scoped)
//   - For app state: could be a user ID, session ID, or any identifier
//
// The Relation field is nullable - not everything needs to relate to something.
// For example, a kanban board entity might have no relation, while its cards
// relate to the board.
//
// # Value Field
//
// The Value field stores JSON data. Higher-level packages (tag, link) define
// typed Value structs for marshalling/unmarshalling. At the entity level,
// Value is a raw JSON string, providing maximum flexibility.
//
// # Soft Deletion
//
// Entities support soft deletion via the DeletedAt field. When set, the entity
// is considered deleted but remains in the database for audit purposes. The
// Purge operation permanently removes soft-deleted entities.
//
// # Keys
//
// Each entity has a unique Key (9-character nanoid). Keys are immutable and
// serve as stable identifiers across the entity's lifecycle.
//
// # Insert-Only Semantics
//
// Entities follow insert-only semantics for state changes. Rather than updating
// an entity in place, you soft-delete the old one and create a new one. This
// provides a complete audit trail of changes.
package entity

import "github.com/jpl-au/llmd/pkg/model/core"

// Entity represents metadata, state, or relationships stored in the entities table.
//
// Entity is the low-level building block for llmd's metadata system. Most users
// will interact with higher-level types (Tag, Link) rather than Entity directly.
// However, understanding Entity is useful for building custom entity types or
// working with the entities table directly.
type Entity struct {
	// ID is the database row ID (auto-incremented).
	// Not typically used in application code; prefer Key for identification.
	ID int64

	// Key is the unique identifier for this entity (9-character nanoid).
	// Keys are stable, URL-safe, and suitable for external references.
	Key string

	// Namespace identifies the entity type (e.g., "core:tag", "core:link").
	// Convention: "domain:type" format for namespacing.
	Namespace string

	// Relation is an optional reference to what this entity relates to.
	// Meaning depends on namespace: document path for tags, source path for links,
	// or any identifier relevant to the entity type. Can be empty.
	Relation string

	// Value is the JSON payload containing entity-specific data.
	// Higher-level packages define typed structs for this field.
	// At the entity level, this is a raw JSON string.
	Value string

	// Origin tracks who created this entity and from where.
	core.Origin

	// CreatedAt is the Unix timestamp (milliseconds) when created.
	// Set automatically at INSERT time.
	CreatedAt int64

	// DeletedAt is the Unix timestamp when soft-deleted, or nil if active.
	// Soft-deleted entities are excluded from normal queries but remain in
	// the database until purged.
	DeletedAt *int64
}
