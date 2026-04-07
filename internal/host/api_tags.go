package host

import (
	"context"
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/resolve"
	"github.com/jpl-au/llmd/internal/llmd/tags"
	"github.com/jpl-au/llmd/internal/validate"
	"github.com/jpl-au/llmd/sdk"
)

// tagErr translates internal tag errors to SDK sentinel errors.
func tagErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, tags.ErrNotFound):
		return fmt.Errorf("%w: %w", sdk.ErrNotFound, err)
	case errors.Is(err, tags.ErrInvalid):
		return fmt.Errorf("%w: %w", sdk.ErrInvalidArg, err)
	case errors.Is(err, tags.ErrExists):
		return fmt.Errorf("%w: %w", sdk.ErrExists, err)
	default:
		return err
	}
}

// tagAPI implements [sdk.TagStore] by delegating to the internal tags
// package. The mapping is straightforward since SDK and internal types
// are closely aligned - the main translation is building a core.Origin
// from the author string and converting internal tag entities to SDK
// Tag/TagInfo structs.
type tagAPI struct {
	ctx   context.Context
	store *llmd.Store
	lim   validate.Limits
}

// newTagAPI creates a tag API bridge wrapping the given store.
// The context controls cancellation and timeout for all store operations.
func newTagAPI(store *llmd.Store, lim validate.Limits, ctx context.Context) *tagAPI {
	return &tagAPI{ctx: ctx, store: store, lim: lim}
}

// Add attaches a tag to a document. Creates the tag entity if it does
// not already exist; no-ops if the tag is already present.
func (a *tagAPI) Add(path, name, author string) error {
	path, _, _ = resolve.Identifier(a.ctx, path, a.store.Documents.KeyToPath)
	if err := errors.Join(
		validate.Path(path, a.lim),
		validate.Text(name, "tag name"),
	); err != nil {
		return err
	}
	_, err := a.store.Tags.Add(a.ctx, path, name, tags.Options{
		Origin: origin(author),
	})
	return tagErr(err)
}

// Remove detaches a tag from a document via soft-delete.
func (a *tagAPI) Remove(path, name, author string) error {
	path, _, _ = resolve.Identifier(a.ctx, path, a.store.Documents.KeyToPath)
	if err := validate.Text(name, "tag name"); err != nil {
		return err
	}
	return tagErr(a.store.Tags.Remove(a.ctx, path, name, tags.Options{
		Origin: origin(author),
	}))
}

// List returns all tags attached to a document. Converts internal tag
// entities to SDK Tag structs.
func (a *tagAPI) List(path string) ([]sdk.Tag, error) {
	path, _, _ = resolve.Identifier(a.ctx, path, a.store.Documents.KeyToPath)
	tt, err := a.store.Tags.List(a.ctx, path)
	if err != nil {
		return nil, err
	}
	out := make([]sdk.Tag, len(tt))
	for i, t := range tt {
		out[i] = sdk.Tag{Name: t.Value.Tag, Path: t.Relation}
	}
	return out, nil
}

// All returns every tag in the store with usage counts. Each TagInfo
// contains the tag name and the number of documents it appears on.
func (a *tagAPI) All() ([]sdk.TagInfo, error) {
	infos, err := a.store.Tags.ListAll(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]sdk.TagInfo, len(infos))
	for i, info := range infos {
		out[i] = sdk.TagInfo{Name: info.Name, Count: info.Count}
	}
	return out, nil
}

// Find returns document paths that have the given tag attached.
func (a *tagAPI) Find(name string) ([]string, error) {
	if err := validate.Text(name, "tag name"); err != nil {
		return nil, err
	}
	return a.store.Tags.Find(a.ctx, name)
}
