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

// newAuditAPI creates the SDK-to-internal bridge for audit operations.
// The context is captured once and reused for all calls because the
// host creates one API instance per session — each session has a
// single cancellation scope.
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

// Add implements [sdk.AuditStore].
func (a *auditAPI) Add(opts sdk.AuditOpts) (*sdk.Audit, error) {
	aud, err := a.store.Audits.Add(a.ctx, audits.AddOptions{
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
	out := auditToSDK(aud)
	return &out, nil
}

// Reply implements [sdk.AuditStore].
func (a *auditAPI) Reply(id string, opts sdk.AuditOpts) (*sdk.Audit, error) {
	aud, err := a.store.Audits.Reply(a.ctx, id, audits.AddOptions{
		Content:  opts.Content,
		Author:   opts.Author,
		Assignee: opts.Assignee,
		Status:   opts.Status,
	})
	if err != nil {
		return nil, auditErr(err)
	}
	out := auditToSDK(aud)
	return &out, nil
}

// Read implements [sdk.AuditStore].
func (a *auditAPI) Read(id string) (*sdk.Audit, error) {
	aud, err := a.store.Audits.Read(a.ctx, id)
	if err != nil {
		return nil, auditErr(err)
	}
	out := auditToSDK(aud)
	return &out, nil
}

// List implements [sdk.AuditStore].
func (a *auditAPI) List(opts sdk.AuditListOpts) ([]sdk.Audit, error) {
	var sinceMS int64
	if !opts.Since.IsZero() {
		sinceMS = opts.Since.UnixMilli()
	}
	aa, err := a.store.Audits.List(a.ctx, audits.ListOptions{
		Target:   opts.Target,
		ByAuthor: opts.ByAuthor,
		Assignee: opts.Assignee,
		Status:   opts.Status,
		Pending:  opts.Pending,
		SinceMS:  sinceMS,
	})
	if err != nil {
		return nil, auditErr(err)
	}
	out := make([]sdk.Audit, len(aa))
	for i, v := range aa {
		out[i] = auditToSDK(&v)
	}
	return out, nil
}

// Thread implements [sdk.AuditStore].
func (a *auditAPI) Thread(id string) ([]sdk.Audit, error) {
	aa, err := a.store.Audits.Thread(a.ctx, id)
	if err != nil {
		return nil, auditErr(err)
	}
	out := make([]sdk.Audit, len(aa))
	for i, v := range aa {
		out[i] = auditToSDK(&v)
	}
	return out, nil
}

// Resolve implements [sdk.AuditStore].
func (a *auditAPI) Resolve(id, author string) (*sdk.Audit, error) {
	aud, err := a.store.Audits.Resolve(a.ctx, id, author)
	if err != nil {
		return nil, auditErr(err)
	}
	out := auditToSDK(aud)
	return &out, nil
}

// Delete implements [sdk.AuditStore].
func (a *auditAPI) Delete(id, author string) error {
	return auditErr(a.store.Audits.Delete(a.ctx, id, author))
}

// Restore implements [sdk.AuditStore].
func (a *auditAPI) Restore(id, author string) (*sdk.Audit, error) {
	aud, err := a.store.Audits.Restore(a.ctx, id, author)
	if err != nil {
		return nil, auditErr(err)
	}
	out := auditToSDK(aud)
	return &out, nil
}

// Status implements [sdk.AuditStore].
func (a *auditAPI) Status(author string, opts sdk.AuditStatusOpts) (*sdk.AuditStatus, error) {
	var sinceMS int64
	if !opts.Since.IsZero() {
		sinceMS = opts.Since.UnixMilli()
	}
	result, err := a.store.Audits.Status(a.ctx, author, sinceMS)
	if err != nil {
		return nil, auditErr(err)
	}
	pending := make([]sdk.Audit, len(result.Pending))
	for i, v := range result.Pending {
		pending[i] = auditToSDK(&v)
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
