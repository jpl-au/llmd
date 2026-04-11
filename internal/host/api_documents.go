package host

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/internal/llmd/resolve"
	"github.com/jpl-au/llmd/internal/llmd/search"
	docpath "github.com/jpl-au/llmd/internal/path"
	"github.com/jpl-au/llmd/internal/validate"
	"github.com/jpl-au/llmd/sdk"
)

// docErr translates internal document and history errors to SDK sentinel
// errors. Both packages define their own ErrNotFound; both map to
// sdk.ErrNotFound. Edit-specific errors (no match, not unique, no-op)
// map to their SDK counterparts so consumers can branch on stable
// sentinels. All other errors pass through unchanged.
func docErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, documents.ErrNotFound):
		return fmt.Errorf("%w: %w", sdk.ErrNotFound, err)
	case errors.Is(err, history.ErrNotFound):
		return fmt.Errorf("%w: %w", sdk.ErrNotFound, err)
	case errors.Is(err, documents.ErrNoMatch):
		return fmt.Errorf("%w: %w", sdk.ErrNoMatch, err)
	case errors.Is(err, documents.ErrNotUnique):
		return fmt.Errorf("%w: %w", sdk.ErrNotUnique, err)
	case errors.Is(err, documents.ErrNoOp):
		return fmt.Errorf("%w: %w", sdk.ErrNoOp, err)
	default:
		return err
	}
}

// documentAPI implements [sdk.DocumentStore] by delegating to the
// internal storage packages (documents, history, search, bulk). Each
// method translates flat SDK arguments into the internal option structs
// and maps internal results back to SDK types. Mutating operations stamp
// a [core.Origin] with Source:"cli" so version history records the source.
type documentAPI struct {
	ctx   context.Context
	store *llmd.Store
	lim   validate.Limits
}

// newDocumentAPI creates a document API bridge wrapping the given store.
// The context controls cancellation and timeout for all store operations.
func newDocumentAPI(store *llmd.Store, lim validate.Limits, ctx context.Context) *documentAPI {
	return &documentAPI{ctx: ctx, store: store, lim: lim}
}

// resolveDoc translates an identifier (path, key, or either with
// :version suffix) to a normalised path and optional version. The
// version parameter from flags takes precedence when non-zero.
func (a *documentAPI) resolveDoc(identifier string, flagVersion int) (string, *int, error) {
	p, v, _ := resolve.Identifier(a.ctx, identifier, a.store.Documents.KeyToPath)
	p, err := docpath.Normalise(p)
	if err != nil {
		return "", nil, err
	}
	if flagVersion > 0 {
		return p, &flagVersion, nil
	}
	return p, v, nil
}

// Read returns document content. Version 0 means latest (nil pointer
// in internal options); a positive version reads that specific version.
func (a *documentAPI) Read(path string, version int) ([]byte, error) {
	p, v, err := a.resolveDoc(path, version)
	if err != nil {
		return nil, err
	}
	if err := validate.Path(p, a.lim); err != nil {
		return nil, err
	}
	var opts documents.ReadOptions
	if v != nil {
		opts.Version = v
	}
	doc, err := a.store.Documents.Read(a.ctx, p, opts)
	if err != nil {
		return nil, docErr(err)
	}
	return []byte(doc.Content), nil
}

