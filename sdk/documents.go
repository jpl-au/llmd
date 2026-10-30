package sdk

import "time"

// DocumentStore is the document storage interface. It covers reading,
// writing, searching, history, and bulk operations on documents.
//
// Mutation methods take an options struct rather than positional
// author/message arguments. This keeps the contract stable as new
// options are added (e.g. EditOpts.ReplaceAll). Each opts struct
// requires Author; everything else is optional.
type DocumentStore interface {
	// Read returns document content. See ReadOpts for the available
	// options (version selection, line-based slicing).
	Read(path string, opts ReadOpts) ([]byte, error)

	// Write creates or updates a document, recording a new version.
	// See WriteOpts for the available options.
	Write(path string, content []byte, opts WriteOpts) error

	// Delete soft-deletes a document (recoverable via Restore until
	// Vacuum). See DeleteOpts for the available options.
	Delete(path string, opts DeleteOpts) error

	// Restore recovers a soft-deleted document. See RestoreOpts for
	// the available options.
	Restore(path string, opts RestoreOpts) error

	// Move renames a document, preserving version history. See
	// MoveOpts for the available options.
	Move(from, to string, opts MoveOpts) error

	// List returns documents matching the path prefix. Use ListOpts
	// to include deleted documents or change sort order.
	List(prefix string, opts ListOpts) ([]Doc, error)

	// Exists reports whether a (non-deleted) document exists at path.
	Exists(path string) (bool, error)

	// Edit performs a search-and-replace within a document, creating a
	// new version with the substitution applied. By default the search
	// string must occur exactly once; set EditOpts.ReplaceAll to allow
	// (and require) multiple matches. See EditOpts for the full set of
	// options.
	Edit(path, old, new string, opts EditOpts) error

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

	// Revert creates a new version with the content from a previous
	// version. The old version is not modified - revert is
	// non-destructive. See RevertOpts for the available options.
	Revert(path string, version int, opts RevertOpts) error

	// Vacuum permanently deletes soft-deleted data and reclaims disk
	// space. Outside a dry run this operation cannot be undone.
	Vacuum(opts VacuumOpts) (VacuumResult, error)

	// Import reads files from a filesystem directory into the store.
	Import(dir string, opts ImportOpts) (*ImportResult, error)

	// Export writes documents to a filesystem directory.
	Export(prefix, dir string, opts ExportOpts) (*ExportResult, error)

	// Preview returns the first lines of a document, skipping the title
	// heading and leading blank lines. Useful for task board summaries
	// and review views. Returns empty string if the path is empty or
	// the document does not exist.
	Preview(path string, lines int) (string, error)

	// References returns the documents path's markdown links to and the documents whose markdown links to path. See [References].
	References(path string) (*References, error)
}

// References are the documents one document's markdown refers to (Out) and the documents whose markdown refers to it (In), by path. A link refers to a document when it names that document's .md file relative to the folder of the document it is in. Only documents that exist are listed. Out is in the order the links first appear, In in path order.
type References struct {
	Out []string `json:"out"`
	In  []string `json:"in"`
}

// Doc represents a document's metadata (not its content - use Read for
// that). Returned by List. CreatedAt is a Unix timestamp. Deleted is
// true for soft-deleted documents (only visible with ListOpts.Deleted).
type Doc struct {
	Key       string `json:"key"`
	Path      string `json:"path"`
	Version   int    `json:"version"`
	Author    string `json:"author"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"created_at"`
	Deleted   bool   `json:"deleted"`
}

// ReadOpts configures a Read operation.
//
// Version selects a specific historical version. Zero means the
// latest (non-deleted) version.
//
// Offset and Limit provide line-based slicing - essential for AI
// consumers that need to fetch a specific slice of a long document
// without burning context on the rest of the file. Offset is the
// 1-indexed line number to start from (0 means line 1). Limit caps
// the number of lines returned (0 means no cap). Line counting
// matches what a human sees in an editor: lines separated by '\n',
// inclusive on the start line.
type ReadOpts struct {
	Version int
	Offset  int
	Limit   int
}

