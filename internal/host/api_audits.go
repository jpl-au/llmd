package host

import (
	"context"
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/audits"
	"github.com/jpl-au/llmd/sdk"
)

// auditAPI implements [sdk.AuditStore] by delegating to the internal
// audits package.
type auditAPI struct {
	ctx   context.Context
	store *llmd.Store
}

func newAuditAPI(store *llmd.Store, ctx context.Context) *auditAPI {
	return &auditAPI{ctx: ctx, store: store}
}

// auditErr translates internal audit errors to SDK sentinels.
func auditErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, audits.ErrNotFound):
		return fmt.Errorf("%w: %w", sdk.ErrNotFound, err)
	case errors.Is(err, audits.ErrMissingAuthor):
		return fmt.Errorf("%w: %w", sdk.ErrMissingArg, err)
	case errors.Is(err, audits.ErrMissingTarget):
		return fmt.Errorf("%w: %w", sdk.ErrMissingArg, err)
	default:
		return err
	}
}

// auditToSDK converts an internal audit to the SDK representation.
func auditToSDK(a *audits.Audit) sdk.Audit {
	return sdk.Audit{
		ID:         a.ID,
		Target:     a.Target,
		TargetType: a.TargetType,
		Version:    a.Version,
		Author:     a.Author,
		Assignee:   a.Assignee,
		Status:     a.Status,
		Content:    a.Content,
		ParentID:   a.ParentID,
		CreatedAt:  a.CreatedAt,
	}
}

func (api *auditAPI) Add(opts sdk.AuditOpts) (*sdk.Audit, error) {
	a, err := api.store.Audits.Add(api.ctx, audits.AddOptions{
		Target:   opts.Target,
		Content:  opts.Content,
		Author:   opts.Author,
		Assignee: opts.Assignee,
		Status:   opts.Status,
		Version:  opts.Version,
	})
	if err != nil {
		return nil, auditErr(err)
	}
	out := auditToSDK(a)
	return &out, nil
}

func (api *auditAPI) Reply(id string, opts sdk.AuditOpts) (*sdk.Audit, error) {
	a, err := api.store.Audits.Reply(api.ctx, id, audits.AddOptions{
		Content:  opts.Content,
		Author:   opts.Author,
		Assignee: opts.Assignee,
		Status:   opts.Status,
	})
	if err != nil {
		return nil, auditErr(err)
	}
	out := auditToSDK(a)
	return &out, nil
}

func (api *auditAPI) Read(id string) (*sdk.Audit, error) {
	a, err := api.store.Audits.Read(api.ctx, id)
	if err != nil {
		return nil, auditErr(err)
	}
	out := auditToSDK(a)
	return &out, nil
}

func (api *auditAPI) List(opts sdk.AuditListOpts) ([]sdk.Audit, error) {
	aa, err := api.store.Audits.List(api.ctx, audits.ListOptions{
		Target:   opts.Target,
		ByAuthor: opts.ByAuthor,
		Assignee: opts.Assignee,
		Status:   opts.Status,
		Pending:  opts.Pending,
	})
	if err != nil {
		return nil, auditErr(err)
	}
	out := make([]sdk.Audit, len(aa))
	for i, a := range aa {
		out[i] = auditToSDK(&a)
	}
	return out, nil
}

func (api *auditAPI) Thread(id string) ([]sdk.Audit, error) {
	aa, err := api.store.Audits.Thread(api.ctx, id)
	if err != nil {
		return nil, auditErr(err)
	}
	out := make([]sdk.Audit, len(aa))
	for i, a := range aa {
		out[i] = auditToSDK(&a)
	}
	return out, nil
}

func (api *auditAPI) Resolve(id, author string) (*sdk.Audit, error) {
	a, err := api.store.Audits.Resolve(api.ctx, id, author)
	if err != nil {
		return nil, auditErr(err)
	}
	out := auditToSDK(a)
	return &out, nil
}

func (api *auditAPI) Delete(id, author string) error {
	return auditErr(api.store.Audits.Delete(api.ctx, id, author))
}

func (api *auditAPI) Status(author string) (*sdk.AuditStatus, error) {
	result, err := api.store.Audits.Status(api.ctx, author)
	if err != nil {
		return nil, auditErr(err)
	}
	pending := make([]sdk.Audit, len(result.Pending))
	for i, a := range result.Pending {
		pending[i] = auditToSDK(&a)
	}
	return &sdk.AuditStatus{
		Author:  result.Author,
		Pending: pending,
		Summary: sdk.AuditSummary{
			Total:     result.Summary.Total,
			NeedsWork: result.Summary.NeedsWork,
			Pending:   result.Summary.Pending,
		},
	}, nil
}