// Write creates or updates a document, recording a new version.
// The author and message in opts are recorded in the version history.
func (a *documentAPI) Write(path string, content []byte, opts sdk.WriteOpts) error {
	p, _, err := a.resolveDoc(path, 0)
	if err != nil {
		return err
	}
	path = p
	if err := errors.Join(
		validate.Path(path, a.lim),
		validate.Content(content, a.lim),
	); err != nil {
		return err
	}
	// Normalise line endings to \n so content is consistent regardless
	// of which OS wrote it.
	s := strings.ReplaceAll(string(content), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	o := origin(opts.Author)
	o.Message = opts.Message
	_, err = a.store.Documents.Write(a.ctx, path, s, documents.WriteOptions{
		Origin: o,
	})
	return docErr(err)
}

// Delete soft-deletes a document. The document can be recovered via
// Restore until a Vacuum permanently removes it.
func (a *documentAPI) Delete(path string, opts sdk.DeleteOpts) error {
	p, _, err := a.resolveDoc(path, 0)
	if err != nil {
		return err
	}
	path = p
	if err := validate.Path(path, a.lim); err != nil {
		return err
	}
	return docErr(a.store.Documents.Delete(a.ctx, path, documents.DeleteOptions{
		Origin: origin(opts.Author),
	}))
}

// Restore recovers a soft-deleted document, clearing its deleted_at
// timestamp so it reappears in normal listings.
func (a *documentAPI) Restore(path string, opts sdk.RestoreOpts) error {
	p, _, err := a.resolveDoc(path, 0)
	if err != nil {
		return err
	}
	path = p
	if err := validate.Path(path, a.lim); err != nil {
		return err
	}
	return docErr(a.store.Documents.Restore(a.ctx, path, documents.RestoreOptions{
		Origin: origin(opts.Author),
	}))
}

// Move renames a document, preserving its full version history.
// Tags and links follow the document to the new path.
func (a *documentAPI) Move(from, to string, opts sdk.MoveOpts) error {
	var errs []error
	f, _, err := a.resolveDoc(from, 0)
	if err != nil {
		errs = append(errs, err)
	} else if err := validate.Path(f, a.lim); err != nil {
		errs = append(errs, err)
	}
	from = f
	to, err = docpath.Normalise(to)
	if err != nil {
		errs = append(errs, err)
	} else if err := validate.Path(to, a.lim); err != nil {
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	return docErr(a.store.Documents.Move(a.ctx, from, to, documents.MoveOptions{
		Origin: origin(opts.Author),
	}))
}

// List returns document metadata for all documents matching the given
// path prefix. Results are converted from internal Info structs to SDK
// Doc structs. The Reverse option is applied after the database query.
func (a *documentAPI) List(prefix string, opts sdk.ListOpts) ([]sdk.Doc, error) {
	if err := validate.Null(prefix, "prefix"); err != nil {
		return nil, err
	}
	if strings.Contains(prefix, "..") {
		return nil, docpath.ErrInvalid
	}
	var sinceMS int64
	if !opts.Since.IsZero() {
		sinceMS = opts.Since.UnixMilli()
	}
	infos, err := a.store.Documents.List(a.ctx, documents.ListOptions{
		Prefix:         prefix,
		IncludeDeleted: opts.Deleted,
		Sort:           opts.Sort,
		SinceMS:        sinceMS,
	})
	if err != nil {
		return nil, err
	}

	docs := make([]sdk.Doc, len(infos))
	for i, info := range infos {
		docs[i] = sdk.Doc{
			Key:       info.Key,
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
	p, _, err := a.resolveDoc(path, 0)
	if err != nil {
		return false, err
	}
	path = p
	if err := validate.Path(path, a.lim); err != nil {
		return false, err
	}
	return a.store.Documents.Exists(a.ctx, path)
}

// Edit performs a search-and-replace within a document, creating a new
// version with the substitution applied. The search string must match
// exactly once unless opts.ReplaceAll is set. See [sdk.EditOpts] and
// [documents.Documents.Edit] for the full semantics.
func (a *documentAPI) Edit(path, old, new string, opts sdk.EditOpts) error {
	p, _, err := a.resolveDoc(path, 0)
	if err != nil {
		return err
	}
	path = p
	if err := errors.Join(
		validate.Path(path, a.lim),
		validate.Text(old, "old text"),
		validate.Text(new, "new text"),
	); err != nil {
		return err
	}
	o := origin(opts.Author)
	o.Message = opts.Message
	_, err = a.store.Documents.Edit(a.ctx, path, old, new, documents.EditOptions{
		Origin:     o,
		ReplaceAll: opts.ReplaceAll,
	})
	return docErr(err)
}

// Glob returns document paths matching a shell-style glob pattern.
// Delegates to the search package's glob implementation.
func (a *documentAPI) Glob(pattern string) ([]string, error) {
	return a.store.Search.Glob(a.ctx, pattern)
}

// Grep performs FTS5 full-text search. Maps sdk.GrepOpts to internal
// search.Options (the Mode enum values are identical by design so a
// direct cast works). Results are flattened: each search.Result may
// contain multiple matches, which become individual sdk.GrepHit entries.
// For GrepPaths mode, results have no matches - just a path.
func (a *documentAPI) Grep(query string, opts sdk.GrepOpts) ([]sdk.GrepHit, error) {
	var errs []error
	if err := validate.Null(query, "query"); err != nil {
		errs = append(errs, err)
	}
	if err := validate.Null(opts.Path, "path"); err != nil {
		errs = append(errs, err)
	} else if strings.Contains(opts.Path, "..") {
		errs = append(errs, docpath.ErrInvalid)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	searchOpts := search.Options{
		Path:    opts.Path,
		Mode:    search.Mode(opts.Mode),
		Context: opts.Context,
	}

	// FTS5 treats a lot of punctuation as syntax (#, -, :, etc.) and
	// silently tokenises some characters to nothing, so a literal
	// search for "#" or "foo-bar" either errors or returns no hits.
	// Quote the query as a phrase by default so literal searches just
	// work, and only leave it alone when it clearly contains
	// deliberate FTS5 syntax (boolean operators, prefix *, or the
	// user has already added their own quotes).
	if needsFTS5Quoting(query) {
		query = quoteForFTS5(query)
	}
	results, err := a.store.Search.FullText(a.ctx, query, searchOpts)
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
	p, _, err := a.resolveDoc(path, 0)
	if err != nil {
		return nil, err
	}
	path = p
	if err := validate.Path(path, a.lim); err != nil {
		return nil, err
	}
	var opts history.ListOptions
	if limit > 0 {
		opts.Limit = limit
	}

	infos, err := a.store.History.List(a.ctx, path, opts)
	if err != nil {
		return nil, docErr(err)
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

	result, err := a.store.History.Diff(a.ctx, src, dst, opts)
	if err != nil {
		return "", 0, 0, err
	}

	return result.Unified, result.Stats.Added, result.Stats.Removed, nil
}

// Revert creates a new version with the content from a previous version.
// The old version is preserved - revert is non-destructive.
func (a *documentAPI) Revert(path string, version int, opts sdk.RevertOpts) error {
	p, v, err := a.resolveDoc(path, version)
	if err != nil {
		return err
	}
	path = p
	if v != nil {
		version = *v
	}
	if err := validate.Path(path, a.lim); err != nil {
		return err
	}
	o := origin(opts.Author)
	o.Message = opts.Message
	_, err = a.store.History.Revert(a.ctx, path, version, history.RevertOptions{
		Origin: o,
	})
	return docErr(err)
}

// Vacuum permanently removes all soft-deleted data and reclaims disk
// space. Returns counts of deleted documents, tags, and links.
func (a *documentAPI) Vacuum() (sdk.VacuumResult, error) {
	r, err := a.store.Vacuum(a.ctx)
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
	r, err := a.store.Bulk.Import(a.ctx, dir, bulk.ImportOptions{
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

// Preview returns the first n non-blank lines of a document, skipping
// the title heading. Returns empty string for missing documents.
func (a *documentAPI) Preview(path string, n int) (string, error) {
	if path == "" || n <= 0 {
		return "", nil
	}
	body, err := a.Read(path, 0)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(body), "\n")
	var preview []string
	for _, line := range lines {
		if len(preview) == 0 && strings.HasPrefix(line, "# ") {
			continue
		}
		if strings.TrimSpace(line) == "" && len(preview) == 0 {
			continue
		}
		preview = append(preview, line)
		if len(preview) >= n {
			break
		}
	}
	return strings.Join(preview, "\n"), nil
}

// Export writes store documents to a filesystem directory as .md files.
// Preserves the document path hierarchy under the destination directory.
func (a *documentAPI) Export(prefix, dir string, opts sdk.ExportOpts) (*sdk.ExportResult, error) {
	r, err := a.store.Bulk.Export(a.ctx, prefix, dir, bulk.ExportOptions{
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

// quoteForFTS5 wraps a query in double quotes so FTS5 treats it as a
// literal phrase. Embedded double quotes are escaped by doubling them,
// per the FTS5 string syntax. The 99% case for grep is "find this
// string", so most queries pass through this on their way to the
// search engine.
func quoteForFTS5(query string) string {
	return `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
}

// needsFTS5Quoting reports whether a query should be wrapped as a
// literal phrase before going to FTS5. The default answer is yes -
// users want literal searches and FTS5's punctuation handling is a
// footgun. The exceptions are queries that clearly use FTS5 syntax
// deliberately:
//
//   - Contains a double quote: the user is managing quoting themselves
//     (e.g. `"exact phrase" OR baz`).
//   - Contains a prefix wildcard `*` outside a quoted phrase.
//   - Contains a boolean operator as a standalone token: ` AND `, ` OR `,
//     ` NOT `, or `NEAR(`. The surrounding spaces and parenthesis avoid
//     matching English words like "AUTHOR" or "north".
//
// Anything else - punctuation-only queries, hyphenated terms, phrases
// with colons - is quoted so FTS5 treats it literally.
func needsFTS5Quoting(query string) bool {
	if strings.ContainsAny(query, `"*`) {
		return false
	}
	if strings.Contains(query, " AND ") ||
		strings.Contains(query, " OR ") ||
		strings.Contains(query, " NOT ") ||
		strings.Contains(query, "NEAR(") {
		return false
	}
	return true
}
