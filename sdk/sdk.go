//go:build wasip1

// Package sdk provides the plugin SDK for llmd.
//
// This package is only compiled when targeting wasip1 (WebAssembly System Interface
// Preview 1) as plugins run inside a WebAssembly sandbox. The SDK provides all the
// types and interfaces needed to build llmd plugins.
//
// # Architecture Overview
//
// llmd uses a WebAssembly-based plugin system built on github.com/knqyf263/go-plugin
// and the wazero runtime. Plugins are compiled to WASM modules and loaded at runtime
// by the host. Communication between host and plugins uses Protocol Buffers.
//
// The plugin architecture consists of:
//   - Plugin: The main interface that all plugins must implement
//   - Host API: Functions exposed by the host for plugins to call (document operations, search, etc.)
//   - Commands: CLI/MCP commands that plugins provide
//   - Events: Asynchronous notifications that plugins can subscribe to
//
// # Building Plugins
//
// Plugins must be built with the c-shared build mode to create a "reactor" WASM
// module (one that initialises but doesn't exit, allowing the host to call exports):
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
//
// In c-shared mode, main() is not called, so plugin registration must happen in
// init():
//
//	func init() {
//	    sdk.Register(&MyPlugin{})
//	}
//
//	func main() {
//	    // Required for compilation but not called in c-shared mode
//	}
//
// # Example Plugin
//
// A minimal plugin implementation:
//
//	type MyPlugin struct{}
//
//	func (p *MyPlugin) Manifest() sdk.Manifest {
//	    return sdk.Manifest{
//	        Name:        "myplugin",
//	        Version:     "1.0.0",
//	        Description: "Example plugin",
//	        Commands: []sdk.Command{
//	            {
//	                Name:        "hello",
//	                Description: "Say hello",
//	                MCPEnabled:  true,
//	            },
//	        },
//	    }
//	}
//
//	func (p *MyPlugin) ExecuteCommand(ctx sdk.Context, cmd string, args []string, flags map[string]any) (sdk.Result, error) {
//	    switch cmd {
//	    case "hello":
//	        return sdk.TextResult("Hello, world!"), nil
//	    default:
//	        return nil, sdk.ErrUnknownCommand{Command: cmd}
//	    }
//	}
//
// # Host API
//
// Plugins can access the document store and other host functionality through the
// global Host variable:
//
//	content, err := sdk.Host.Read("docs/readme")
//	if err != nil {
//	    return nil, err
//	}
//
// See the HostAPI interface for available operations.
package sdk

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jpl-au/llmd/proto/plugin"
)

// Plugin is the interface that all llmd plugins must implement.
//
// Every plugin must implement this interface to be loaded by the host. The interface
// provides metadata about the plugin through Manifest() and handles command execution
// through ExecuteCommand().
//
// Plugins may optionally implement additional interfaces:
//   - EventHandler: To receive notifications about document store events
//   - Shutdowner: To perform cleanup when the plugin is unloaded
type Plugin interface {
	// Manifest returns the plugin's metadata and command definitions.
	// This is called once when the plugin is initialised and should return
	// static information about the plugin's capabilities.
	Manifest() Manifest

	// ExecuteCommand executes a command and returns the result.
	// The cmd parameter is the command name as registered in the manifest.
	// The args parameter contains positional arguments passed to the command.
	// The flags parameter contains named flags with their parsed values.
	// Returns a Result (TextResult or JSONResult) on success, or an error on failure.
	ExecuteCommand(ctx Context, cmd string, args []string, flags map[string]any) (Result, error)
}

// EventHandler is an optional interface for plugins that want to handle events.
//
// Plugins that implement this interface will receive notifications when events
// they have subscribed to occur. Event subscriptions are declared in the Manifest's
// SubscribedEvents field.
//
// Events are delivered asynchronously and should be handled quickly to avoid
// blocking other operations.
type EventHandler interface {
	// HandleEvent is called when a subscribed event occurs.
	// The event contains details about what occurred (document written, tag added, etc.).
	// Returning an error will log the failure but won't affect other event handlers.
	HandleEvent(event Event) error
}

