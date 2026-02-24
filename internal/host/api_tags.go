package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/tags"
	"github.com/jpl-au/llmd/sdk"
)

// tagAPI implements sdk.TagStore by delegating to the internal tags package.
type tagAPI struct {
	store *llmd.Store
}

func newTagAPI(store *llmd.Store) *tagAPI {
	return &tagAPI{store: store}
}

func (a *tagAPI) Add(path, name, author string) error {
	_, err := a.store.Tags.Add(context.Background(), path, name, tags.Options{
		Origin: origin(author),
	})
	return err
}

func (a *tagAPI) Remove(path, name, author string) error {
	return a.store.Tags.Remove(context.Background(), path, name, tags.Options{
		Origin: origin(author),
	})
}

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

func (a *tagAPI) Find(name string) ([]string, error) {
	return a.store.Tags.Find(context.Background(), name)
}
