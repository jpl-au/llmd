// Package sdk provides the plugin SDK for llmd.
package sdk

import (
	"context"
	"errors"
)

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

	// ErrNotFound means the requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrNoSpec means a task's spec document is missing or has no
	// content beyond the title heading. Tasks cannot leave the
	// backlog until the spec describes what the work actually is.
	ErrNoSpec = errors.New("task has no spec")

	// ErrExists means the resource already exists.
	ErrExists = errors.New("already exists")
)

// Domain stores. Each domain has its own focused interface with
// unprefixed methods. Set by the host before command execution.
// Nil when the host has no store (e.g. discovery-only mode).
var (
	Documents  DocumentStore
	Tasks      TaskStore
	Links      LinkStore
	Tags       TagStore
	Audits     AuditStore
	Activities ActivityStore
	Mirror     MirrorStore
	Git        GitStore
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
// (e.g. "grep" -> "llmd_grep").
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

// Context carries per-invocation data to commands. It embeds
// [context.Context] for cancellation and timeout propagation, and
// holds request-scoped domain store instances so commands access the
// store through the context rather than package-level globals.
//
// Author is the configured user identity (required for write
// operations). Stdin contains piped input when present, or nil for
// interactive terminals.
type Context struct {
	context.Context

	Author string
	Stdin  []byte
	DBPath string // Override database path (empty = default)

	// Domain stores bound to this request's lifecycle context.
	// Commands should use these instead of the package-level globals.
	Documents  DocumentStore
	Tasks      TaskStore
	Links      LinkStore
	Tags       TagStore
	Audits     AuditStore
	Activities ActivityStore
	Mirror     MirrorStore
	Git        GitStore
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

// Init creates a new store at the given path (or the default path if
// empty). Returns the path of the created store. Set by the host at
// startup.
var Init func(dbPath string) (string, error)

// Dispatch executes a command by name through the host. Set by the host
// at startup. Used by commands that need to invoke other commands (e.g.
// MCP server dispatching tool calls). The context controls cancellation
// and timeout for the dispatched command.
var Dispatch func(ctx context.Context, cmd string, args []string, author string, stdin []byte, dbPath string) (Response, error)

// AllCommands returns all registered commands. Set by the host at startup.
var AllCommands func() map[string]*Command

// PluginNames returns the names of loaded yaegi plugins. Set by the host.
var PluginNames func() []string
