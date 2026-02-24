// api.go bridges sdk.Store to the internal storage packages. Each method
// translates sdk types (flat args) into internal option structs (documents.*,
// history.*, search.*) and maps internal results back to sdk types.
//
// All mutating operations stamp a core.Origin with Source:"cli" so the
// version history records where the change came from.

package host

import (
	"context"
	"errors"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/internal/llmd/search"
	"github.com/jpl-au/llmd/internal/llmd/tags"
	"github.com/jpl-au/llmd/internal/llmd/tasks"
	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/pkg/model/task"
	"github.com/jpl-au/llmd/sdk"
)

// api implements sdk.Store by delegating to the internal storage packages.
type api struct {
	store *llmd.Store
}

// newAPI wraps a Store as an sdk.Store for plugin access.
func newAPI(store *llmd.Store) *api {
	return &api{store: store}
}

// Read returns document content. Version 0 means latest (nil pointer
// in internal options); a positive version reads that specific version.
func (a *api) Read(path string, version int) ([]byte, error) {
	var opts documents.ReadOptions
	// nil Version pointer = latest; non-nil = specific version
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
func (a *api) Write(path string, content []byte, author, msg string) error {
	_, err := a.store.Documents.Write(context.Background(), path, string(content), documents.WriteOptions{
		Origin: core.Origin{Author: author, Message: msg, Source: "cli"},
	})
	return err
}

// Delete soft-deletes a document (recoverable until Vacuum).
func (a *api) Delete(path, author string) error {
	return a.store.Documents.Delete(context.Background(), path, documents.DeleteOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
}

// Restore recovers a soft-deleted document.
func (a *api) Restore(path, author string) error {
	return a.store.Documents.Restore(context.Background(), path, documents.RestoreOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
}

// Move renames a document, preserving its version history.
func (a *api) Move(from, to, author string) error {
	return a.store.Documents.Move(context.Background(), from, to, documents.MoveOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
}

// List returns documents matching the path prefix. Reverse is applied
// here rather than in the database query so all sort modes (path,
// time) can be reversed uniformly.
func (a *api) List(prefix string, opts sdk.ListOpts) ([]sdk.Doc, error) {
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

// Exists reports whether a non-deleted document exists at path.
func (a *api) Exists(path string) (bool, error) {
	return a.store.Documents.Exists(context.Background(), path)
}

// Glob returns document paths matching a shell-style glob pattern.
func (a *api) Glob(pattern string) ([]string, error) {
	return a.store.Search.Glob(context.Background(), pattern)
}

// Grep performs FTS5 full-text search. Maps sdk.GrepOpts to internal
// search.Options (the Mode enum values are identical by design so a
// direct cast works). Results are flattened: each search.Result may
// contain multiple matches, which become individual sdk.GrepHit entries.
// For GrepPaths mode, results have no matches — just a path.
func (a *api) Grep(query string, opts sdk.GrepOpts) ([]sdk.GrepHit, error) {
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
			// GrepPaths mode: no match detail, just the path
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
// Limit 0 means all versions (zero-value ListOptions has no limit).
func (a *api) History(path string, limit int) ([]sdk.Version, error) {
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
			Num:       info.Version,
			Author:    info.Author,
			Message:   info.Message,
			CreatedAt: info.CreatedAt,
		}
	}
	return versions, nil
}

// Diff computes a unified diff between two document versions.
// src and dst use "path" or "path:version" format. Context 0 means
// the default (3 lines). Returns unified diff text, lines added,
// and lines removed.
func (a *api) Diff(src, dst string, ctx int) (string, int, int, error) {
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
func (a *api) Revert(path string, version int, author, msg string) error {
	_, err := a.store.History.Revert(context.Background(), path, version, history.RevertOptions{
		Origin: core.Origin{Author: author, Message: msg, Source: "cli"},
	})
	return err
}

// Edit performs a search-and-replace within a document, creating a new version.
func (a *api) Edit(path, old, new, author, msg string) error {
	_, err := a.store.Documents.Edit(context.Background(), path, old, new, documents.EditOptions{
		Origin: core.Origin{Author: author, Message: msg, Source: "cli"},
	})
	return err
}

// Vacuum permanently deletes all soft-deleted data and reclaims disk space.
func (a *api) Vacuum() (sdk.VacuumResult, error) {
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

// TagAdd attaches a tag to a document.
func (a *api) TagAdd(path, name, author string) error {
	_, err := a.store.Tags.Add(context.Background(), path, name, tags.Options{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
	return err
}

// TagRemove removes a tag from a document.
func (a *api) TagRemove(path, name, author string) error {
	return a.store.Tags.Remove(context.Background(), path, name, tags.Options{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
}

// TagList returns all tags on a document.
func (a *api) TagList(path string) ([]sdk.Tag, error) {
	tt, err := a.store.Tags.List(context.Background(), path)
	if err != nil {
		return nil, err
	}
	out := make([]sdk.Tag, len(tt))
	for i, t := range tt {
		out[i] = sdk.Tag{Name: t.Value.Tag, Path: t.Relation}
	}
	return out, nil
}

// Tags returns all tags in the store with usage counts.
func (a *api) Tags() ([]sdk.TagInfo, error) {
	infos, err := a.store.Tags.ListAll(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]sdk.TagInfo, len(infos))
	for i, info := range infos {
		out[i] = sdk.TagInfo{Name: info.Name, Count: info.Count}
	}
	return out, nil
}

// TagFind returns document paths that have the given tag.
func (a *api) TagFind(name string) ([]string, error) {
	return a.store.Tags.Find(context.Background(), name)
}

// LinkAdd creates a directed link between two documents.
func (a *api) LinkAdd(from, to, label, author string) error {
	_, err := a.store.Links.Add(context.Background(), from, to, links.Options{
		Origin: core.Origin{Author: author, Source: "cli"},
		Label:  label,
	})
	return err
}

// LinkRemove removes a link between two documents.
func (a *api) LinkRemove(from, to, author string) error {
	return a.store.Links.Remove(context.Background(), from, to, links.Options{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
}

// LinkList returns links for a document.
func (a *api) LinkList(path, dir string) ([]sdk.Link, error) {
	var d links.Direction
	switch dir {
	case "in":
		d = links.Incoming
	case "both":
		d = links.Both
	default:
		d = links.Outgoing
	}
	ll, err := a.store.Links.List(context.Background(), path, links.Options{
		Direction: d,
	})
	if err != nil {
		return nil, err
	}
	out := make([]sdk.Link, len(ll))
	for i, l := range ll {
		out[i] = sdk.Link{From: l.Relation, To: l.Value.To, Label: l.Value.Label}
	}
	return out, nil
}

// Import reads files from a filesystem directory into the store.
func (a *api) Import(dir string, opts sdk.ImportOpts) (*sdk.ImportResult, error) {
	r, err := a.store.Bulk.Import(context.Background(), dir, bulk.ImportOptions{
		Origin: core.Origin{Source: "cli"},
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

// Export writes documents to a filesystem directory.
func (a *api) Export(prefix, dir string, opts sdk.ExportOpts) (*sdk.ExportResult, error) {
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

func taskToSDK(t *task.Task) *sdk.Task {
	return &sdk.Task{
		Key:        t.Key,
		Title:      t.Title,
		Status:     t.Status,
		Priority:   t.Priority,
		Position:   t.Position,
		AssignedTo: t.AssignedTo,
		Flags:      t.Flags,
		Path:       t.Path,
		Author:     t.Author,
		CreatedAt:  t.CreatedAt,
	}
}

func (a *api) TaskAdd(title string, body []byte, opts sdk.TaskAddOpts) (*sdk.Task, error) {
	t, err := a.store.Tasks.Add(context.Background(), title, body, tasks.AddOptions{
		Origin:     core.Origin{Author: opts.Author, Source: "cli"},
		Status:     opts.Status,
		Priority:   opts.Priority,
		AssignedTo: opts.AssignedTo,
		Path:       opts.Path,
	})
	if err != nil {
		return nil, err
	}
	return taskToSDK(t), nil
}

func (a *api) TaskRead(key string) (*sdk.Task, error) {
	t, err := a.store.Tasks.Read(context.Background(), key)
	if err != nil {
		return nil, err
	}
	return taskToSDK(t), nil
}

func (a *api) TaskList(opts sdk.TaskListOpts) ([]*sdk.Task, error) {
	tt, err := a.store.Tasks.List(context.Background(), tasks.ListOptions{
		Status:     opts.Status,
		AssignedTo: opts.AssignedTo,
		Priority:   opts.Priority,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*sdk.Task, len(tt))
	for i, t := range tt {
		out[i] = taskToSDK(t)
	}
	return out, nil
}

func (a *api) TaskMove(key, status, author string) error {
	err := a.store.Tasks.Move(context.Background(), key, status, author)
	if errors.Is(err, tasks.ErrNoSpec) {
		return sdk.ErrNoSpec
	}
	return err
}

func (a *api) TaskSet(key, author string, opts sdk.TaskSetOpts) error {
	return a.store.Tasks.Set(context.Background(), key, author, tasks.SetOptions{
		Title:      opts.Title,
		Priority:   opts.Priority,
		Position:   opts.Position,
		AssignedTo: opts.AssignedTo,
		Flag:       opts.Flag,
		Unflag:     opts.Unflag,
	})
}

func (a *api) TaskDelete(key, author string) (*sdk.Task, error) {
	t, err := a.store.Tasks.Delete(context.Background(), key, author)
	if err != nil {
		return nil, err
	}
	return taskToSDK(t), nil
}

func (a *api) TaskRestore(key, author string) (*sdk.Task, error) {
	t, err := a.store.Tasks.Restore(context.Background(), key, author)
	if err != nil {
		return nil, err
	}
	return taskToSDK(t), nil
}

func (a *api) TaskColumns() ([]string, error) {
	return a.store.Tasks.Columns(context.Background())
}

func (a *api) TaskAddColumn(name, after, author string) error {
	return a.store.Tasks.AddColumn(context.Background(), name, after, author)
}

func (a *api) TaskRemoveColumn(name, author string) error {
	return a.store.Tasks.RemoveColumn(context.Background(), name, author)
}

func (a *api) TaskMoveColumn(name, after, author string) error {
	return a.store.Tasks.MoveColumn(context.Background(), name, after, author)
}

func (a *api) TaskLog(key string, limit int) ([]sdk.TaskEvent, error) {
	events, err := a.store.Tasks.Log(context.Background(), key, limit)
	if err != nil {
		return nil, err
	}
	out := make([]sdk.TaskEvent, len(events))
	for i, e := range events {
		out[i] = sdk.TaskEvent{
			Timestamp: e.Timestamp,
			Subject:   e.Subject,
			Actor:     e.Actor,
			Action:    e.Action,
			OldValue:  e.OldValue,
			NewValue:  e.NewValue,
		}
	}
	return out, nil
}
