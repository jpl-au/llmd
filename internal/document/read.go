// read.go implements document retrieval operations for the Service layer.
//
// Separated from service.go to isolate read-only operations. The Service
// layer adds path normalisation and glob expansion on top of the raw store
// operations, ensuring consistent path handling across all entry points.
//
// Design: All paths are normalised before reaching the store. This prevents
// "docs/readme" and "docs//readme" from being treated as different documents.

package document

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jpl-au/llmd/internal/diff"
	"github.com/jpl-au/llmd/internal/glob"
	"github.com/jpl-au/llmd/internal/store"
)

// Latest retrieves the latest version of a document.
func (s *Service) Latest(ctx context.Context, p string, includeDeleted bool) (*store.Document, error) {
	p, err := s.normalizePath(p)
	if err != nil {
		return nil, err
	}
	return s.store.Latest(ctx, p, includeDeleted)
}

// Version retrieves a specific version of a document.
func (s *Service) Version(ctx context.Context, p string, ver int) (*store.Document, error) {
	p, err := s.normalizePath(p)
	if err != nil {
		return nil, err
	}
	return s.store.Version(ctx, p, ver)
}

// ByKey retrieves a document by its unique 8-char key.
func (s *Service) ByKey(ctx context.Context, key string) (*store.Document, error) {
	return s.store.ByKey(ctx, key)
}

// Resolve returns a document by path or key. Designed for user-facing entry
// points such as CLI commands and MCP tools where input could be either type.
//
// Users see keys in llmd ls output and naturally want to use them with other
// commands. However, an 8-character string like "my-notes" could be either a
// valid path or a key. We resolve this ambiguity by checking both, with path
// taking precedence. If you created a document at that path, you probably mean
// the path rather than some random key that happens to match.
//
// SQLite in WAL mode supports concurrent reads, so we run both lookups in
// parallel rather than sequentially. This halves latency for the ambiguous
// 8-character case.
//
// If input resolves as a key, you get that specific version, which may not be
// the latest. If it resolves as a path, you get the latest version. This
// matches user intent since a key is a precise reference to a specific version.
func (s *Service) Resolve(ctx context.Context, pathOrKey string, includeDeleted bool) (*store.Document, error) {
	// Keys are always exactly 8 characters. Longer or shorter inputs can only
	// be paths.
	if len(pathOrKey) != 8 {
		return s.Latest(ctx, pathOrKey, includeDeleted)
	}

	// For 8-character inputs, check path and key concurrently.
	var pathDoc, keyDoc *store.Document
	var pathErr, keyErr error

	var wg sync.WaitGroup
	wg.Go(func() {
		pathDoc, pathErr = s.Latest(ctx, pathOrKey, includeDeleted)
	})
	wg.Go(func() {
		keyDoc, keyErr = s.ByKey(ctx, pathOrKey)
	})
	wg.Wait()

	// Path takes precedence. If someone created a document at "my-notes", they
	// mean that path rather than a key that happens to match.
	if pathErr == nil {
		return pathDoc, nil
	}
	if keyErr == nil {
		return keyDoc, nil
	}
	// Both failed. Return path error since that is more intuitive for users.
	return nil, pathErr
}

// List returns documents matching a prefix.
func (s *Service) List(ctx context.Context, prefix string, includeDeleted, deletedOnly bool) ([]store.Document, error) {
	prefix, err := s.normalizePrefix(prefix)
	if err != nil {
		return nil, err
	}
	return s.store.List(ctx, prefix, includeDeleted, deletedOnly)
}

// History returns version history for a document.
func (s *Service) History(ctx context.Context, p string, limit int, includeDeleted bool) ([]store.Document, error) {
	p, err := s.normalizePath(p)
	if err != nil {
		return nil, err
	}
	return s.store.History(ctx, p, limit, includeDeleted)
}

// Exists checks if a document exists without fetching content.
func (s *Service) Exists(ctx context.Context, p string) (bool, error) {
	p, err := s.normalizePath(p)
	if err != nil {
		return false, err
	}
	return s.store.Exists(ctx, p)
}