// Shutdowner is an optional interface for plugins that need cleanup.
//
// Plugins that allocate resources (file handles, network connections, goroutines)
// should implement this interface to ensure proper cleanup when the host shuts down
// or the plugin is unloaded.
type Shutdowner interface {
	// Shutdown is called when the plugin is being unloaded.
	// This is the plugin's opportunity to release resources and perform cleanup.
	// The host will wait for this method to return before unloading the plugin.
	Shutdown() error
}

// Manifest describes a plugin and its capabilities.
//
// The Manifest is returned by Plugin.Manifest() and provides all the information
// the host needs to integrate the plugin. This includes metadata (name, version),
// the commands the plugin provides, and which events it wants to receive.
type Manifest struct {
	// Name is the unique identifier for this plugin.
	// It should be lowercase, alphanumeric with hyphens (e.g., "my-plugin").
	Name string `json:"name"`

	// Version is the plugin version in semantic versioning format (e.g., "1.0.0").
	Version string `json:"version"`

	// Author is the plugin author's name or organisation (optional).
	Author string `json:"author,omitempty"`

	// Description is a brief summary of what the plugin does (optional).
	Description string `json:"description,omitempty"`

	// MinHostVersion is the minimum llmd host version required (optional).
	// If specified, the host will refuse to load the plugin if incompatible.
	MinHostVersion string `json:"min_host_version,omitempty"`

	// Commands lists all CLI/MCP commands this plugin provides.
	Commands []Command `json:"commands"`

	// SubscribedEvents lists event types this plugin wants to receive.
	// Use the Event* constants (e.g., EventDocumentWritten) to subscribe.
	SubscribedEvents []string `json:"subscribed_events,omitempty"`
}

// Command defines a CLI/MCP command provided by a plugin.
//
// Commands can be invoked from the command line (llmd <command>) or via MCP
// (Model Context Protocol) when MCPEnabled is true. Each command has a name,
// optional flags, and a description for help text.
type Command struct {
	// Name is the command name as typed on the command line (e.g., "cat", "ls").
	// Should be lowercase, short, and memorable.
	Name string `json:"name"`

	// Description is a one-line summary shown in help text.
	Description string `json:"description"`

	// Usage shows the command syntax (e.g., "cat <path>").
	// Optional but recommended for commands with arguments.
	Usage string `json:"usage,omitempty"`

	// Flags lists optional flags the command accepts.
	Flags []Flag `json:"flags,omitempty"`

	// MCPEnabled controls whether this command is exposed as an MCP tool.
	// Set to true to allow AI assistants to invoke this command.
	MCPEnabled bool `json:"mcp_enabled"`

	// MCPName overrides the MCP tool name (defaults to the command name).
	// Use this when the command name conflicts with existing MCP tools.
	MCPName string `json:"mcp_name"`
}

// Flag defines a command flag.
//
// Flags provide optional parameters to commands. They can be specified with
// either the long form (--name) or short form (-n if defined).
type Flag struct {
	// Name is the flag name without dashes (e.g., "version" for --version).
	Name string `json:"name"`

	// Short is the single-character shorthand (e.g., "v" for -v).
	// Optional but recommended for commonly-used flags.
	Short string `json:"short,omitempty"`

	// Type specifies the flag value type: "string", "int", "bool", or "stringSlice".
	// The type determines how the flag value is parsed.
	Type string `json:"type"`

	// Default is the default value if the flag is not specified.
	// For bool flags, omit this to default to false.
	Default string `json:"default,omitempty"`

	// Description is shown in help text for this flag.
	Description string `json:"description"`

	// Required marks this flag as mandatory.
	// If true, the command will fail if the flag is not provided.
	Required bool `json:"required,omitempty"`
}

