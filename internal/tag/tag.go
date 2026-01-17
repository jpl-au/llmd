// Package tag provides document tagging operations for the CLI layer.
//
// This package orchestrates tag add/remove/list operations, handling both
// the service calls and output formatting. Tags provide a flexible labelling
// system for document organisation and filtering.

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

// Add adds a tag to a document.
func Add(ctx context.Context, w io.Writer, svc service.Service, path, tag string) (Result, error) {
	result := Result{Path: path, Tag: tag, Action: "add"}

	// Resolve path or key to get actual document path
	doc, _, err := svc.Resolve(ctx, path, true)
	if err != nil {
		return result, err
	}
	path = doc.Path
	result.Path = path

	if err := svc.Tag(ctx, path, tag, store.NewTagOptions()); err != nil {
		return result, err
	}

	if tags, err := svc.ListTags(ctx, path, store.NewTagOptions()); err == nil {
		result.Tags = tags
	}

	fmt.Fprintf(w, "Added tag %q to %s\n", tag, path)
	return result, nil
}

// Remove removes a tag from a document.
func Remove(ctx context.Context, w io.Writer, svc service.Service, path, tag string) (Result, error) {
	result := Result{Path: path, Tag: tag, Action: "remove"}

	// Resolve path or key to get actual document path
	doc, _, err := svc.Resolve(ctx, path, true)
	if err != nil {
		return result, err
	}
	path = doc.Path
	result.Path = path

	if err := svc.Untag(ctx, path, tag, store.NewTagOptions()); err != nil {
		return result, err
	}

	if tags, err := svc.ListTags(ctx, path, store.NewTagOptions()); err == nil {
		result.Tags = tags
	}

	fmt.Fprintf(w, "Removed tag %q from %s\n", tag, path)
	return result, nil
}

// List lists tags for a document or all tags if path is empty.
func List(ctx context.Context, w io.Writer, svc service.Service, path string) (Result, error) {
	result := Result{Path: path}

	// Resolve path or key to get actual document path (if path provided)
	if path != "" {
		doc, _, err := svc.Resolve(ctx, path, true)
		if err != nil {
			return result, err
		}
		path = doc.Path
		result.Path = path
	}

	tags, err := svc.ListTags(ctx, path, store.NewTagOptions())
	if err != nil {
		return result, err
	}
	result.Tags = tags

	for _, t := range tags {
		fmt.Fprintln(w, t)
	}
	return result, nil
}
