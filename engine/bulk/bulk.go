// Package bulk provides batch import and export between the document
// store and the filesystem.
//
// Import reads markdown files from a directory into the store, detecting
// whether each file is new, updated, or unchanged (via XXH3 hash
// comparison). Export writes store documents back to the filesystem as
// .md files, preserving path hierarchy. Both operations support prefix
// filtering, dry-run mode, and force overwrite.
package bulk

import (
	"github.com/jpl-au/llmd/engine/documents"
	"github.com/jpl-au/llmd/engine/links"
	"github.com/jpl-au/llmd/engine/tags"
)

// Errors returned by bulk operations.
var ()

// Bulk provides batch import and export operations. It delegates
// individual document reads and writes to the Documents package,
// handling the filesystem traversal and change-detection logic, and
// carries tags and links in a .meta.yaml beside the files.
type Bulk struct {
	docs  *documents.Documents
	tags  *tags.Tags
	links *links.Links
}

// New creates a Bulk instance that delegates document, tag and link
// operations to the given handles.
func New(docs *documents.Documents, t *tags.Tags, l *links.Links) *Bulk {
	return &Bulk{docs: docs, tags: t, links: l}
}
