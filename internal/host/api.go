// api.go bridges sdk.DocumentStore to the internal storage packages.
// Each method translates SDK types (flat args) into internal option
// structs (documents.*, history.*, search.*) and maps internal results
// back to SDK types.
//
// All mutating operations stamp a core.Origin with Source:"cli" so the
// version history records where the change came from.

package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/internal/llmd/search"
	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/sdk"
)

// origin builds a core.Origin for CLI operations.
func origin(author string) core.Origin {
	return core.Origin{Author: author, Source: "cli"}
}

// documentAPI implements [sdk.DocumentStore] by delegating to the
// internal storage packages (documents, history, search, bulk). Each
// method translates flat SDK arguments into the internal option structs
// and maps internal results back to SDK types. Mutating operations stamp
// a [core.Origin] with Source:"cli" so version history records the source.
type documentAPI struct {
	store *llmd.Store
}

// newDocumentAPI creates a document API bridge wrapping the given store.
// The returned value satisfies [sdk.DocumentStore] and is assigned to
// the sdk.Documents global by [New].
func newDocumentAPI(store *llmd.Store) *documentAPI {
	return &documentAPI{store: store}
}

// Read returns document content. Version 0 means latest (nil pointer
// in internal options); a positive version reads that specific version.
func (a *documentAPI) Read(path string, version int) ([]byte, error) {
	var opts documents.ReadOptions
	if version > 0 {
		opts.Version = &version
	}
	doc, err := a.store.Documents.Read(context.Background(), path, opts)
	if err != nil {
		return nil, err
	}
	return []byte(doc.Content), nil
}

// Write creates or updates a document, recording a new version.
// The author and message are recorded in the version history.
func (a *documentAPI) Write(path string, content []byte, author, msg string) error {
	o := origin(author)
	o.Message = msg
	_, err := a.store.Documents.Write(context.Background(), path, string(content), documents.WriteOptions{
		Origin: o,
	})
	return err
}

// Delete soft-deletes a document. The document can be recovered via
// Restore until a Vacuum permanently removes it.
func (a *documentAPI) Delete(path, author string) error {
	return a.store.Documents.Delete(context.Background(), path, documents.DeleteOptions{
		Origin: origin(author),
	})
}

// Restore recovers a soft-deleted document, clearing its deleted_at
// timestamp so it reappears in normal listings.
func (a *documentAPI) Restore(path, author string) error {
	return a.store.Documents.Restore(context.Background(), path, documents.RestoreOptions{
		Origin: origin(author),
	})
}

// Move renames a document, preserving its full version history.
// Tags and links follow the document to the new path.
func (a *documentAPI) Move(from, to, author string) error {
	return a.store.Documents.Move(context.Background(), from, to, documents.MoveOptions{
		Origin: origin(author),
	})
}

// List returns document metadata for all documents matching the given
// path prefix. Results are converted from internal Info structs to SDK
// Doc structs. The Reverse option is applied after the database query.
func (a *documentAPI) List(prefix string, opts sdk.ListOpts) ([]sdk.Doc, error) {
	infos, err := a.store.Documents.List(context.Background(), documents.ListOptions{
		Prefix:         prefix,
		IncludeDeleted: opts.Deleted,
		Sort:           opts.Sort,
	})
	if err != nil {
		return nil, err
	}

	docs := make([]sdk.Doc, len(infos))
	for i, info := range infos {
		docs[i] = sdk.Doc{
			Path:      info.Path,
			Version:   info.Version,
			Author:    info.Author,
			Message:   info.Message,
			CreatedAt: info.CreatedAt,
			Deleted:   info.DeletedAt != nil,
		}
	}

	if opts.Reverse {
		for i, j := 0, len(docs)-1; i < j; i, j = i+1, j-1 {
			docs[i], docs[j] = docs[j], docs[i]
		}
	}

	return docs, nil
}

// Exists reports whether a non-deleted document exists at the given path.
func (a *documentAPI) Exists(path string) (bool, error) {
	return a.store.Documents.Exists(context.Background(), path)
}

// Edit performs a search-and-replace within a document, creating a new
// version with the substitution applied.
func (a *documentAPI) Edit(path, old, new, author, msg string) error {
	o := origin(author)
	o.Message = msg
	_, err := a.store.Documents.Edit(context.Background(), path, old, new, documents.EditOptions{
		Origin: o,
	})
	return err
}

// Glob returns document paths matching a shell-style glob pattern.
// Delegates to the search package's glob implementation.
func (a *documentAPI) Glob(pattern string) ([]string, error) {
	return a.store.Search.Glob(context.Background(), pattern)
}