// WriteOpts configures a Write operation. Author is required; Message
// is an optional version message (equivalent to a git commit message).
type WriteOpts struct {
	Author  string
	Message string
}

// DeleteOpts configures a Delete operation. Author is required.
// Delete is recorded against the document's history but does not
// persist a message.
type DeleteOpts struct {
	Author string
}

// RestoreOpts configures a Restore operation. Author is required.
type RestoreOpts struct {
	Author string
}

// MoveOpts configures a Move operation. Author is required.
type MoveOpts struct {
	Author string
}

// RevertOpts configures a Revert operation. Author is required;
// Message is an optional version message describing the revert.
type RevertOpts struct {
	Author  string
	Message string
}

// EditOpts configures an Edit operation.
//
// Author is required (Edit creates a new version, and every version
// records who made it). Message is an optional version message,
// equivalent to a git commit message. ReplaceAll opts into substituting
// every occurrence of the search string instead of requiring exactly
// one match.
type EditOpts struct {
	Author     string
	Message    string
	ReplaceAll bool
}

// ListOpts configures List behaviour.
type ListOpts struct {
	Deleted bool      // Include soft-deleted documents
	Sort    string    // Sort field (default: path alphabetical)
	Reverse bool      // Reverse the sort order
	Since   time.Time // Only documents updated after this time
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
	Path string `json:"path"`

	// Line is the 1-indexed line number (GrepLines, GrepSections).
	Line int `json:"line"`

	// Column is the 0-indexed byte position in the line (GrepLines).
	Column int `json:"column"`

	// Text contains the matched content.
	Text string `json:"text"`

	// Before contains context lines before the match (GrepLines).
	Before []string `json:"before"`

	// After contains context lines after the match (GrepLines).
	After []string `json:"after"`

	// Section is the markdown heading text (GrepSections).
	Section string `json:"section"`
}

// Version is a single entry in a document's version history.
// Number is the 1-indexed version number. CreatedAt is a Unix timestamp.
type Version struct {
	Path      string `json:"path"` // document path, resolved when history was requested by key
	Number    int    `json:"number"`
	Author    string `json:"author"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"created_at"`
}

// VacuumResult contains the counts from a vacuum operation.
type VacuumResult struct {
	Documents int64    `json:"documents"`
	Tags      int64    `json:"tags"`
	Links     int64    `json:"links"`
	Messages  int64    `json:"messages"` // queue messages older than the configured retention
	Paths     []string `json:"paths"`    // documents purged, or that would be, in path order
	DryRun    bool     `json:"dry_run"`  // nothing was changed
}

// VacuumOpts narrows a vacuum. DryRun reports what would be purged
// without changing anything. Before purges only items deleted before
// that time; the zero time purges everything soft-deleted. Queue
// messages are governed by the store's retention (the queue.retention
// configuration key), not by Before.
type VacuumOpts struct {
	DryRun bool
	Before time.Time
}

// ImportOpts configures a bulk import operation.
type ImportOpts struct {
	Author string // Who the new versions are recorded under; required
	Prefix string // Target path prefix in store
	DryRun bool   // Show what would change without importing
}

// ImportResult contains the results of a bulk import.
type ImportResult struct {
	Created []string   `json:"created"`
	Updated []string   `json:"updated"`
	Skipped []string   `json:"skipped"`
	Meta    MetaCounts `json:"meta"`
	Errors  []string   `json:"errors"` // "path: reason" for files, tags or links that could not be imported
}

// MetaCounts counts the tag and link changes an import or mirror push
// made from the .meta.yaml beside the files.
type MetaCounts struct {
	Tagged   int `json:"tagged"`
	Untagged int `json:"untagged"`
	Linked   int `json:"linked"`
	Unlinked int `json:"unlinked"`
}

// ExportOpts configures a bulk export operation.
type ExportOpts struct {
	Overwrite bool // Overwrite existing files
}

// ExportResult contains the results of a bulk export.
type ExportResult struct {
	Exported []string `json:"exported"`
	Skipped  []string `json:"skipped"`
	Errors   []string `json:"errors"` // "path: reason" for documents that could not be exported
}
