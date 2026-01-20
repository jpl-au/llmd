package history

import (
	"github.com/jpl-au/llmd/pkg/model/document"
)

// DiffResult contains the result of comparing two documents.
type DiffResult struct {
	Doc1 *document.Document
	Doc2 *document.Document
}

// Diff compares two documents.
// This is a simple operation - callers are responsible for fetching documents.
func Diff(doc1, doc2 *document.Document) *DiffResult {
	return &DiffResult{
		Doc1: doc1,
		Doc2: doc2,
	}
}