// Count returns the number of documents matching a path prefix.
func (s *Service) Count(ctx context.Context, prefix string) (int64, error) {
	prefix, err := s.normalizePrefix(prefix)
	if err != nil {
		return 0, err
	}
	return s.store.Count(ctx, prefix)
}

// Meta returns document metadata without content.
func (s *Service) Meta(ctx context.Context, p string) (*store.DocumentMeta, error) {
	p, err := s.normalizePath(p)
	if err != nil {
		return nil, err
	}
	return s.store.Meta(ctx, p)
}

// Glob returns document paths matching a glob pattern.
func (s *Service) Glob(ctx context.Context, pattern string) ([]string, error) {
	all, err := s.store.ListPaths(ctx, "")
	if err != nil {
		return nil, err
	}

	if pattern == "" {
		return all, nil
	}

	var paths []string
	for _, p := range all {
		if glob.Match(pattern, p) {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// Diff compares two versions of a document or two documents.
func (s *Service) Diff(ctx context.Context, p string, opts diff.Options) (diff.Result, error) {
	// o = old content, n = new content, ol = old label, nl = new label
	var o, n, ol, nl string
	var err error

	switch {
	case opts.FileContent != "":
		o, n, ol, nl, err = s.diffWithFile(ctx, p, opts)
	case opts.Path2 != "":
		o, n, ol, nl, err = s.diffTwoPaths(ctx, p, opts)
	case opts.Version1 > 0 && opts.Version2 > 0:
		o, n, ol, nl, err = s.diffVersions(ctx, p, opts)
	default:
		o, n, ol, nl, err = s.diffPrevious(ctx, p, opts)
	}

	if err != nil {
		return diff.Result{}, err
	}
	return diff.Compute(o, n, ol, nl), nil
}

func (s *Service) diffWithFile(ctx context.Context, p string, opts diff.Options) (o, n, ol, nl string, err error) {
	np2, err := s.normalizePath(opts.Path2)
	if err != nil {
		return "", "", "", "", err
	}
	doc, err := s.store.Latest(ctx, np2, opts.IncludeDeleted)
	if err != nil {
		return "", "", "", "", fmt.Errorf("reading %s: %w", np2, err)
	}
	return opts.FileContent, doc.Content, p, np2 + " (v" + strconv.Itoa(doc.Version) + ")", nil
}

func (s *Service) diffTwoPaths(ctx context.Context, p string, opts diff.Options) (o, n, ol, nl string, err error) {
	np1, err := s.normalizePath(p)
	if err != nil {
		return "", "", "", "", err
	}
	np2, err := s.normalizePath(opts.Path2)
	if err != nil {
		return "", "", "", "", err
	}

	// Fetch both documents concurrently
	var d1, d2 *store.Document
	var err1, err2 error

	var wg sync.WaitGroup
	wg.Go(func() {
		d1, err1 = s.store.Latest(ctx, np1, opts.IncludeDeleted)
	})
	wg.Go(func() {
		d2, err2 = s.store.Latest(ctx, np2, opts.IncludeDeleted)
	})
	wg.Wait()

	if err1 != nil {
		return "", "", "", "", fmt.Errorf("reading %s: %w", np1, err1)
	}
	if err2 != nil {
		return "", "", "", "", fmt.Errorf("reading %s: %w", np2, err2)
	}
	return d1.Content, d2.Content,
		np1 + " (v" + strconv.Itoa(d1.Version) + ")",
		np2 + " (v" + strconv.Itoa(d2.Version) + ")", nil
}

func (s *Service) diffVersions(ctx context.Context, p string, opts diff.Options) (o, n, ol, nl string, err error) {
	np, err := s.normalizePath(p)
	if err != nil {
		return "", "", "", "", err
	}

	// Fetch both versions concurrently
	var d1, d2 *store.Document
	var err1, err2 error

	var wg sync.WaitGroup
	wg.Go(func() {
		d1, err1 = s.store.Version(ctx, np, opts.Version1)
	})
	wg.Go(func() {
		d2, err2 = s.store.Version(ctx, np, opts.Version2)
	})
	wg.Wait()

	if err1 != nil {
		return "", "", "", "", fmt.Errorf("reading %s v%d: %w", np, opts.Version1, err1)
	}
	if err2 != nil {
		return "", "", "", "", fmt.Errorf("reading %s v%d: %w", np, opts.Version2, err2)
	}
	return d1.Content, d2.Content,
		np + " v" + strconv.Itoa(opts.Version1),
		np + " v" + strconv.Itoa(opts.Version2), nil
}

func (s *Service) diffPrevious(ctx context.Context, p string, opts diff.Options) (o, n, ol, nl string, err error) {
	np, err := s.normalizePath(p)
	if err != nil {
		return "", "", "", "", err
	}
	docs, err := s.store.History(ctx, np, 2, opts.IncludeDeleted)
	if err != nil {
		return "", "", "", "", err
	}
	if len(docs) < 2 {
		return "", "", "", "", fmt.Errorf("only one version exists for %s", np)
	}
	return docs[1].Content, docs[0].Content,
		np + " v" + strconv.Itoa(docs[1].Version),
		np + " v" + strconv.Itoa(docs[0].Version), nil
}

// ListPaths returns document paths without loading content. Enables efficient
// enumeration for glob matching and directory listings where content isn't needed.
func (s *Service) ListPaths(ctx context.Context, prefix string) ([]string, error) {
	prefix, err := s.normalizePrefix(prefix)
	if err != nil {
		return nil, err
	}
	return s.store.ListPaths(ctx, prefix)
}

// ListDeletedPaths returns paths of soft-deleted documents. Enables trash
// management and vacuum preview without loading document content.
func (s *Service) ListDeletedPaths(ctx context.Context, prefix string) ([]string, error) {
	prefix, err := s.normalizePrefix(prefix)
	if err != nil {
		return nil, err
	}
	return s.store.ListDeletedPaths(ctx, prefix)
}

// ListMeta returns metadata for multiple documents matching a prefix. Enables
// efficient batch queries for listings that need size/version info without
// loading full document content.
func (s *Service) ListMeta(ctx context.Context, prefix string, includeDeleted bool) ([]store.DocumentMeta, error) {
	prefix, err := s.normalizePrefix(prefix)
	if err != nil {
		return nil, err
	}
	return s.store.ListMeta(ctx, prefix, includeDeleted)
}

// CountDeleted returns the count of soft-deleted documents. Enables vacuum
// preview and trash management without loading document data.
func (s *Service) CountDeleted(ctx context.Context, prefix string) (int64, error) {
	prefix, err := s.normalizePrefix(prefix)
	if err != nil {
		return 0, err
	}
	return s.store.CountDeleted(ctx, prefix)
}

// DeletedBefore returns paths of documents deleted before the given time.
// Enables targeted vacuum operations that preserve recently deleted items.
func (s *Service) DeletedBefore(ctx context.Context, t time.Time, prefix string) ([]string, error) {
	prefix, err := s.normalizePrefix(prefix)
	if err != nil {
		return nil, err
	}
	return s.store.DeletedBefore(ctx, t, prefix)
}

// VersionCount returns the number of versions for a document without loading
// full history. Enables version management decisions and display.
func (s *Service) VersionCount(ctx context.Context, p string) (int, error) {
	p, err := s.normalizePath(p)
	if err != nil {
		return 0, err
	}
	return s.store.VersionCount(ctx, p)
}

// ListAuthors returns all distinct authors who have written documents.
// Enables author-based filtering and audit reporting.
func (s *Service) ListAuthors(ctx context.Context) ([]string, error) {
	return s.store.ListAuthors(ctx)
}

// Stats returns aggregate database statistics. Provides operational visibility
// for capacity planning and monitoring dashboards.
func (s *Service) Stats(ctx context.Context) (*store.Stats, error) {
	return s.store.Stats(ctx)
}
