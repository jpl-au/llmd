// Package tag provides document tagging operations for the CLI layer.
//
// This package orchestrates tag add/remove/list operations, handling
// output formatting. The service layer handles path/key resolution.

package tag

import (
	"context"
	"fmt"
	"io"

	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/store"
)

// Result contains the outcome of a tag operation.
type Result struct {
	Path   string   `json:"path,omitempty"`
	Tag    string   `json:"tag,omitempty"`
	Action string   `json:"action,omitempty"`
	Tags   []string `json:"tags"`
}

// Add adds a tag to a document. Path can be a document path or key.
func Add(ctx context.Context, w io.Writer, svc service.Service, path, tag string) (Result, error) {
	result := Result{Path: path, Tag: tag, Action: "add"}

	if err := svc.Tag(ctx, path, tag, store.NewTagOptions()); err != nil {
		return result, err
	}

	// Get resolved path and current tags for result
	if doc, _, err := svc.Resolve(ctx, path, true); err == nil {
		result.Path = doc.Path
		if tags, err := svc.ListTags(ctx, doc.Path, store.NewTagOptions()); err == nil {
			result.Tags = tags
		}
	}

	fmt.Fprintf(w, "Added tag %q to %s\n", tag, result.Path)
	return result, nil
}

// Remove removes a tag from a document. Path can be a document path or key.
func Remove(ctx context.Context, w io.Writer, svc service.Service, path, tag string) (Result, error) {
	result := Result{Path: path, Tag: tag, Action: "remove"}

	if err := svc.Untag(ctx, path, tag, store.NewTagOptions()); err != nil {
		return result, err
	}

	// Get resolved path and current tags for result
	if doc, _, err := svc.Resolve(ctx, path, true); err == nil {
		result.Path = doc.Path
		if tags, err := svc.ListTags(ctx, doc.Path, store.NewTagOptions()); err == nil {
			result.Tags = tags
		}
	}

	fmt.Fprintf(w, "Removed tag %q from %s\n", tag, result.Path)
	return result, nil
}

// List lists tags for a document or all tags if path is empty.
// Path can be a document path or key.
func List(ctx context.Context, w io.Writer, svc service.Service, path string) (Result, error) {
	result := Result{Path: path}

	tags, err := svc.ListTags(ctx, path, store.NewTagOptions())
	if err != nil {
		return result, err
	}
	result.Tags = tags

	// Get resolved path for result if path was provided
	if path != "" {
		if doc, _, err := svc.Resolve(ctx, path, true); err == nil {
			result.Path = doc.Path
		}
	}

	for _, t := range tags {
		fmt.Fprintln(w, t)
	}
	return result, nil
}
