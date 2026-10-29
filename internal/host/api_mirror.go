package host

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/bulk"
	"github.com/jpl-au/llmd/sdk"
)

// mirrorAPI implements [sdk.MirrorStore] by delegating to the internal
// bulk package for both pull (store → filesystem) and push (filesystem
// → store) operations.
type mirrorAPI struct {
	ctx   context.Context
	store *llmd.Store
}

// newMirrorAPI creates a mirror API bridge wrapping the given store.
// The context controls cancellation and timeout for all store operations.
func newMirrorAPI(store *llmd.Store, ctx context.Context) *mirrorAPI {
	return &mirrorAPI{ctx: ctx, store: store}
}

// Directory returns the mirror directory for the active store.
// Derives the directory name from the database filename:
// llmd.db → .llmd/llmd/, llmd-docs.db → .llmd/llmd-docs/.
func (a *mirrorAPI) Directory() string {
	return filepath.Join(".llmd", mirrorName(a.store.Path()))
}

// mirrorName is the name of a store's mirror directory: the database
// filename without .db.
func mirrorName(dbPath string) string {
	return strings.TrimSuffix(filepath.Base(dbPath), ".db")
}

// Pull writes store documents to the filesystem, skipping unchanged
// files and removing stale ones.
func (a *mirrorAPI) Pull(prefix, dir string) (*sdk.PullResult, error) {
	r, err := a.store.Bulk.Mirror(a.ctx, prefix, dir)
	if err != nil {
		return nil, err
	}
	return &sdk.PullResult{
		Wrote:   r.Wrote,
		Skipped: r.Skipped,
		Removed: r.Removed,
	}, nil
}

// Push imports .md files from the filesystem back into the store and
// deletes the documents whose mirror file was removed. A prefix limits
// it to the files under that path, as pull's prefix limits what is
// written.
func (a *mirrorAPI) Push(dir string, opts sdk.PushOpts) (*sdk.PushResult, error) {
	r, err := a.store.Bulk.MirrorPush(a.ctx, dir, bulk.MirrorPushOptions{
		Origin: origin("mirror"),
		Prefix: opts.Prefix,
	})
	if err != nil {
		return nil, err
	}
	out := &sdk.PushResult{
		Created: r.Created,
		Updated: r.Updated,
		Skipped: r.Skipped,
		Deleted: r.Deleted,
		Meta:    sdk.MetaCounts(r.Meta),
	}
	for _, e := range r.Errors {
		out.Errors = append(out.Errors, e.Path+": "+e.Err.Error())
	}
	return out, nil
}
