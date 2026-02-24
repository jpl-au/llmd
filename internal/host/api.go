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

// documentAPI implements sdk.DocumentStore by delegating to the
// internal storage packages.
type documentAPI struct {
	store *llmd.Store
}

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

func (a *documentAPI) Write(path string, content []byte, author, msg string) error {
	o := origin(author)
	o.Message = msg
	_, err := a.store.Documents.Write(context.Background(), path, string(content), documents.WriteOptions{
		Origin: o,
	})
	return err
}

func (a *documentAPI) Delete(path, author string) error {
	return a.store.Documents.Delete(context.Background(), path, documents.DeleteOptions{
		Origin: origin(author),
	})
}

func (a *documentAPI) Restore(path, author string) error {
	return a.store.Documents.Restore(context.Background(), path, documents.RestoreOptions{
		Origin: origin(author),
	})
}

func (a *documentAPI) Move(from, to, author string) error {
	return a.store.Documents.Move(context.Background(), from, to, documents.MoveOptions{
		Origin: origin(author),
	})
}

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

func (a *documentAPI) Exists(path string) (bool, error) {
	return a.store.Documents.Exists(context.Background(), path)
}

func (a *documentAPI) Edit(path, old, new, author, msg string) error {
	o := origin(author)
	o.Message = msg
	_, err := a.store.Documents.Edit(context.Background(), path, old, new, documents.EditOptions{
		Origin: o,
	})
	return err
}

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

func (a *documentAPI) Revert(path string, version int, author, msg string) error {
	o := origin(author)
	o.Message = msg
	_, err := a.store.History.Revert(context.Background(), path, version, history.RevertOptions{
		Origin: o,
	})
	return err
}

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
