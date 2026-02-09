// Package sdk provides the plugin SDK for llmd.
package sdk

// Plugin is what plugins implement.
type Plugin interface {
	Name() string
	Commands() []Command
	Exec(ctx Context, cmd string, args []string) (Response, error)
}

// Command describes a command.
type Command struct {
	Name    string
	Desc    string
	Usage   string
	Flags   []Flag
	MCP     bool
	MCPName string
}

// Flag describes a command flag.
type Flag struct {
	Name  string
	Short string
	Type  string // "bool", "string", "int"
	Desc  string
}

// Context provides execution context.
type Context struct {
	Author string
	Stdin  []byte
}

// Response is returned by commands.
type Response interface{ Response() }

// Text is plain text output.
type Text string

func (Text) Response() {}

// Data is structured output (for --json).
type Data struct{ V any }

func (Data) Response() {}

// Result has both text and structured data.
type Result struct {
	Text string
	Data any
}

func (Result) Response() {}

// API provides store access to plugins.
var API Store

// Store provides store access.
type Store interface {
	Read(path string, version int) ([]byte, error)
	Write(path string, content []byte, author, msg string) error
	Delete(path, author string) error
	Restore(path, author string) error
	Move(from, to, author string) error
	List(prefix string, opts ListOpts) ([]Doc, error)
	Exists(path string) (bool, error)
	Glob(pattern string) ([]string, error)
	Grep(query string, opts GrepOpts) ([]GrepHit, error)
	History(path string, limit int) ([]Version, error)
	Diff(a, b string, ctx int) (string, int, int, error)
	Revert(path string, version int, author, msg string) error
	Edit(path, old, new, author, msg string) error
}

// Doc represents a document.
type Doc struct {
	Path      string
	Version   int
	Author    string
	Message   string
	CreatedAt int64
	Deleted   bool
}

// ListOpts configures List.
type ListOpts struct {
	Deleted bool
	Sort    string
	Reverse bool
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

// Version is a document version.
type Version struct {
	Num       int
	Author    string
	Message   string
	CreatedAt int64
}
