package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/sdk"
)

// linkAPI implements sdk.LinkStore by delegating to the internal links package.
type linkAPI struct {
	store *llmd.Store
}

func newLinkAPI(store *llmd.Store) *linkAPI {
	return &linkAPI{store: store}
}

func (a *linkAPI) Add(from, to, label, author string) error {
	_, err := a.store.Links.Add(context.Background(), from, to, links.Options{
		Origin: origin(author),
		Label:  label,
	})
	return err
}

func (a *linkAPI) Remove(from, to, author string) error {
	return a.store.Links.Remove(context.Background(), from, to, links.Options{
		Origin: origin(author),
	})
}

func (a *linkAPI) List(path, dir string) ([]sdk.Link, error) {
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
