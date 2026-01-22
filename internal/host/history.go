// This file implements version history host functions.
//
// llmd maintains full version history for all documents. Every write creates
// a new version, and no data is ever truly deleted. These functions allow
// plugins to list versions, compare them, and revert to previous states.
package host

import (
	"context"
	"strconv"

	"github.com/jpl-au/llmd/internal/llmd/core"
	"github.com/jpl-au/llmd/internal/llmd/history"
	hostpb "github.com/jpl-au/llmd/proto/host"
)

// HistoryList returns version history for a document.
//
// Returns version metadata (version number, author, message, timestamp) in
// reverse chronological order (newest first). Use Limit to restrict the
// number of versions returned.
func (h *HostFuncs) HistoryList(ctx context.Context, req *hostpb.HistoryRequest) (*hostpb.VersionList, error) {
	opts := history.ListOptions{
		Limit: int(req.Limit),
	}

	infos, err := h.store.History.List(ctx, req.Path, opts)
	if err != nil {
		return nil, err
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

	return &hostpb.VersionList{Versions: versions}, nil
}

// HistoryDiff compares two versions of a document.
//
// Returns a diff showing the differences between Version1 and Version2.
// If either version is 0, the latest version is used. The diff format is
// similar to unified diff format.
func (h *HostFuncs) HistoryDiff(ctx context.Context, req *hostpb.DiffRequest) (*hostpb.DiffResult, error) {
	v1 := req.Path
	v2 := req.Path
	if req.Version1 > 0 {
		v1 = req.Path + ":" + strconv.Itoa(int(req.Version1))
	}
	if req.Version2 > 0 {
		v2 = req.Path + ":" + strconv.Itoa(int(req.Version2))
	}

	result, err := h.store.History.Diff(ctx, v1, v2)
	if err != nil {
		return nil, err
	}

	diff := "--- " + result.A.Path + "\n+++ " + result.B.Path + "\n"
	diff += result.A.Content + "\n---\n" + result.B.Content

	return &hostpb.DiffResult{Diff: diff}, nil
}

// HistoryRevert reverts a document to a previous version.
//
// Creates a new version with the content from the specified version. This
// doesn't delete any history - it adds a new version with old content.
// Returns the new document state.
func (h *HostFuncs) HistoryRevert(ctx context.Context, req *hostpb.RevertRequest) (*hostpb.Document, error) {
	opts := history.RevertOptions{
		WriteContext: core.WriteContext{
			Author: req.Author,
			Source: "plugin",
		},
	}

	doc, err := h.store.History.Revert(ctx, req.Path, int(req.Version), opts)
	if err != nil {
		return nil, err
	}

	return &hostpb.Document{
		Path:      doc.Path,
		Content:   []byte(doc.Content),
		Version:   int32(doc.Version),
		Author:    doc.Author,
		Message:   doc.Message,
		CreatedAt: doc.CreatedAt,
	}, nil
}
