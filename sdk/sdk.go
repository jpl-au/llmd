// Package sdk provides the extension SDK for llmd.
package sdk

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/jpl-au/llmd/pkg/events"
)

// Common errors returned by commands. These allow callers to
// programmatically distinguish error categories without parsing
// strings. Commands attach a kind with Errorf, which keeps the kind's
// own text out of the message:
//
//	sdk.Errorf(sdk.ErrMissingArg, "cat: a path is required.")
var (
	// ErrMissingArg means a required positional argument was not provided.
	ErrMissingArg = errors.New("missing required argument")

	// ErrInvalidArg means an argument was present but malformed
	// (e.g. non-numeric version string).
	ErrInvalidArg = errors.New("invalid argument")

	// ErrUnknownCmd means the command name was not found in the extension's
	// command table.
	ErrUnknownCmd = errors.New("unknown command")

	// ErrNotFound means the requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrNoneReady means no task in the column can be claimed: every
	// task is flagged human, hold or blocked, waits on a dependency, or
	// sits in the backlog without a spec.
	ErrNoneReady = errors.New("no task ready")

	// ErrNoSpec means a task's spec document is missing or has no
	// content beyond the title heading. Tasks cannot leave the
	// backlog until the spec describes what the work actually is.
	ErrNoSpec = errors.New("task has no spec")

	// ErrExists means the resource already exists.
	ErrExists = errors.New("already exists")

	// ErrCycle means a dependency would create a cycle.
	ErrCycle = errors.New("dependency cycle detected")

	// ErrAgentRunning means a task already has a running agent.
	ErrAgentRunning = errors.New("agent already running for task")

	// ErrAgentNotRunning means the task's agent run has already
	// completed, failed, or been stopped.
	ErrAgentNotRunning = errors.New("agent run is not running")

	// ErrNotReady means a task's dependencies are not satisfied.
	ErrNotReady = errors.New("task dependencies not satisfied")

	// ErrNoMatch means an edit's search string was not found in the
	// target document.
	ErrNoMatch = errors.New("no match found")

	// ErrNotUnique means an edit's search string matched more than one
	// place in the target document. The caller must either disambiguate
	// with more context or opt into ReplaceAll.
	ErrNotUnique = errors.New("search string is not unique")

	// ErrNoOp means an edit's old and new strings were identical, so the
	// operation would have produced no change.
	ErrNoOp = errors.New("old and new are identical")

	// ErrUnavailable means something the command depends on outside
	// the store is missing or not in a usable state: git is not
	// installed, the directory is not a repository, HEAD is detached.
	ErrUnavailable = errors.New("not available")
)

// Runtime is the host's surface for commands that need to reach back
// into it: dispatching another command (the MCP and HTTP servers),
// listing the command table, watching store events, or creating a
// store. It is carried on every [Context] so commands never depend on
// package-level state.
type Runtime interface {
	// Dispatch executes a command by name. The context controls
	// cancellation and timeout for the dispatched command.
	Dispatch(ctx context.Context, cmd string, args []string, author string, stdin []byte, dbPath string) (Response, error)

	// Commands returns all registered commands, keyed by name.
	Commands() map[string]*Command

	// Subscribe registers a callback that receives every store event.
	// The callback runs synchronously on the emitting goroutine and
	// must not block. A no-op when the host has no store.
	Subscribe(fn func(events.Event))

	// Watch reports changes other processes make to the store, by
	// polling at the interval, until ctx is done. Each change reaches
	// every Subscribe callback as a store.changed event. Changes this
	// process makes are already delivered as typed events and are not
	// repeated. A host without a store waits for one to be attached,
	// then watches it. Returns nil once ctx is done.
	Watch(ctx context.Context, interval time.Duration) error

	// Init creates a new store at dbPath (or the default path when
	// empty) and returns the path created. When a store is already
	// there it changes nothing and returns its path with an ErrExists
	// error.
	Init(dbPath string) (string, error)
}

