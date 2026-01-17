// tags.go implements document tagging operations for the Service layer.
//
// Separated from write.go because tags have independent lifecycle from
// document content. Tags can be added/removed without creating new document
// versions, and they fire their own events.
//
// Design: Tag operations verify document existence before modifying tags.
// This prevents orphaned tags on non-existent documents, though soft-deleted
// documents can still be tagged (useful for organising trash before vacuum).

package document

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/store"
	"github.com/jpl-au/llmd/internal/validate"
)

// Tag adds a label to a document. Verifies the document exists first to prevent
// orphaned tags. Tags are metadata that persist across document versions.
// path can be a document path or a key.
func (s *Service) Tag(ctx context.Context, path, tag string, opts store.TagOptions) error {
	opts.MaxPath = s.maxPath
	doc, _, err := s.Resolve(ctx, path, true)
	if err != nil {
		return fmt.Errorf("tag %q: document not found: %w", path, err)
	}
	path = doc.Path // Use resolved path
	if err := s.store.Tag(ctx, path, tag, opts); err != nil {
		return fmt.Errorf("tag %q with %q: %w", path, tag, err)
	}
	s.fireEvent(extension.TagEvent{Path: path, Tag: tag, Source: opts.Source, Added: true})
	return nil
}

// Untag removes a label from a document (soft delete). The tag can be restored
// until vacuum permanently removes it.
// path can be a document path or a key.
func (s *Service) Untag(ctx context.Context, path, tag string, opts store.TagOptions) error {
	opts.MaxPath = s.maxPath
	doc, _, err := s.Resolve(ctx, path, true)
	if err != nil {
		return fmt.Errorf("untag %q: document not found: %w", path, err)
	}
	path = doc.Path // Use resolved path
	if err := s.store.Untag(ctx, path, tag, opts); err != nil {
		return fmt.Errorf("untag %q from %q: %w", tag, path, err)
	}
	s.fireEvent(extension.TagEvent{Path: path, Tag: tag, Source: opts.Source, Added: false})
	return nil
}

// ListTags returns all unique tags. If a path is provided, returns only tags on
// that document; otherwise returns all tags in the system for discovery/autocomplete.
// path can be a document path or a key.
func (s *Service) ListTags(ctx context.Context, path string, opts store.TagOptions) ([]string, error) {
	if path != "" {
		opts.MaxPath = s.maxPath
		// Resolve path or key to get actual document path
		doc, _, err := s.Resolve(ctx, path, true)
		if err != nil {
			return nil, fmt.Errorf("list tags %q: document not found: %w", path, err)
		}
		path = doc.Path
	}
	return s.store.ListTags(ctx, path, opts)
}

// PathsWithTag returns all document paths that have a specific tag.
// Enables tag-based navigation and filtering.
func (s *Service) PathsWithTag(ctx context.Context, tag string, opts store.TagOptions) ([]string, error) {
	return s.store.PathsWithTag(ctx, tag, opts)
}

// ListByTag returns documents matching both a path prefix and a tag. Combines
// hierarchical path filtering with label-based filtering for targeted queries.
func (s *Service) ListByTag(ctx context.Context, prefix, tag string, includeDeleted, deletedOnly bool, opts store.TagOptions) ([]store.Document, error) {
	if prefix != "" {
		// Enforce path length limit for prefix filtering.
		opts.MaxPath = s.maxPath
		// Validate prefix structure (null bytes, traversal) before query.
		if _, err := validate.Path(prefix, 0); err != nil {
			return nil, err
		}
	}
	return s.store.ListByTag(ctx, prefix, tag, includeDeleted, deletedOnly, opts)
}
