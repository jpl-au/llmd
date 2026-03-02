package host

import (
	"context"
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/links"
	"github.com/jpl-au/llmd/sdk"
)

// linkErr translates internal link errors to SDK sentinel errors.
func linkErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, links.ErrNotFound):
		return fmt.Errorf("%w: %v", sdk.ErrNotFound, err)
	case errors.Is(err, links.ErrSelfLink):
		return fmt.Errorf("%w: %v", sdk.ErrInvalidArg, err)
	case errors.Is(err, links.ErrExists):
		return fmt.Errorf("%w: %v", sdk.ErrExists, err)
	default:
		return err
	}
}

// linkAPI implements [sdk.LinkStore] by delegating to the internal links
// package. It translates between the SDK's string-based direction parameter
// ("in", "out", "both") and the internal typed Direction constants.
type linkAPI struct {
	store *llmd.Store
	lim   limits
}

// newLinkAPI creates a link API bridge wrapping the given store.
// The returned value satisfies [sdk.LinkStore] and is assigned to the
// sdk.Links global by [New].
func newLinkAPI(store *llmd.Store, lim limits) *linkAPI {
	return &linkAPI{store: store, lim: lim}
}

// Add creates a directed link from one document to another with an
// optional label. Stamps a CLI origin for provenance tracking.
func (a *linkAPI) Add(from, to, label, author string) error {
	if err := checkPath(from, a.lim); err != nil {
		return err
	}
	if err := checkPath(to, a.lim); err != nil {
		return err
	}
	if err := checkText(label, "label"); err != nil {
		return err
	}
	_, err := a.store.Links.Add(context.Background(), from, to, links.Options{
		Origin: origin(author),
		Label:  label,
	})
	return linkErr(err)
}

// Remove deletes the link between two documents.
func (a *linkAPI) Remove(from, to, author string) error {
	return linkErr(a.store.Links.Remove(context.Background(), from, to, links.Options{
		Origin: origin(author),
	}))
}

// List returns links for a document. The dir parameter controls which
// direction to query: "in" for incoming, "out" (default) for outgoing,
// "both" for all links. Converts the string direction to the internal
// Direction type before querying.
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