// Extension is the interface that command providers implement. The host
// discovers extensions at startup (compiled extensions via init()),
// calls Commands() to build its command table, and dispatches execution
// to Exec().
//
// Extensions receive raw args and parse flags themselves - the host
// does not interpret command arguments. See doc.go for a full example.
type Extension interface {
	Name() string
	Commands() []Command
	Exec(ctx Context, cmd string, args []string) (Response, error)
}

// Command describes an extension command. Name is the invocation word
// (e.g. "cat", "ls"). Desc is a short one-line description shown in
// help output. Usage shows the argument pattern (e.g. "cat [options] <path>...").
//
// MCP controls whether the command is exposed as an MCP tool.
// MCPName overrides the tool name to avoid collisions with host tools
// (e.g. "grep" -> "llmd_grep").
//
// NeedsAuthor marks commands that create versions (write, edit, rm, mv,
// restore, revert). The host checks this before dispatch and requires
// an author to be configured.
//
// Streams marks commands that take over the raw I/O streams (stdin/stdout)
// for their own protocol. The host must not pre-read stdin for these
// commands. Examples: mcp (JSON-RPC over stdio), serve (HTTP).
type Command struct {
	Name  string
	Desc  string
	Usage string
	Flags []Flag
	MCP   bool

	// MCPName is the tool name when it must differ from Name, as for
	// grep, find, glob and init, whose names clash with an agent's
	// built-in tools.
	MCPName string

	// Brief is the command's description for an agent, hand-written
	// for commands whose Desc and Flags would run long: a compact list
	// of subcommands and the flags that matter. It is shown in the MCP
	// tool list and in the --help of a piped llmd, where a model reads
	// it (see AgentHelp). When empty, Summary and the flags are shown
	// instead. The full Desc stays for a person's --help.
	Brief string

	// Guide is the guide page that documents the command, named in
	// its help as "Details: guide <page>". Empty means the page is the
	// command's name; "-" means there is no page, and help names none.
	Guide string

	NeedsAuthor bool
	Streams     bool

	// Routes lists the command's HTTP routes. A command with none gets
	// the server's default: GET /<name> and GET /<name>/{path...} for
	// reads, or POST for a command that needs an author. Commands with
	// subcommands declare their routes so that every mutation is a
	// POST or DELETE and never a GET.
	Routes []Route
}

// Route is one HTTP route for a command.
//
// Pattern is a net/http pattern without the method, such as
// "/task/move/{id}/{column}". A trailing "{path...}" wildcard holds a
// document path with slashes; any earlier wildcard is a single URL
// segment, so a document path there is URL-encoded.
//
// Args are the arguments the command receives, in order. An entry of
// the form "{name}" is replaced by that part of the URL and dropped
// when empty. An entry "{?name}" is replaced by the query parameter
// name, for free text a URL segment cannot carry well (edit's old and
// new text, a sed expression); that parameter is then not a flag. Any
// other entry is passed as written, which is how a route selects a
// subcommand or a flag. Other query parameters are appended as flags
// and the request body is the command's stdin, for every route.
type Route struct {
	Method  string
	Pattern string
	Args    []string
}

// Context carries per-invocation data to commands. It embeds
// [context.Context] for cancellation and timeout propagation, and
// holds request-scoped domain store instances so commands access the
// store through the context. There is no package-level state: a
// command receives everything it may need here.
//
// Author is the configured user identity (required for write
// operations). Stdin is the command's input source: a pipe or file
// for the CLI, the request content for MCP and HTTP, or nil when there
// is none. Commands read it through Input or OptionalInput, only when
// they need it, so a command given its content as an argument never
// waits on an open pipe. Runtime reaches back into the host.
type Context struct {
	context.Context

	Author string
	Stdin  io.Reader
	// TTY reports whether the output goes to a person at a terminal:
	// true for the CLI writing to one, false for a pipe, MCP and HTTP.
	// Commands style output (tables, colour) only when it is set; the
	// server's own terminal says nothing about its clients.
	TTY     bool
	DBPath  string // Override database path (empty = default)
	Runtime Runtime

	// Domain stores bound to this request's lifecycle context. Nil
	// when the host has no store (discovery-only mode).
	Documents  DocumentStore
	Tasks      TaskStore
	Links      LinkStore
	Tags       TagStore
	Audits     AuditStore
	Queue      QueueStore
	Activities ActivityStore
	Mirror     MirrorStore
	Git        GitStore
	Agents     AgentStore
	Rules      RuleStore
	Hooks      HookStore
}

