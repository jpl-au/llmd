// Package rm provides soft-deletion of documents.
//
// Deletion is always soft - documents are marked deleted but remain recoverable
// via restore until vacuum permanently removes them. This safety net prevents
// accidental data loss and enables audit trails.
package rm

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jpl-au/llmd/internal/service"
)

// Options configures a delete operation.
type Options struct {
	Recursive bool // Delete all documents under path
	Version   int  // If > 0, delete only this specific version
}

// Result contains the outcome of a delete operation.
type Result struct {
	Path    string   `json:"path"`
	Version int      `json:"version,omitempty"` // Version deleted (if version-specific)
	Key     string   `json:"key,omitempty"`     // Key deleted (if key-specific)
	Deleted []string `json:"deleted,omitempty"` // Paths of deleted documents (for recursive)
}

// Run soft-deletes a document or documents recursively.
func Run(ctx context.Context, w io.Writer, svc service.Service, path string, opts Options) (Result, error) {
	result := Result{Path: path}

	// Version-specific deletion cannot be combined with recursive
	if opts.Version > 0 && opts.Recursive {
		return result, fmt.Errorf("--version and --recursive cannot be used together")
	}

	// For simple delete (no version, no recursive), try to resolve as path or key
	if opts.Version == 0 && !opts.Recursive && len(path) == 8 {
		// Try path first
		_, err := svc.Latest(ctx, path, false)
		if err != nil {
			// Path not found, try as key
			doc, keyErr := svc.ByKey(ctx, path)
			if keyErr == nil {
				// Found as key - delete that specific version
				if err := svc.DeleteVersion(ctx, doc.Path, doc.Version); err != nil {
					return result, err
				}
				result.Path = doc.Path
				result.Version = doc.Version
				result.Key = path
				result.Deleted = []string{doc.Path}
				fmt.Fprintf(w, "Deleted %s (version %d, key %s)\n", doc.Path, doc.Version, path)
				return result, nil
			}
			// Neither path nor key found - return original path error
			return result, err
		}
		// Path found, continue with normal delete below
	}

	if opts.Version > 0 {
		// Delete a specific version only
		if err := svc.DeleteVersion(ctx, path, opts.Version); err != nil {
			return result, err
		}
		result.Version = opts.Version
		result.Deleted = []string{path}
		fmt.Fprintf(w, "Deleted %s (version %d)\n", path, opts.Version)
	} else if opts.Recursive {
		// List all documents under path
		docs, err := svc.List(ctx, path, false, false)
		if err != nil {
			return result, err
		}

		for _, doc := range docs {
			// Only delete if path starts with the prefix
			if strings.HasPrefix(doc.Path, path) {
				if err := svc.Delete(ctx, doc.Path); err != nil {
					return result, err
				}
				result.Deleted = append(result.Deleted, doc.Path)
				fmt.Fprintf(w, "Deleted %s\n", doc.Path)
			}
		}

		if len(result.Deleted) == 0 {
			fmt.Fprintf(w, "No documents found under %s\n", path)
		}
	} else {
		if err := svc.Delete(ctx, path); err != nil {
			return result, err
		}
		result.Deleted = []string{path}
		fmt.Fprintf(w, "Deleted %s\n", path)
	}

	return result, nil
}
