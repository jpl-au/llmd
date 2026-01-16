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
// p can be a document path or a key.
func (s *Service) Tag(ctx context.Context, p, tag string, opts store.TagOptions) error {
	opts.MaxPath = s.maxPath
	doc, err := s.Resolve(ctx, p, true)
	if err != nil {
		return fmt.Errorf("tag %q: document not found: %w", p, err)
	}
	p = doc.Path // Use resolved path
	if err := s.store.Tag(ctx, p, tag, opts); err != nil {
		return fmt.Errorf("tag %q with %q: %w", p, tag, err)
	}
	s.fireEvent(extension.TagEvent{Path: p, Tag: tag, Source: opts.Source, Added: true})
	return nil
}

// Untag removes a label from a document (soft delete). The tag can be restored
// until vacuum permanently removes it.
// p can be a document path or a key.
func (s *Service) Untag(ctx context.Context, p, tag string, opts store.TagOptions) error {
	opts.MaxPath = s.maxPath
	doc, err := s.Resolve(ctx, p, true)
	if err != nil {
		return fmt.Errorf("untag %q: document not found: %w", p, err)
	}
	p = doc.Path // Use resolved path
	if err := s.store.Untag(ctx, p, tag, opts); err != nil {
		return fmt.Errorf("untag %q from %q: %w", tag, p, err)
	}
	s.fireEvent(extension.TagEvent{Path: p, Tag: tag, Source: opts.Source, Added: false})
	return nil
}

// ListTags returns all unique tags. If a path is provided, returns only tags on
// that document; otherwise returns all tags in the system for discovery/autocomplete.
func (s *Service) ListTags(ctx context.Context, p string, opts store.TagOptions) ([]string, error) {
	if p != "" {
		// We still validate here because ListTags doesn't take MaxPath explicitly in previous implementation,
		// but since we updated TagOptions, we can pass it there.
		opts.MaxPath = s.maxPath
	}
	return s.store.ListTags(ctx, p, opts)
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
		// ListByTag logic modification
		opts.MaxPath = s.maxPath
		// Validate prefix using validate if needed, or rely on store query being safe (parameterized).
		// However, validate.Path(p, 0) was just checking for null bytes etc.
		if _, err := validate.Path(prefix, 0); err != nil {
			return nil, err
		}
	}
	return s.store.ListByTag(ctx, prefix, tag, includeDeleted, deletedOnly, opts)
}
