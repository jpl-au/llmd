// search.go implements full-text search operations for the Service layer.
//
// Separated from read.go because FTS5 queries have different semantics than
// exact path lookups. This file handles search-specific concerns like query
// parsing and result ranking.
//
// Design: Search prefix paths are normalised but queries are passed through
// unchanged to leverage FTS5's native query syntax (AND, OR, prefix*, "phrases").

package document

import (
	"context"

	"github.com/jpl-au/llmd/internal/path"
	"github.com/jpl-au/llmd/internal/store"
)

// Search performs full-text search.
func (s *Service) Search(ctx context.Context, query, prefix string, includeDeleted, deletedOnly bool) ([]store.Document, error) {
	if prefix != "" {
		var err error
		prefix, err = path.Normalise(prefix)
		if err != nil {
			return nil, err
		}
	}
	return s.store.Search(ctx, query, prefix, includeDeleted, deletedOnly)
}
