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
	"errors"
	"fmt"
	"strings"

	"github.com/jpl-au/qwr"
)

var (
	// ErrInvalidQuery is returned when the FTS5 query syntax is invalid.
	// Common causes: unbalanced quotes, invalid operators, empty query.
	ErrInvalidQuery = errors.New("the search query is not valid. Check that its quotes and brackets are balanced.")

	// ErrInvalidGlob is returned when the glob pattern is malformed.
	// Common causes: unbalanced brackets, invalid character classes.
	ErrInvalidGlob = errors.New("that is not a valid glob pattern.")
)

// Search provides full-text search and glob matching over the document store.
// It operates on the FTS5 index which contains the latest non-deleted version
// of each document.
type Search struct {
	db *qwr.Manager
}

// New creates a Search instance using the given database connection.
// The database must have the content_fts table initialised.
func New(db *qwr.Manager) *Search {
	return &Search{db: db}
}

// queryError turns a failed FTS5 query into ErrInvalidQuery when the
// query text was at fault (an unbalanced quote, bad syntax), so the
// reader is told to fix the query rather than shown SQLite's message.
func queryError(err error) error {
	msg := err.Error()
	for _, s := range []string{"fts5", "unterminated string", "syntax error"} {
		if strings.Contains(msg, s) {
			return ErrInvalidQuery
		}
	}
	return fmt.Errorf("fts query: %w", err)
}
