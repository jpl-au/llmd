package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/documents"
	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/internal/llmd/search"
	"github.com/jpl-au/llmd/pkg/model/core"
	"github.com/jpl-au/llmd/sdk"
)

type api struct {
	store *llmd.Store
}

func newAPI(store *llmd.Store) *api {
	return &api{store: store}
}

func (a *api) Read(path string, version int) ([]byte, error) {
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

func (a *api) Write(path string, content []byte, author, msg string) error {
	_, err := a.store.Documents.Write(context.Background(), path, string(content), documents.WriteOptions{
		Origin: core.Origin{Author: author, Message: msg, Source: "cli"},
	})
	return err
}

func (a *api) Delete(path, author string) error {
	return a.store.Documents.Delete(context.Background(), path, documents.DeleteOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
}

func (a *api) Restore(path, author string) error {
	return a.store.Documents.Restore(context.Background(), path, documents.RestoreOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
}

func (a *api) Move(from, to, author string) error {
	return a.store.Documents.Move(context.Background(), from, to, documents.MoveOptions{
		Origin: core.Origin{Author: author, Source: "cli"},
	})
}

func (a *api) List(prefix string, opts sdk.ListOpts) ([]sdk.Doc, error) {
	infos, err := a.store.Documents.List(context.Background(), documents.ListOptions{
		Prefix:         prefix,
		IncludeDeleted: opts.Deleted,
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

func (a *api) Exists(path string) (bool, error) {
	return a.store.Documents.Exists(context.Background(), path)
}

func (a *api) Glob(pattern string) ([]string, error) {
	return a.store.Search.Glob(context.Background(), pattern)
}

func (a *api) Grep(query string, opts sdk.GrepOpts) ([]sdk.GrepHit, error) {
	docs, err := a.store.Search.FullText(context.Background(), query, search.Options{Path: opts.Path})
	if err != nil {
		return nil, err
	}

	hits := make([]sdk.GrepHit, len(docs))
	for i, doc := range docs {
		hits[i] = sdk.GrepHit{
			Path: doc.Path,
			Line: 1,
			Text: doc.Content,
		}
	}
	return hits, nil
}

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

func (a *api) Revert(path string, version int, author, msg string) error {
	_, err := a.store.History.Revert(context.Background(), path, version, history.RevertOptions{
		Origin: core.Origin{Author: author, Message: msg, Source: "cli"},
	})
	return err
}

func (a *api) Edit(path, old, new, author, msg string) error {
	_, err := a.store.Documents.Edit(context.Background(), path, old, new, documents.EditOptions{
		Origin: core.Origin{Author: author, Message: msg, Source: "cli"},
	})
	return err
}
