// Package store defines document persistence types and the Store interface.
// Implementations handle the actual database operations while consumers
// depend only on this interface, enabling testing and alternative backends.
package store

import (
	"encoding/json"
	"time"
)

// Document represents a single version of a document. Each write creates a new
// version, preserving full history for auditing and recovery.
type Document struct {
	ID        int64  // Database primary key (internal)
	Key       string // Unique 8-char identifier
	Path      string // Document path (e.g., "docs/readme")
	Content   string // Full document content
	Version   int    // Version number (1, 2, 3, ...)
	Author    string // Who created this version
	Message   string // Commit message for this version
	CreatedAt int64  // Unix timestamp of creation
	DeletedAt *int64 // Unix timestamp of deletion, nil if not deleted
}

// DocumentMeta contains document metadata without content.
// Use this for efficient listings where content isn't needed.
// Retrieve via Service.Meta().
type DocumentMeta struct {
	Key       string // Unique 8-char identifier
	Path      string // Document path
	Version   int    // Current version number
	Author    string // Author of current version
	Message   string // Message of current version
	CreatedAt int64  // Unix timestamp of current version
	DeletedAt *int64 // Deletion timestamp, nil if not deleted
	Size      int64  // Content length in bytes
}

// Link represents a connection between two documents, enabling relationship
// tracking and graph-based navigation. Tags allow categorising link types.
type Link struct {
	ID         string // Unique 8-char identifier
	FromPath   string // Source document path
	FromSource string // Source table
	ToPath     string // Target document path
	ToSource   string // Target table
	Tag        string // Optional link type (empty string for untagged)
	CreatedAt  int64  // Unix timestamp of creation
	DeletedAt  *int64 // Unix timestamp of deletion, nil if not deleted
}

// LinkJSON is the API-friendly representation of a Link with formatted
// timestamps and omitted internal fields.
type LinkJSON struct {
	ID         string `json:"id"`
	FromPath   string `json:"from_path"`
	FromSource string `json:"from_source,omitempty"`
	ToPath     string `json:"to_path"`
	ToSource   string `json:"to_source,omitempty"`
	Tag        string `json:"tag,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ToJSON converts a Link to its API representation with RFC3339 timestamps.
func (l *Link) ToJSON() LinkJSON {
	return LinkJSON{
		ID:         l.ID,
		FromPath:   l.FromPath,
		FromSource: l.FromSource,
		ToPath:     l.ToPath,
		ToSource:   l.ToSource,
		Tag:        l.Tag,
		CreatedAt:  time.Unix(l.CreatedAt, 0).UTC().Format(time.RFC3339),
	}
}

// TagOptions configures a tag operation.
type TagOptions struct {
	Source  string // Source table
	MaxPath int    // Max path length for validation
}

// NewTagOptions returns TagOptions with sensible defaults.
func NewTagOptions() TagOptions {
	return TagOptions{Source: "documents"}
}

// WithSource sets the source table.
func (o TagOptions) WithSource(s string) TagOptions {
	o.Source = s
	return o
}

// LinkOptions configures a link operation.
type LinkOptions struct {
	FromSource string // Source table for "from" endpoint
	ToSource   string // Source table for "to" endpoint
	MaxPath    int    // Max path length for validation
}

// NewLinkOptions returns LinkOptions with sensible defaults.
func NewLinkOptions() LinkOptions {
	return LinkOptions{
		FromSource: "documents",
		ToSource:   "documents",
	}
}

// WithFromSource sets the source table for the "from" endpoint.
func (o LinkOptions) WithFromSource(s string) LinkOptions {
	o.FromSource = s
	return o
}

// WithToSource sets the source table for the "to" endpoint.
func (o LinkOptions) WithToSource(s string) LinkOptions {
	o.ToSource = s
	return o
}

// DocJSON is the API-friendly representation of a Document. It uses RFC3339
// timestamps and allows optional content omission for bandwidth efficiency.
type DocJSON struct {
	Key       string `json:"key"`
	Path      string `json:"path"`
	Content   string `json:"content,omitempty"`
	Version   int    `json:"version"`
	Author    string `json:"author"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"created_at"`
	Deleted   bool   `json:"deleted,omitempty"`
}

// ToJSON converts a Document to its API representation. The content parameter
// controls whether to include document content, allowing efficient listings.
func (d *Document) ToJSON(content bool) DocJSON {
	j := DocJSON{
		Key:       d.Key,
		Path:      d.Path,
		Version:   d.Version,
		Author:    d.Author,
		Message:   d.Message,
		CreatedAt: time.Unix(d.CreatedAt, 0).UTC().Format(time.RFC3339),
		Deleted:   d.DeletedAt != nil,
	}
	if content {
		j.Content = d.Content
	}
	return j
}

// MarshalJSON encodes a value with indentation for human-readable CLI output.
// Use this instead of json.Marshal when the output will be displayed to users.
func MarshalJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// WriteOptions configures a write operation.
type WriteOptions struct {
	Author     string
	Message    string
	MaxPath    int   // 0 means no limit (not recommended for writes)
	MaxContent int64 // 0 means no limit (not recommended for writes)
}

// DeleteOptions configures a delete operation.
type DeleteOptions struct {
	MaxPath int
}

// DeleteVersionOptions configures a version-specific delete operation.
type DeleteVersionOptions struct {
	MaxPath int
}

// RestoreOptions configures a restore operation.
type RestoreOptions struct {
	MaxPath int
}

// MoveOptions configures a move operation.
type MoveOptions struct {
	MaxPath int
}

// CopyOptions configures a copy operation.
type CopyOptions struct {
	MaxPath int
}

// Stats provides aggregate database statistics for capacity planning and
// operational visibility. Enables developers to understand store utilisation
// without querying individual tables.
type Stats struct {
	Documents       int64 // Active (non-deleted) document count
	DeletedDocs     int64 // Soft-deleted documents pending vacuum
	TotalVersions   int64 // Sum of all document versions (history depth)
	Tags            int64 // Active tag associations
	Links           int64 // Active link relationships
	Authors         int64 // Distinct authors who have written documents
	OldestDoc       int64 // Unix timestamp of earliest document
	NewestDoc       int64 // Unix timestamp of most recent write
	OldestDeletedAt int64 // Unix timestamp of earliest soft-delete (0 if none)
}
