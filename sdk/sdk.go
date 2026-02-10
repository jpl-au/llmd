// Package sdk provides the plugin SDK for llmd.
package sdk

import "errors"

// Common errors returned by commands. These allow callers to
// programmatically distinguish error categories without parsing
// strings. Commands wrap these with context, e.g.:
//
//	fmt.Errorf("cat: %w", sdk.ErrMissingArg)
var (
	// ErrMissingArg means a required positional argument was not provided.
	ErrMissingArg = errors.New("missing required argument")

	// ErrInvalidArg means an argument was present but malformed
	// (e.g. non-numeric version string).
	ErrInvalidArg = errors.New("invalid argument")

	// ErrUnknownCmd means the command name was not found in the plugin's
	// command table.
	ErrUnknownCmd = errors.New("unknown command")
)

// Plugin is the interface that command providers implement. The host
// discovers plugins at startup (compiled extensions via init(), dynamic
// plugins via Yaegi), calls Commands() to build its command table, and
// dispatches execution to Exec().
//
// Plugins receive raw args and parse flags themselves — the host does
// not interpret command arguments. See doc.go for a full example.
type Plugin interface {
	Name() string
	Commands() []Command
	Exec(ctx Context, cmd string, args []string) (Response, error)
}

// Command describes a plugin command. Name is the invocation word
// (e.g. "cat", "ls"). Desc is a short one-line description shown in
// help output. Usage shows the argument pattern (e.g. "cat [options] <path>...").
//
// MCP controls whether the command is exposed as an MCP tool.
// MCPName overrides the tool name to avoid collisions with host tools
// (e.g. "grep" → "llmd_grep").
//
// NeedsAuthor marks commands that create versions (write, edit, rm, mv,
// restore, revert). The host checks this before dispatch and requires
// an author to be configured.
type Command struct {
	Name        string
	Desc        string
	Usage       string
	Flags       []Flag
	MCP         bool
	MCPName     string
	NeedsAuthor bool
}

// Flag describes a command flag for help output. The host does not parse
// flags — this metadata is only used for --help display and MCP tool
// descriptions. Commands parse their own flags from the raw args slice.
type Flag struct {
	Name  string // Long flag name (e.g. "version" for --version)
	Short string // Optional short form (e.g. "n" for -n)
	Type  string // "bool", "string", or "int"
	Desc  string
}

// Context carries per-invocation data to commands. Author is the
// configured user identity (required for write operations). Stdin
// contains piped input when present, or nil for interactive terminals.
type Context struct {
	Author string
	Stdin  []byte
}

// Response is the marker interface for command return values. It uses
// a marker method instead of a concrete type so the three result types
// (Text, Data, Result) remain distinct at the type-switch level — the
// host switches on the concrete type to decide output format.
//
// Choose between the three implementations:
//   - [Text]: plain text, displayed as-is (most commands)
//   - [Data]: structured data, always JSON-encoded (machine-only output)
//   - [Result]: both text and data (text for terminals, data for --json)
type Response interface{ Response() }

// Text is plain text output, displayed as-is to the terminal.
// Use for simple messages like "Deleted notes/todo.md".
type Text string

func (Text) Response() {}

// Data is structured output that is always JSON-encoded regardless of
// --json flag. Use when the output is only meaningful as structured data
// (e.g. machine-to-machine communication).
type Data struct{ V any }

func (Data) Response() {}

// Result carries both human-readable text and structured data. The host
// displays Text for terminal output and Data when --json is set. Use
// when both humans and machines consume the output (e.g. "ls" shows a
// table for humans but returns document metadata as JSON for scripts).
type Result struct {
	Text string
	Data any
}

func (Result) Response() {}

// API is the global store handle, set by the host before command
// execution. Plugins call sdk.API.Read(), sdk.API.Write(), etc.
// It is nil when the host is created without a store (e.g. for
// discovery-only operations like "llmd plugins").
var API Store

// Dispatch executes a command by name through the host. Set by the host
// at startup. Used by commands that need to invoke other commands (e.g.
// MCP server dispatching tool calls).
var Dispatch func(cmd string, args []string, author string, stdin []byte) (Response, error)

// AllCommands returns all registered commands. Set by the host at startup.
var AllCommands func() map[string]*Command

// PluginNames returns the names of loaded yaegi plugins. Set by the host.
var PluginNames func() []string

// Store is the document store interface exposed to plugins. It
// abstracts the internal storage engine so plugins don't depend on
// database internals.
type Store interface {
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

	// Edit performs a search-and-replace within a document, creating a
	// new version with the substitution applied.
	Edit(path, old, new, author, msg string) error

	// Vacuum permanently deletes all soft-deleted data and reclaims
	// disk space. This operation cannot be undone.
	Vacuum() (VacuumResult, error)

	// TagAdd attaches a tag to a document.
	TagAdd(path, name, author string) error

	// TagRemove removes a tag from a document.
	TagRemove(path, name, author string) error

	// TagList returns all tags on a document.
	TagList(path string) ([]Tag, error)

	// Tags returns all tags in the store with usage counts.
	Tags() ([]TagInfo, error)

	// TagFind returns document paths that have the given tag.
	TagFind(name string) ([]string, error)

	// LinkAdd creates a directed link between two documents.
	LinkAdd(from, to, label, author string) error

	// LinkRemove removes a link between two documents.
	LinkRemove(from, to, author string) error

	// LinkList returns links for a document. Dir controls direction:
	// "out" (default), "in", or "both".
	LinkList(path, dir string) ([]Link, error)

	// Import reads files from a filesystem directory into the store.
	Import(dir string, opts ImportOpts) (*ImportResult, error)

	// Export writes documents to a filesystem directory.
	Export(prefix, dir string, opts ExportOpts) (*ExportResult, error)
}

// VacuumResult contains the counts from a vacuum operation.
type VacuumResult struct {
	Documents int64
	Tags      int64
	Links     int64
}

// Tag represents a tag attached to a document.
type Tag struct {
	Name string
	Path string
}

// TagInfo represents a tag with its usage count across documents.
type TagInfo struct {
	Name  string
	Count int
}

// Link represents a directed relationship between two documents.
type Link struct {
	From  string
	To    string
	Label string
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

// ListOpts configures List behavior.
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
// Num is the 1-indexed version number. CreatedAt is a Unix timestamp.
type Version struct {
	Num       int
	Author    string
	Message   string
	CreatedAt int64
}