// Grep performs FTS5 full-text search. Maps sdk.GrepOpts to internal
// search.Options (the Mode enum values are identical by design so a
// direct cast works). Results are flattened: each search.Result may
// contain multiple matches, which become individual sdk.GrepHit entries.
// For GrepPaths mode, results have no matches — just a path.
func (a *documentAPI) Grep(query string, opts sdk.GrepOpts) ([]sdk.GrepHit, error) {
	searchOpts := search.Options{
		Path:    opts.Path,
		Mode:    search.Mode(opts.Mode),
		Context: opts.Context,
	}

	results, err := a.store.Search.FullText(context.Background(), query, searchOpts)
	if err != nil {
		return nil, err
	}

	var hits []sdk.GrepHit
	for _, r := range results {
		if len(r.Matches) == 0 {
			hits = append(hits, sdk.GrepHit{Path: r.Path})
			continue
		}
		for _, m := range r.Matches {
			hits = append(hits, sdk.GrepHit{
				Path:    r.Path,
				Line:    m.Line,
				Column:  m.Column,
				Text:    m.Text,
				Before:  m.Before,
				After:   m.After,
				Section: m.Section,
			})
		}
	}
	return hits, nil
}

// History returns version history for a document, newest first.
// Converts internal Info structs to SDK Version structs.
func (a *documentAPI) History(path string, limit int) ([]sdk.Version, error) {
	var opts history.ListOptions
	if limit > 0 {
		opts.Limit = limit
	}

	infos, err := a.store.History.List(context.Background(), path, opts)
	if err != nil {
		return nil, err
	}

	versions := make([]sdk.Version, len(infos))
	for i, info := range infos {
		versions[i] = sdk.Version{
			Number:    info.Version,
			Author:    info.Author,
			Message:   info.Message,
			CreatedAt: info.CreatedAt,
		}
	}
	return versions, nil
}

// Diff computes a unified diff between two document versions. Returns
// the diff text, lines added, and lines removed.
func (a *documentAPI) Diff(src, dst string, ctx int) (string, int, int, error) {
	var opts history.DiffOptions
	if ctx > 0 {
		opts.Context = ctx
	}

	result, err := a.store.History.Diff(context.Background(), src, dst, opts)
	if err != nil {
		return "", 0, 0, err
	}

	return result.Unified, result.Stats.Added, result.Stats.Removed, nil
}

// Revert creates a new version with the content from a previous version.
// The old version is preserved — revert is non-destructive.
func (a *documentAPI) Revert(path string, version int, author, msg string) error {
	o := origin(author)
	o.Message = msg
	_, err := a.store.History.Revert(context.Background(), path, version, history.RevertOptions{
		Origin: o,
	})
	return err
}

// Vacuum permanently removes all soft-deleted data and reclaims disk
// space. Returns counts of deleted documents, tags, and links.
func (a *documentAPI) Vacuum() (sdk.VacuumResult, error) {
	r, err := a.store.Vacuum(context.Background())
	if err != nil {
		return sdk.VacuumResult{}, err
	}
	return sdk.VacuumResult{
		Documents: r.Documents,
		Tags:      r.Tags,
		Links:     r.Links,
	}, nil
}

// Import reads markdown files from a filesystem directory into the store.
// Files are attributed to the "import" author. Results report which
// documents were created, updated, or skipped.
func (a *documentAPI) Import(dir string, opts sdk.ImportOpts) (*sdk.ImportResult, error) {
	r, err := a.store.Bulk.Import(context.Background(), dir, bulk.ImportOptions{
		Origin: origin("import"),
		Prefix: opts.Prefix,
		DryRun: opts.DryRun,
		Force:  opts.Force,
	})
	if err != nil {
		return nil, err
	}
	return &sdk.ImportResult{
		Created: r.Created,
		Updated: r.Updated,
		Skipped: r.Skipped,
	}, nil
}

// Export writes store documents to a filesystem directory as .md files.
// Preserves the document path hierarchy under the destination directory.
func (a *documentAPI) Export(prefix, dir string, opts sdk.ExportOpts) (*sdk.ExportResult, error) {
	r, err := a.store.Bulk.Export(context.Background(), prefix, dir, bulk.ExportOptions{
		Overwrite: opts.Overwrite,
	})
	if err != nil {
		return nil, err
	}
	return &sdk.ExportResult{
		Exported: r.Exported,
		Skipped:  r.Skipped,
	}, nil
}
