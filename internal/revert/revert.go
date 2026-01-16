// Package revert provides forward-moving version rollback.
//
// Revert creates a new version with the old content rather than destructively
// moving backwards. This preserves complete history - you can see when a revert
// happened and even revert a revert. Supports targeting by version number or key.
package revert

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/store"
)

// Options configures a revert operation.
type Options struct {
	Author  string // Who is performing the revert
	Message string // Custom message (defaults to "Revert to vN" or "Revert to <key>")
	Key     string // Explicit version key (overrides target interpretation)
}

// Result contains the outcome of a revert operation.
type Result struct {
	Path       string `json:"path"`
	RevertedTo int    `json:"reverted_to"` // Version number reverted to
	NewVersion int    `json:"new_version"` // New version number created
	Key        string `json:"key"`         // Key of the version reverted to
	Author     string `json:"author"`
	Message    string `json:"message"`
}

// Run reverts a document to a previous version by creating a new version
// with the old content. This is forward-moving (preserves history).
//
// The target can be specified as:
//   - A key (8-char identifier): reverts to that specific version
//   - A path + version: reverts to that version of the document
func Run(ctx context.Context, w io.Writer, svc service.Service, target string, version int, opts Options) (Result, error) {
	var doc *store.Document
	var err error
	var result Result
	usedKey := false

	if opts.Key != "" {
		// Explicit key provided via --key flag
		doc, err = svc.ByKey(ctx, opts.Key)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return result, fmt.Errorf("key not found: %s", opts.Key)
			}
			return result, err
		}
		usedKey = true
	} else if version > 0 {
		// Path + version provided
		doc, err = svc.Version(ctx, target, version)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return result, fmt.Errorf("version %d not found for %s", version, target)
			}
			return result, err
		}
	} else {
		// No version - target could be path or key, use Resolve
		var isKey bool
		doc, isKey, err = svc.Resolve(ctx, target, false)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return result, fmt.Errorf("not found: %s", target)
			}
			return result, err
		}
		if isKey {
			usedKey = true
		} else {
			// Found as path but no version specified
			return result, fmt.Errorf("version required: llmd revert %s <version>", target)
		}
	}

	// Build the message
	message := opts.Message
	if message == "" {
		if usedKey {
			message = fmt.Sprintf("Revert to %s", target)
		} else {
			message = fmt.Sprintf("Revert to v%d", doc.Version)
		}
	}

	// Re-check current document state right before write to avoid TOCTOU race.
	// Another process could have deleted the document between our initial fetch
	// and the write operation.
	current, err := svc.Latest(ctx, doc.Path, true)
	if err != nil {
		return result, fmt.Errorf("check current state: %w", err)
	}
	if current.DeletedAt != nil {
		return result, fmt.Errorf("document is deleted (use 'llmd restore %s' first)", doc.Path)
	}

	// Write the old content as a new version
	if err := svc.Write(ctx, doc.Path, doc.Content, opts.Author, message); err != nil {
		return result, fmt.Errorf("write reverted content: %w", err)
	}

	// Get the new version number
	newDoc, err := svc.Latest(ctx, doc.Path, false)
	if err != nil {
		return result, fmt.Errorf("get new version: %w", err)
	}

	result = Result{
		Path:       doc.Path,
		RevertedTo: doc.Version,
		NewVersion: newDoc.Version,
		Key:        doc.Key,
		Author:     opts.Author,
		Message:    message,
	}

	fmt.Fprintf(w, "Reverted %s to v%d (now v%d)\n", doc.Path, doc.Version, newDoc.Version)
	return result, nil
}
