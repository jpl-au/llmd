// Package sdk provides the plugin SDK for llmd.
package sdk

// Plugin is what plugins implement.
type Plugin interface {
	Name() string
	Commands() []Command
	Exec(ctx Context, cmd string, args []string) (Result, error)
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

// Result is returned by commands.
type Result interface{ result() }

// Text is plain text output.
type Text string

func (Text) result() {}

// Data is structured output (for --json).
type Data struct{ V any }

func (Data) result() {}

// Rich has both text and structured data.
type Rich struct {
	Text string
	Data any
}

func (Rich) result() {}

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

// GrepOpts configures Grep.
type GrepOpts struct {
	Path    string
	Context int
}

// GrepHit is a grep result.
type GrepHit struct {
	Path   string
	Line   int
	Text   string
	Before []string
	After  []string
}

// Version is a document version.
type Version struct {
	Num       int
	Author    string
	Message   string
	CreatedAt int64
}
