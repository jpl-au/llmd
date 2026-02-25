package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/tags"
	"github.com/jpl-au/llmd/sdk"
)

// tagAPI implements [sdk.TagStore] by delegating to the internal tags
// package. The mapping is straightforward since SDK and internal types
// are closely aligned — the main translation is building a core.Origin
// from the author string and converting internal tag entities to SDK
// Tag/TagInfo structs.
type tagAPI struct {
	store *llmd.Store
}

// newTagAPI creates a tag API bridge wrapping the given store.
// The returned value satisfies [sdk.TagStore] and is assigned to the
// sdk.Tags global by [New].
func newTagAPI(store *llmd.Store) *tagAPI {
	return &tagAPI{store: store}
}

// Add attaches a tag to a document. Creates the tag entity if it does
// not already exist; no-ops if the tag is already present.
func (a *tagAPI) Add(path, name, author string) error {
	_, err := a.store.Tags.Add(context.Background(), path, name, tags.Options{
		Origin: origin(author),
	})
	return err
}

// Remove detaches a tag from a document via soft-delete.
func (a *tagAPI) Remove(path, name, author string) error {
	return a.store.Tags.Remove(context.Background(), path, name, tags.Options{
		Origin: origin(author),
	})
}

// List returns all tags attached to a document. Converts internal tag
// entities to SDK Tag structs.
func (a *tagAPI) List(path string) ([]sdk.Tag, error) {
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

// All returns every tag in the store with usage counts. Each TagInfo
// contains the tag name and the number of documents it appears on.
func (a *tagAPI) All() ([]sdk.TagInfo, error) {
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

// Find returns document paths that have the given tag attached.
func (a *tagAPI) Find(name string) ([]string, error) {
	return a.store.Tags.Find(context.Background(), name)
}
