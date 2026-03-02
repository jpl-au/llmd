package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/sdk"
)

// mirrorAPI implements [sdk.MirrorStore] by delegating to the internal
// bulk package for both pull (store → filesystem) and push (filesystem
// → store) operations.
type mirrorAPI struct {
	store *llmd.Store
}

func newMirrorAPI(store *llmd.Store) *mirrorAPI {
	return &mirrorAPI{store: store}
}

// Pull writes store documents to the filesystem, skipping unchanged
// files and removing stale ones.
func (a *mirrorAPI) Pull(prefix, dir string) (*sdk.PullResult, error) {
	r, err := a.store.Bulk.Mirror(context.Background(), prefix, dir)
	if err != nil {
		return nil, err
	}
	return &sdk.PullResult{
		Wrote:   r.Wrote,
		Skipped: r.Skipped,
		Removed: r.Removed,
	}, nil
}

// Push imports .md files from the filesystem back into the store.
func (a *mirrorAPI) Push(dir string, opts sdk.PushOpts) (*sdk.PushResult, error) {
	r, err := a.store.Bulk.Import(context.Background(), dir, bulk.ImportOptions{
		Origin: origin("mirror"),
		Prefix: opts.Prefix,
	})
	if err != nil {
		return nil, err
	}
	if r == nil {
		return &sdk.PushResult{}, nil
	}
	return &sdk.PushResult{
		Created: r.Created,
		Updated: r.Updated,
		Skipped: r.Skipped,
	}, nil
}
