// This file implements version history host functions.
//
// llmd maintains full version history for all documents. Every write creates
// a new version, and no data is ever truly deleted. These functions allow
// plugins to list versions, compare them, and revert to previous states.
package host

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd/history"
	"github.com/jpl-au/llmd/pkg/model/core"
	hostpb "github.com/jpl-au/llmd/proto/host"
)

// HistoryList returns version history for a document.
//
// Returns version metadata (version number, author, message, timestamp) in
// reverse chronological order (newest first). Use Limit to restrict the
// number of versions returned.
func (h *HostFuncs) HistoryList(ctx context.Context, req *hostpb.HistoryRequest) (*hostpb.VersionListResponse, error) {
	if h.store == nil {
		return &hostpb.VersionListResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := history.ListOptions{
		Limit: int(req.Limit),
	}

	infos, err := h.store.History.List(ctx, req.Path, opts)
	if err != nil {
		return &hostpb.VersionListResponse{Error: err.Error()}, nil
	}

	versions := make([]*hostpb.VersionInfo, len(infos))
	for i, info := range infos {
		versions[i] = &hostpb.VersionInfo{
			Version:   int32(info.Version),
			Author:    info.Author,
			Message:   info.Message,
			CreatedAt: info.CreatedAt,
		}
	}

	return &hostpb.VersionListResponse{Success: true, Versions: versions}, nil
}

// HistoryDiff compares two documents or document versions.
//
// Source and Target can be filesystem paths, llmd paths, llmd path:version,
// or 9-char keys. Returns a unified diff showing the differences.
func (h *HostFuncs) HistoryDiff(ctx context.Context, req *hostpb.DiffRequest) (*hostpb.DiffResponse, error) {
	if h.store == nil {
		return &hostpb.DiffResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}

	opts := history.DiffOptions{
		Context: int(req.Context),
	}

	result, err := h.store.History.Diff(ctx, req.Source, req.Target, opts)
	if err != nil {
		return &hostpb.DiffResponse{Error: err.Error()}, nil
	}

	return &hostpb.DiffResponse{
		Success: true,
		Diff:    result.Unified,
		Added:   int32(result.Stats.Added),
		Removed: int32(result.Stats.Removed),
	}, nil
}

// HistoryRevert reverts a document to a previous version.
//
// Creates a new version with the content from the specified version. This
// doesn't delete any history - it adds a new version with old content.
// Returns the new document state.
func (h *HostFuncs) HistoryRevert(ctx context.Context, req *hostpb.RevertRequest) (*hostpb.DocumentResponse, error) {
	if h.store == nil {
		return &hostpb.DocumentResponse{Error: ErrStoreNotAvailable.Error()}, nil
	}
	opts := history.RevertOptions{
		Origin: core.Origin{
			Author: req.Author,
			Source: "plugin",
		},
	}

	doc, err := h.store.History.Revert(ctx, req.Path, int(req.Version), opts)
	if err != nil {
		return &hostpb.DocumentResponse{Error: err.Error()}, nil
	}

	return &hostpb.DocumentResponse{
		Success: true,
		Document: &hostpb.Document{
			Path:      doc.Path,
			Content:   []byte(doc.Content),
			Version:   int32(doc.Version),
			Author:    doc.Author,
			Message:   doc.Message,
			CreatedAt: doc.CreatedAt,
		},
	}, nil
}
