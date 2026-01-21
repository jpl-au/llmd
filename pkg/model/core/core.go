// Package core provides shared types embedded across llmd models.
//
// These types capture common fields that appear in multiple models,
// avoiding duplication and ensuring consistency. They are designed
// to be embedded in model structs rather than used directly.
//
// # Provenance
//
// Provenance tracks the origin of data: who created it, from where,
// and when. Every stored item in llmd (documents, tags, links, entities)
// includes provenance information for auditing and traceability.
//
// # Relationship to internal/llmd/core
//
// The internal/llmd/core package provides WriteContext for write operations
// (input). This package provides Provenance for stored data (output).
// WriteContext captures intent; Provenance captures what was recorded.
package core

// Provenance tracks the origin and creation of stored data.
//
// Embed this struct in models that record who created something,
// from what source, and when. All persistent data in llmd includes
// provenance for auditing and traceability.
//
// Example embedding:
//
//	type Tag struct {
//	    Key      string
//	    Relation string
//	    Value    Value
//	    core.Provenance
//	}
type Provenance struct {
	// Author identifies who created this record.
	// Typically a username, email, or "system" for automated operations.
	// Required for all write operations.
	Author string

	// Source identifies where this record originated.
	// Standard values: "cli", "api", "mcp", "import", "sync".
	// Helps track how data entered the system.
	Source string

	// CreatedAt is the Unix timestamp (seconds) when the record was created.
	// Set automatically at write time; never modified afterward.
	CreatedAt int64
}
