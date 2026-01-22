//go:build wasip1

// This file implements the host API client for plugins.
//
// When a plugin is loaded, the host exports functions that the plugin can call
// to interact with the document store. This file provides the client-side
// implementation that calls those host functions via the proto-generated code.
//
// The Host variable is initialised in init() and is available for use as soon
// as the plugin's init() functions complete.

package sdk

import (
	"context"

	hostpb "github.com/jpl-au/llmd/proto/host"
)

// Host provides access to llmd host operations from within a plugin.
//
// This variable is initialised automatically when the plugin loads.
// All operations go through the host, ensuring proper access control,
// auditing, and consistency.
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
	Read(path string) ([]byte, error)

	// Write creates or updates a document.
	Write(path string, content []byte, author, message string) error

	// Edit performs a search/replace on a document.
	Edit(path, old, new, author, message string) error

	// Delete soft-deletes a document.
	Delete(path string, author string) error

	// List returns document paths matching the prefix.
	List(prefix string) ([]string, error)

	// Search performs a full-text search across all documents.
	Search(query string) ([]SearchResult, error)

	// Grep searches documents using a regular expression pattern.
	Grep(pattern string) ([]GrepResult, error)
}

// SearchResult represents a full-text search result.
type SearchResult struct {
	Path    string
	Snippet string
	Score   float32
}

// GrepResult represents a regular expression search result.
type GrepResult struct {
	Path    string
	Line    int
	Content string
}

// hostAPIImpl implements HostAPI using the proto-generated host client.
//
// This type wraps the proto-generated client and provides the higher-level
// SDK interface. Each method converts SDK types to proto types, calls the
// host function, and converts the response back.
type hostAPIImpl struct {
	client hostpb.Host
}

// init initialises the Host variable with a client that calls host functions.
// This runs automatically when the plugin module is instantiated.
func init() {
	Host = &hostAPIImpl{
		client: hostpb.NewHost(),
	}
}

// Read retrieves a document's content from the host's document store.
// Returns the raw content bytes, or an error if the document doesn't exist.
func (h *hostAPIImpl) Read(path string) ([]byte, error) {
	doc, err := h.client.DocumentRead(context.Background(), &hostpb.ReadRequest{
		Path: path,
	})
	if err != nil {
		return nil, err
	}
	return doc.Content, nil
}

// Write creates or updates a document in the host's document store.
// The author and message are recorded in the document's version history.
// If the document doesn't exist, it is created. If it exists, a new version is written.
func (h *hostAPIImpl) Write(path string, content []byte, author, message string) error {
	_, err := h.client.DocumentWrite(context.Background(), &hostpb.WriteRequest{
		Path:    path,
		Content: content,
		Author:  author,
		Message: message,
	})
	return err
}

// Edit performs a search/replace on a document.
// The old text must exist in the document. A new version is created with the replacement.
func (h *hostAPIImpl) Edit(path, old, new, author, message string) error {
	_, err := h.client.DocumentEdit(context.Background(), &hostpb.EditRequest{
		Path:    path,
		OldText: old,
		NewText: new,
		Author:  author,
		Message: message,
	})
	return err
}

// Delete soft-deletes a document from the host's document store.
// Soft-deleted documents can be restored later. The deletion is recorded
// in the version history with the specified author.
func (h *hostAPIImpl) Delete(path string, author string) error {
	_, err := h.client.DocumentDelete(context.Background(), &hostpb.DeleteRequest{
		Path:   path,
		Author: author,
	})
	return err
}

// List returns the paths of all documents matching the given prefix.
// An empty prefix returns all documents. Paths are returned in lexicographical order.
func (h *hostAPIImpl) List(prefix string) ([]string, error) {
	resp, err := h.client.DocumentList(context.Background(), &hostpb.ListRequest{
		Prefix: prefix,
	})
	if err != nil {
		return nil, err
	}
	paths := make([]string, len(resp.Documents))
	for i, doc := range resp.Documents {
		paths[i] = doc.Path
	}
	return paths, nil
}

// Search performs a full-text search across all documents in the store.
// Returns matching documents with snippets showing the match context.
// Results are ordered by relevance score (highest first).
func (h *hostAPIImpl) Search(query string) ([]SearchResult, error) {
	resp, err := h.client.SearchFullText(context.Background(), &hostpb.SearchRequest{
		Query: query,
	})
	if err != nil {
		return nil, err
	}
	results := make([]SearchResult, len(resp.Matches))
	for i, m := range resp.Matches {
		results[i] = SearchResult{
			Path:    m.Path,
			Snippet: m.Snippet,
			Score:   m.Score,
		}
	}
	return results, nil
}

// Grep searches all documents using a regular expression pattern.
// Returns matching lines with their path and line number.
// The pattern uses Go's regexp syntax.
func (h *hostAPIImpl) Grep(pattern string) ([]GrepResult, error) {
	resp, err := h.client.SearchRegex(context.Background(), &hostpb.GrepRequest{
		Pattern: pattern,
	})
	if err != nil {
		return nil, err
	}
	results := make([]GrepResult, len(resp.Matches))
	for i, m := range resp.Matches {
		results[i] = GrepResult{
			Path:    m.Path,
			Line:    int(m.Line),
			Content: m.Content,
		}
	}
	return results, nil
}
