package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/sdk"
)

// activityAPI implements [sdk.ActivityStore] by delegating to the
// internal store's RecentActivity method.
type activityAPI struct {
	ctx   context.Context
	store *llmd.Store
}

// newActivityAPI creates an activity API bridge wrapping the given store.
// The context controls cancellation and timeout for all store operations.
func newActivityAPI(store *llmd.Store, ctx context.Context) *activityAPI {
	return &activityAPI{ctx: ctx, store: store}
}

// Recent returns the most recent events across all domains.
func (a *activityAPI) Recent(limit int) ([]sdk.Activity, error) {
	events, err := a.store.RecentActivity(a.ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]sdk.Activity, len(events))
	for i, e := range events {
		out[i] = sdk.Activity{
			Type:      e.Type,
			Action:    e.Action,
			Subject:   e.Subject,
			Author:    e.Author,
			Detail:    e.Detail,
			Timestamp: e.Timestamp,
		}
	}
	return out, nil
}
