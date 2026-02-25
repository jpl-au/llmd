// Package meta provides content metadata computation for documents.
//
// Metadata (size in bytes, line count) is computed at write time and
// stored as a JSON blob alongside the document version. This avoids
// re-scanning content for listings and statistics. The computation is
// intentionally simple — more expensive analysis (word count, readability
// scores, etc.) can be added here later without changing the storage
// format.
package meta

import (
	"strings"

	"github.com/jpl-au/llmd/pkg/model/document"
)

// Compute returns metadata for the given content.
func Compute(content string) *document.Meta {
	return &document.Meta{
		Size:  len(content),
		Lines: countLines(content),
	}
}

// countLines returns the number of lines in content.
// Empty content has 0 lines. Content without newlines has 1 line.
func countLines(content string) int {
	if len(content) == 0 {
		return 0
	}
	n := strings.Count(content, "\n")
	// Add 1 for final line if content doesn't end with newline
	if len(content) > 0 && content[len(content)-1] != '\n' {
		n++
	}
	return n
}
