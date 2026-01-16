// links.go implements document relationship operations for the Service layer.
//
// Separated from tags.go because links represent relationships between
// documents (graph edges), not labels on individual documents. Links have
// two endpoints and their own event types.
//
// Design: Links fire events on creation and deletion to enable knowledge
// graph extensions to maintain secondary indices or trigger workflows.

package document

import (
	"context"
	"sync"

	"github.com/jpl-au/llmd/extension"
	"github.com/jpl-au/llmd/internal/store"
)

// Link creates a bidirectional link between two documents, returns the link ID.
func (s *Service) Link(ctx context.Context, from, to, tag string, opts store.LinkOptions) (string, error) {
	opts.MaxPath = s.maxPath
	id, err := s.store.Link(ctx, from, to, tag, opts)
	if err != nil {
		return "", err
	}
	s.fireEvent(extension.LinkEvent{ID: id, FromPath: from, ToPath: to, Tag: tag, Created: true})
	return id, nil
}

// UnlinkByID removes a link by its unique ID (soft delete).
func (s *Service) UnlinkByID(ctx context.Context, id string) error {
	if err := s.store.UnlinkByID(ctx, id); err != nil {
		return err
	}
	s.fireEvent(extension.LinkEvent{ID: id, Created: false})
	return nil
}

// UnlinkByTag removes all links with a specific tag (soft delete).
// Fires LinkEvent for each deleted link to notify extensions.
func (s *Service) UnlinkByTag(ctx context.Context, tag string, opts store.LinkOptions) (int64, error) {
	// Fetch links and delete concurrently - both operations are independent
	var links []store.Link
	var count int64
	var listErr, unlinkErr error

	var wg sync.WaitGroup
	wg.Go(func() {
		links, listErr = s.store.ListLinksByTag(ctx, tag, opts)
	})
	wg.Go(func() {
		count, unlinkErr = s.store.UnlinkByTag(ctx, tag, opts)
	})
	wg.Wait()

	if listErr != nil {
		return 0, listErr
	}
	if unlinkErr != nil {
		return 0, unlinkErr
	}

	// Fire events for each deleted link.
	for _, link := range links {
		s.fireEvent(extension.LinkEvent{
			ID:       link.ID,
			FromPath: link.FromPath,
			ToPath:   link.ToPath,
			Tag:      link.Tag,
			Created:  false,
		})
	}
	return count, nil
}

// ListLinks returns all links for a document.
func (s *Service) ListLinks(ctx context.Context, path, tag string, opts store.LinkOptions) ([]store.Link, error) {
	opts.MaxPath = s.maxPath
	return s.store.ListLinks(ctx, path, tag, opts)
}

// ListLinksByTag returns all links with a specific tag.
func (s *Service) ListLinksByTag(ctx context.Context, tag string, opts store.LinkOptions) ([]store.Link, error) {
	return s.store.ListLinksByTag(ctx, tag, opts)
}

// ListOrphanLinkPaths returns document paths with no links.
func (s *Service) ListOrphanLinkPaths(ctx context.Context, opts store.LinkOptions) ([]string, error) {
	return s.store.ListOrphanLinkPaths(ctx, opts)
}

// DeleteLinksForPath soft-deletes all links involving a document. Enables
// cleanup when documents are removed or reorganised to maintain referential
// integrity in the link graph.
func (s *Service) DeleteLinksForPath(ctx context.Context, path string, opts store.LinkOptions) error {
	return s.store.DeleteLinksForPath(ctx, path, opts)
}