// Context provides execution context to commands.
//
// The Context is passed to ExecuteCommand and provides information about how
// and by whom the command was invoked. Commands can use this to customise their
// behaviour (e.g., different output for CLI vs MCP).
type Context struct {
	// Interface indicates how the command was invoked (CLI, MCP, or API).
	// Commands may behave differently depending on the interface.
	Interface Interface

	// Author identifies who is executing the command (username or service name).
	// This is recorded in version history for auditing.
	Author string

	// Format indicates the preferred output format.
	// Commands should respect this when returning results.
	Format OutputFormat

	// Env contains environment variables passed to the command.
	// Only populated if the host allows environment access for this plugin.
	Env map[string]string
}

// Interface indicates how the command was invoked.
type Interface int

const (
	// InterfaceCLI indicates the command was invoked from the command line.
	InterfaceCLI Interface = iota

	// InterfaceMCP indicates the command was invoked via Model Context Protocol.
	InterfaceMCP

	// InterfaceAPI indicates the command was invoked via the HTTP/gRPC API.
	InterfaceAPI
)

// OutputFormat indicates the preferred output format.
type OutputFormat int

const (
	// FormatText indicates plain text output is preferred.
	FormatText OutputFormat = iota

	// FormatJSON indicates structured JSON output is preferred.
	FormatJSON

	// FormatTable indicates tabular output is preferred.
	FormatTable
)

// Result is the result of a command execution.
//
// Commands return either TextResult for plain text output or JSONResult for
// structured data. The host converts the result to the appropriate format
// based on the context (CLI output, MCP response, etc.).
type Result interface {
	isResult()
}

// TextResult represents plain text output from a command.
//
// Use TextResult when the output is human-readable text that doesn't need
// structured parsing. This is appropriate for most CLI-style commands.
//
// Example:
//
//	return sdk.TextResult("Document created successfully"), nil
type TextResult string

func (TextResult) isResult() {}

// JSONResult represents structured JSON output from a command.
//
// Use JSONResult when the output is structured data that callers may want to
// parse programmatically. This is particularly useful for MCP commands where
// AI assistants need to process the results.
//
// Example:
//
//	return sdk.JSONResult{Data: map[string]any{
//	    "path":    doc.Path,
//	    "version": doc.Version,
//	}}, nil
type JSONResult struct {
	// Data is the structured data to return.
	// This will be serialised as JSON.
	Data any `json:"data"`
}

func (JSONResult) isResult() {}

