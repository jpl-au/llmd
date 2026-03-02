package sdk

// DocumentStore is the document storage interface. It covers reading,
// writing, searching, history, and bulk operations on documents.
type DocumentStore interface {
	// Read returns document content. Version 0 means latest;
	// a positive version reads that specific historical version.
	Read(path string, version int) ([]byte, error)

	// Write creates or updates a document, recording a new version.
	Write(path string, content []byte, author, msg string) error

	// Delete soft-deletes a document (recoverable via Restore until Vacuum).
	Delete(path, author string) error

	// Restore recovers a soft-deleted document.
	Restore(path, author string) error

	// Move renames a document, preserving version history.
	Move(from, to, author string) error

	// List returns documents matching the path prefix. Use ListOpts
	// to include deleted documents or change sort order.
	List(prefix string, opts ListOpts) ([]Doc, error)

	// Exists reports whether a (non-deleted) document exists at path.
	Exists(path string) (bool, error)

	// Edit performs a search-and-replace within a document, creating a
	// new version with the substitution applied.
	Edit(path, old, new, author, msg string) error

	// Glob returns paths matching a shell-style glob pattern
	// (e.g. "notes/*.md", "**/*.txt").
	Glob(pattern string) ([]string, error)

	// Grep performs FTS5 full-text search. The query uses SQLite FTS5
	// syntax (e.g. "hello world", "title:foo", "NEAR(a b)").
	// See GrepOpts for result modes (lines, sections, paths, etc.).
	Grep(query string, opts GrepOpts) ([]GrepHit, error)

	// History returns version history for a document, newest first.
	// Limit 0 means all versions.
	History(path string, limit int) ([]Version, error)

	// Diff computes a unified diff. Arguments use "path" or "path:version"
	// format. When only one argument is given, it diffs against the
	// previous version. ctx is lines of context (0 = default 3).
	// Returns the unified diff text, lines added, and lines removed.
	Diff(a, b string, ctx int) (string, int, int, error)

	// Revert creates a new version with the content from a previous version.
	// The old version is not modified — revert is non-destructive.
	Revert(path string, version int, author, msg string) error

	// Vacuum permanently deletes all soft-deleted data and reclaims
	// disk space. This operation cannot be undone.
	Vacuum() (VacuumResult, error)

	// Import reads files from a filesystem directory into the store.
	Import(dir string, opts ImportOpts) (*ImportResult, error)

	// Export writes documents to a filesystem directory.
	Export(prefix, dir string, opts ExportOpts) (*ExportResult, error)
}

// Doc represents a document's metadata (not its content — use Read for
// that). Returned by List. CreatedAt is a Unix timestamp. Deleted is
// true for soft-deleted documents (only visible with ListOpts.Deleted).
type Doc struct {
	Path      string
	Version   int
	Author    string
	Message   string
	CreatedAt int64
	Deleted   bool
}

// ListOpts configures List behaviour.
type ListOpts struct {
	Deleted bool   // Include soft-deleted documents
	Sort    string // Sort field (default: path alphabetical)
	Reverse bool   // Reverse the sort order
}

// GrepMode controls the granularity of grep results.
type GrepMode int

const (
	// GrepFull returns entire document content (default).
	GrepFull GrepMode = iota

	// GrepSections returns markdown sections containing matches.
	// Each section is delimited by headings.
	GrepSections

	// GrepLines returns individual matching lines with optional context.
	// Use GrepOpts.Context to specify context lines.
	GrepLines

	// GrepPaths returns only document paths, no content.
	// GrepHit.Text will be empty.
	GrepPaths

	// GrepSnippets returns FTS5-generated highlighted snippets.
	// Text includes <b></b> markers around matches.
	GrepSnippets
)

// GrepOpts configures a Grep search.
type GrepOpts struct {
	// Path limits results to documents under this path prefix.
	Path string

	// Context specifies lines of context for GrepLines mode.
	// Ignored for other modes.
	Context int

	// Mode controls result granularity. Default is GrepFull.
	Mode GrepMode
}

// GrepHit represents a search match. Which fields are populated
// depends on the GrepMode used:
//
//	GrepFull:     Path, Text (entire document)
//	GrepPaths:    Path only
//	GrepLines:    Path, Line, Column, Text, Before, After
//	GrepSections: Path, Line, Text, Section
//	GrepSnippets: Path, Text (with highlights)
type GrepHit struct {
	// Path is the document path.
	Path string

	// Line is the 1-indexed line number (GrepLines, GrepSections).
	Line int

	// Column is the 0-indexed byte position in the line (GrepLines).
	Column int

	// Text contains the matched content.
	Text string

	// Before contains context lines before the match (GrepLines).
	Before []string

	// After contains context lines after the match (GrepLines).
	After []string

	// Section is the markdown heading text (GrepSections).
	Section string
}

// Version is a single entry in a document's version history.
// Number is the 1-indexed version number. CreatedAt is a Unix timestamp.
type Version struct {
	Number    int
	Author    string
	Message   string
	CreatedAt int64
}

// VacuumResult contains the counts from a vacuum operation.
type VacuumResult struct {
	Documents int64
	Tags      int64
	Links     int64
}

// ImportOpts configures a bulk import operation.
type ImportOpts struct {
	Prefix string // Target path prefix in store
	DryRun bool   // Show what would change without importing
	Force  bool   // Import even if content is unchanged
}

// ImportResult contains the results of a bulk import.
type ImportResult struct {
	Created []string
	Updated []string
	Skipped []string
}

// ExportOpts configures a bulk export operation.
type ExportOpts struct {
	Overwrite bool // Overwrite existing files
}

// ExportResult contains the results of a bulk export.
type ExportResult struct {
	Exported []string
	Skipped  []string
}