// Response is the marker interface for command return values. It uses
// a marker method instead of a concrete type so the four result types
// (Text, Markdown, Data, Result) remain distinct at the type-switch
// level - the host switches on the concrete type to decide output
// format.
//
// Choose between the four implementations:
//   - [Text]: plain text or already-styled terminal output (tables,
//     status lines), displayed as-is. The host never re-renders Text.
//   - [Markdown]: markdown source. The host renders it via glamour for
//     interactive terminals and emits it raw for pipes, MCP, HTTP and
//     other consumers that will reprocess the content.
//   - [Data]: structured data, always JSON-encoded (machine-only output).
//   - [Result]: both already-formatted text and structured data, for
//     commands whose human form is not markdown (e.g. tables for "ls").
//
// Rule of thumb: if your text output is markdown that came from a
// document or that you generated as markdown, use Markdown so users
// get a rendered view. If your text output is a lipgloss-styled table
// or status display, use Text or Result so the host leaves it alone.
type Response interface{ Response() }

// Text is plain text output, displayed as-is to the terminal. The
// host does not re-render or transform it. Use for simple messages
// ("Deleted notes/todo.md") and for already-styled terminal output
// (lipgloss tables, status displays) that must not be touched.
type Text string

func (Text) Response() {}

// Markdown carries markdown source content for human display, paired
// with optional structured data for machine consumption. The host
// renders the Text field via glamour for interactive terminals and
// returns it raw for pipes, --json, MCP, HTTP, hook payloads, and any
// other consumer that will reprocess the content. When --json is set
// (or the consumer asks for JSON), Data is encoded instead.
//
// Use Markdown when a command's primary output is markdown content
// from documents (cat, guide, future preview/head/tail commands) so
// the rendering decision happens once at the host boundary instead
// of in every command. Data is optional - leave it nil for commands
// that have no structured representation.
type Markdown struct {
	Text string
	Data any
}

func (Markdown) Response() {}

// Data is structured output that is always JSON-encoded regardless of
// --json flag. Use when the output is only meaningful as structured data
// (e.g. machine-to-machine communication).
type Data struct{ V any }

func (Data) Response() {}

// Result carries both human-readable text and structured data. The host
// displays Text for terminal output and Data when --json is set. Use
// when both humans and machines consume the output (e.g. "ls" shows a
// table for humans but returns document metadata as JSON for scripts).
//
// Result.Text is treated as already-formatted output and is never
// re-rendered. For markdown content that should be rendered for humans,
// use [Markdown] instead.
type Result struct {
	Text string
	Data any
	// Note is a line about the result rather than part of it, such as
	// "(5 more not shown; use --all)". The CLI prints it on stderr so
	// piped output stays one item per line; MCP appends it to the text.
	Note string
}

func (Result) Response() {}

// JSONText is the JSON form of a response that has no structured data:
// its text, under one key. An object rather than a bare string, so a
// command can add fields later without breaking what parses it.
type JSONText struct {
	Text string `json:"text"`
}

// JSON returns the value a response is encoded as when JSON is asked
// for (--json, or Output: json over HTTP): its structured data when it
// has some, otherwise its text as a JSONText. Every transport calls
// this, so --json means the same thing from every command and surface.
func JSON(r Response) any {
	switch v := r.(type) {
	case Text:
		return JSONText{Text: string(v)}
	case Markdown:
		return jsonData(v.Data, v.Text)
	case Result:
		return jsonData(v.Data, v.Text)
	case Data:
		return v.V
	}
	return JSONText{}
}

// jsonData is a response's data, or its text when it has none. Data
// that is only a string (cat's document) is text too, so --json never
// prints a bare string.
func jsonData(data any, text string) any {
	switch d := data.(type) {
	case nil:
		return JSONText{Text: text}
	case string:
		return JSONText{Text: d}
	}
	return data
}
