// Package search provides full-text search and path glob matching for documents.
//
// Full-text search uses SQLite FTS5 and supports its query syntax:
//   - Simple terms: "hello world" matches documents containing both words
//   - Phrases: `"exact phrase"` matches the exact sequence
//   - Prefix: "auto*" matches automate, automatic, etc.
//   - Boolean: "foo AND bar", "foo OR bar", "NOT foo"
//   - Negation: "foo -bar" excludes documents with bar
//
// Search results can be returned in different granularities via [Mode]:
//   - [ModeFull]: entire document content (default)
//   - [ModePaths]: only document paths, no content
//   - [ModeLines]: matching lines with configurable context
//   - [ModeSections]: markdown sections containing matches
//   - [ModeSnippets]: FTS5-generated snippets with highlights
//
// Path glob matching uses standard glob patterns with ** for recursive matching.
package search

import (
	"database/sql"
	"errors"
)

var (
	// ErrInvalidQuery is returned when the FTS5 query syntax is invalid.
	// Common causes: unbalanced quotes, invalid operators, empty query.
	ErrInvalidQuery = errors.New("invalid FTS query")

	// ErrInvalidGlob is returned when the glob pattern is malformed.
	// Common causes: unbalanced brackets, invalid character classes.
	ErrInvalidGlob = errors.New("invalid glob pattern")
)

// Search provides full-text search and glob matching over the document store.
// It operates on the FTS5 index which contains the latest non-deleted version
// of each document.
type Search struct {
	db *sql.DB
}

// New creates a Search instance using the given database connection.
// The database must have the content_fts table initialised.
func New(db *sql.DB) *Search {
	return &Search{db: db}
}