// Event represents a document store event.
//
// Events are delivered to plugins that implement EventHandler and have subscribed
// to the event type in their Manifest. Events provide a way for plugins to react
// to changes in the document store.
type Event struct {
	// Type identifies the event (e.g., "document.written").
	// Use the Event* constants to compare event types.
	Type string `json:"type"`

	// Path is the document path that triggered the event.
	Path string `json:"path"`

	// Version is the document version after the event (for document events).
	Version int `json:"version,omitempty"`

	// Author is the user or service that caused the event.
	Author string `json:"author"`

	// Timestamp is the Unix timestamp (nanoseconds) when the event occurred.
	Timestamp int64 `json:"timestamp"`

	// Metadata contains additional event-specific information.
	// The contents vary by event type.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Common event types for subscription.
//
// Use these constants when declaring SubscribedEvents in your Manifest and when
// comparing Event.Type in your EventHandler.
const (
	// EventDocumentWritten fires when a document is created or updated.
	EventDocumentWritten = "document.written"

	// EventDocumentDeleted fires when a document is soft-deleted.
	EventDocumentDeleted = "document.deleted"

	// EventDocumentMoved fires when a document is moved to a new path.
	EventDocumentMoved = "document.moved"

	// EventDocumentRestored fires when a soft-deleted document is restored.
	EventDocumentRestored = "document.restored"

	// EventTagAdded fires when a tag is added to a document.
	EventTagAdded = "tag.added"

	// EventTagRemoved fires when a tag is removed from a document.
	EventTagRemoved = "tag.removed"

	// EventLinkAdded fires when a link is created between documents.
	EventLinkAdded = "link.added"

	// EventLinkRemoved fires when a link is removed between documents.
	EventLinkRemoved = "link.removed"
)

// ErrUnknownCommand is returned when a plugin receives an unknown command.
//
// Return this error from ExecuteCommand when the command name doesn't match
// any commands declared in the plugin's Manifest. This indicates a bug in
// either the plugin or the host's command routing.
type ErrUnknownCommand struct {
	Command string
}

func (e ErrUnknownCommand) Error() string {
	return "unknown command: " + e.Command
}

// Register registers a plugin with the host.
//
// This function must be called from init(), not main(), because plugins are
// built with -buildmode=c-shared which creates a "reactor" WASM module where
// main() is not executed.
//
// Example:
//
//	func init() {
//	    sdk.Register(&MyPlugin{})
//	}
//
//	func main() {
//	    // Required for compilation but not called
//	}
func Register(p Plugin) {
	plugin.RegisterPlugin(&pluginAdapter{p: p})
}

// pluginAdapter adapts the SDK Plugin interface to the proto-generated Plugin interface.
// This is an internal type that bridges the user-friendly SDK types to the wire format.
type pluginAdapter struct {
	p Plugin
}

// Init implements plugin.Plugin.
// Called by the host after loading to retrieve the plugin's manifest.
func (a *pluginAdapter) Init(_ context.Context, _ *plugin.InitRequest) (*plugin.Manifest, error) {
	m := a.p.Manifest()
	return manifestToProto(m), nil
}

// ExecuteCommand implements plugin.Plugin.
// Converts the proto request to SDK types, executes the command, and converts the result back.
func (a *pluginAdapter) ExecuteCommand(ctx context.Context, req *plugin.CommandRequest) (*plugin.CommandResponse, error) {
	// Convert proto request to SDK types
	sdkCtx := Context{
		Interface: Interface(req.GetContext().GetInterface()),
		Author:    req.GetContext().GetAuthor(),
		Format:    OutputFormat(req.GetContext().GetFormat()),
		Env:       req.GetContext().GetEnv(),
	}

	// Parse flags from JSON
	flags := make(map[string]any)
	for k, v := range req.GetFlags() {
		flags[k] = v
	}

	result, err := a.p.ExecuteCommand(sdkCtx, req.GetCommand(), req.GetArgs(), flags)
	if err != nil {
		return &plugin.CommandResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	resp := &plugin.CommandResponse{Success: true}
	switch r := result.(type) {
	case TextResult:
		resp.Output = string(r)
		resp.Format = plugin.OutputFormat_FORMAT_TEXT
	case JSONResult:
		data, _ := json.Marshal(r.Data)
		resp.Output = string(data)
		resp.Format = plugin.OutputFormat_FORMAT_JSON
	}

	return resp, nil
}

// HandleEvent implements plugin.Plugin.
// Forwards events to the plugin's EventHandler if implemented.
func (a *pluginAdapter) HandleEvent(ctx context.Context, e *plugin.Event) (*plugin.Empty, error) {
	handler, ok := a.p.(EventHandler)
	if !ok {
		return &plugin.Empty{}, nil
	}

	// Convert proto event to SDK type
	metadata := make(map[string]any)
	for k, v := range e.GetMetadata() {
		metadata[k] = v
	}

	sdkEvent := Event{
		Type:      e.GetType(),
		Path:      e.GetPath(),
		Version:   int(e.GetVersion()),
		Author:    e.GetAuthor(),
		Timestamp: e.GetTimestamp(),
		Metadata:  metadata,
	}

	if err := handler.HandleEvent(sdkEvent); err != nil {
		return nil, err
	}
	return &plugin.Empty{}, nil
}

// Shutdown implements plugin.Plugin.
// Calls the plugin's Shutdown method if it implements Shutdowner.
func (a *pluginAdapter) Shutdown(_ context.Context, _ *plugin.Empty) (*plugin.Empty, error) {
	if s, ok := a.p.(Shutdowner); ok {
		if err := s.Shutdown(); err != nil {
			return nil, err
		}
	}
	return &plugin.Empty{}, nil
}

// manifestToProto converts an SDK Manifest to the proto-generated Manifest type.
// This is called during plugin initialisation to send the manifest to the host.
func manifestToProto(m Manifest) *plugin.Manifest {
	commands := make([]*plugin.Command, len(m.Commands))
	for i, cmd := range m.Commands {
		flags := make([]*plugin.Flag, len(cmd.Flags))
		for j, f := range cmd.Flags {
			flags[j] = &plugin.Flag{
				Name:        f.Name,
				Short:       f.Short,
				Type:        f.Type,
				Default:     f.Default,
				Description: f.Description,
				Required:    f.Required,
			}
		}
		commands[i] = &plugin.Command{
			Name:        cmd.Name,
			Description: cmd.Description,
			Usage:       cmd.Usage,
			Flags:       flags,
			McpEnabled:  cmd.MCPEnabled,
			McpName:     cmd.MCPName,
		}
	}

	return &plugin.Manifest{
		Name:             m.Name,
		Version:          m.Version,
		Author:           m.Author,
		Description:      m.Description,
		MinHostVersion:   m.MinHostVersion,
		Commands:         commands,
		SubscribedEvents: m.SubscribedEvents,
	}
}

// Host provides access to llmd host operations from within a plugin.
//
// This variable is initialised automatically when the plugin loads. It provides
// the bridge between the plugin's WASM sandbox and the host's document store.
//
// All operations go through the host, ensuring proper access control, auditing,
// and consistency. Plugins cannot directly access the filesystem or database.
var Host HostAPI

// HostAPI defines the operations available to plugins.
//
// This interface provides access to the llmd document store and related
// functionality. All methods are safe to call from any goroutine.
//
// Document paths use forward slashes and should not start with a slash
// (e.g., "notes/todo" not "/notes/todo").
type HostAPI interface {
	// Read retrieves a document's content by path.
	// Returns ErrNotFound if the document doesn't exist.
	Read(path string) ([]byte, error)

	// Write creates or updates a document.
	// The author is recorded in version history.
	// The message is an optional commit message describing the change.
	Write(path string, content []byte, author, message string) error

	// Delete soft-deletes a document.
	// Soft-deleted documents can be restored with Restore.
	// The author is recorded in version history.
	Delete(path string, author string) error

	// List returns document paths matching the prefix.
	// An empty prefix returns all documents.
	// Paths are returned in lexicographical order.
	List(prefix string) ([]string, error)

	// Search performs a full-text search across all documents.
	// The query uses the search engine's query syntax.
	// Results are ordered by relevance score.
	Search(query string) ([]SearchResult, error)

	// Grep searches documents using a regular expression pattern.
	// Returns matching lines with their locations.
	Grep(pattern string) ([]GrepResult, error)
}

// SearchResult represents a full-text search result.
type SearchResult struct {
	// Path is the document path that matched.
	Path string

	// Snippet is a text excerpt showing the match in context.
	Snippet string

	// Score indicates relevance (higher is more relevant).
	Score float32
}

// GrepResult represents a regular expression search result.
type GrepResult struct {
	// Path is the document path containing the match.
	Path string

	// Line is the 1-based line number of the match.
	Line int

	// Content is the full line that matched.
	Content string
}

// SplitCommand splits a command string that may include subcommands.
//
// This is useful for plugins that implement command hierarchies (e.g., "tag add",
// "tag remove"). The command string is split on whitespace.
//
// Example:
//
//	parts := sdk.SplitCommand("tag add")
//	// parts = ["tag", "add"]
func SplitCommand(cmd string) []string {
	return strings.Fields(cmd)
}
