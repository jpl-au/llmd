// Package log provides centralised audit logging for llmd operations.
// Logs are stored in ~/.llmd/log/llmd-log.db and track all CLI commands
// and MCP tool invocations across projects.
//
// # Fluent API
//
// Use the fluent builder API to construct and write log entries:
//
//	log.Event("document:cat", "read").
//		Author(cmd.Author()).
//		Path(p).
//		Version(doc.Version).
//		Write(err)
//
//	log.Event("search:find", "search").
//		Author(cmd.Author()).
//		Detail("query", query).
//		Detail("count", len(results)).
//		Write(err)
//
// The source parameter follows the format "{extension}:{command}" for CLI
// commands or "mcp:{tool}" for MCP tools. Examples: "document:cat",
// "search:grep", "mcp:write".
package log

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	global *Logger
	mu     sync.Mutex
)

// Entry represents a single log entry.
type Entry struct {
	Source  string         // e.g., "document:cat", "mcp:llmd_read"
	Author  string         // who performed the action
	Action  string         // verb: read, write, delete, etc.
	Path    string         // input: document path requested
	Version int            // input: document version requested

	// Output fields - populated after operation succeeds
	ResolvedPath  string // output: resolved/canonical path (if different from input)
	ResultVersion int    // output: version created or accessed

	// Timing
	Start int64 // unix timestamp when Event() called
	End   int64 // unix timestamp when Write() called

	Success bool           // whether operation succeeded
	Error   string         // error message if failed
	Detail  map[string]any // additional operation-specific data
}

// Builder constructs a log entry using a fluent API.
// Create with [Event], chain methods to set fields, then call [Builder.Write]
// to write the entry.
type Builder struct {
	entry Entry
}

// Event creates a new log entry builder for an operation.
//
// The source identifies where the operation originated:
//   - CLI commands: "{extension}:{command}" (e.g., "document:cat", "search:grep")
//   - MCP tools: "mcp:{tool}" (e.g., "mcp:write", "mcp:search")
//
// The action describes what operation was performed:
//   - "read", "write", "delete", "list", "search", "edit", "move", "restore", etc.
//
// Example:
//
//	log.Event("document:write", "write").
//		Author(cmd.Author()).
//		Path(p).
//		Write(err)
func Event(source, action string) *Builder {
	return &Builder{
		entry: Entry{
			Source: source,
			Action: action,
			Start:  time.Now().Unix(),
		},
	}
}

// Author sets who performed the operation.
//
// For CLI commands, use cmd.Author() which returns the configured author.
// For MCP tools, use "mcp" as the author.
//
// Example:
//
//	log.Event("document:cat", "read").Author(cmd.Author())
func (b *Builder) Author(author string) *Builder {
	b.entry.Author = author
	return b
}

// Path sets the document path this operation affects.
//
// Use for operations that target a specific document or path prefix.
// Leave unset for operations that don't target documents (e.g., config).
//
// Example:
//
//	log.Event("document:cat", "read").Path("docs/readme")
func (b *Builder) Path(path string) *Builder {
	b.entry.Path = path
	return b
}

// Version sets the input document version for this operation.
//
// Use for operations where the user specified a version to access.
//
// Example:
//
//	log.Event("document:cat", "read").Path(p).Version(requestedVersion)
func (b *Builder) Version(version int) *Builder {
	b.entry.Version = version
	return b
}

// Resolved sets the resolved/canonical path (output).
//
// Use when the actual path differs from input, such as when a key
// is resolved to a path, or when path normalization changes the path.
//
// Example:
//
//	l.Resolved(result.Path)  // After confirming success
func (b *Builder) Resolved(path string) *Builder {
	b.entry.ResolvedPath = path
	return b
}

// ResultVersion sets the version that resulted from the operation (output).
//
// For writes: the new version created.
// For reads: the version that was actually accessed.
//
// Example:
//
//	l.ResultVersion(newDoc.Version)  // After confirming success
func (b *Builder) ResultVersion(version int) *Builder {
	b.entry.ResultVersion = version
	return b
}

// Detail adds a key-value pair to the log entry's detail map.
//
// Use for operation-specific data that doesn't fit standard fields:
// search queries, result counts, source/destination paths, etc.
// Can be called multiple times to add multiple details.
//
// Example:
//
//	log.Event("search:find", "search").
//		Detail("query", query).
//		Detail("count", len(results))
func (b *Builder) Detail(key string, value any) *Builder {
	if b.entry.Detail == nil {
		b.entry.Detail = make(map[string]any)
	}
	b.entry.Detail[key] = value
	return b
}

// Write writes the log entry to the database, deriving success/failure from err.
//
// If err is nil, the entry is logged as successful.
// If err is non-nil, the entry is logged as failed with the error message.
//
// This is the standard way to complete a log entry after an operation.
//
// Example:
//
//	doc, err := svc.Latest(path)
//	log.Event("document:cat", "read").Path(path).Write(err)
//	if err != nil {
//		return err
//	}
func (b *Builder) Write(err error) {
	b.entry.End = time.Now().Unix()
	b.entry.Success = err == nil
	if err != nil {
		b.entry.Error = err.Error()
	}
	Log(b.entry)
}

// Open initialises the global logger. Safe to call multiple times.
// Errors are returned but callers may choose to ignore them (best-effort logging).
func Open() error {
	mu.Lock()
	defer mu.Unlock()

	if global != nil {
		return nil
	}

	p := dbPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", p)
	if err != nil {
		return err
	}

	if err := migrate(db); err != nil {
		db.Close()
		return err
	}

	global = &Logger{db: db}
	return nil
}

// SetProject sets the project identifier for subsequent log entries.
// The dir should be the absolute path to the .llmd directory.
func SetProject(dir string) {
	mu.Lock()
	defer mu.Unlock()
	if global != nil {
		global.project = hash(dir)
	}
}

// Log writes an entry. Safe to call if logger not initialised (no-op).
func Log(e Entry) {
	mu.Lock()
	l := global
	mu.Unlock()

	if l == nil {
		return
	}
	l.log(e)
}

// Close closes the global logger.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if global != nil {
		global.db.Close()
		global = nil
	}
}
